package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"sync"
)

type MPVPlayer struct {
	cmd          *exec.Cmd
	socketDir    string
	socketID     string
	socketPath   string
	conn         net.Conn
	mutex        sync.Mutex
	
	// Command and state channels
	commandCh    chan func()
	stateCh      chan bool
	isActive     bool
	
	// Control channel for the worker
	done         chan struct{}

	// New fields for auto-restart
	currentURL   string
	autoRestart  bool
	manualStop   bool
}

type MPVCommand struct {
	Command []interface{} `json:"command"`
}

type MPVResponse struct {
	Data interface{} `json:"data"`
	Error string     `json:"error"`
}

func init() {
	// Check if socat is available
	cmd := exec.Command("which", "socat")
	if err := cmd.Run(); err != nil {
		log.Printf("Warning: socat not found. IPC communication with MPV may be limited.")
		log.Printf("Please install socat for better MPV control.")
	}
}

// Helper function to send commands to MPV socket
func sendToMPVSocket(socketPath string, command string) ([]byte, error) {
	// First try with socat, which is the most reliable method
	socatCmd := exec.Command("socat", "-", socketPath)
	socatCmd.Stdin = strings.NewReader(command + "\n")
	
	// Try socat first
	output, err := socatCmd.CombinedOutput()
	if err == nil {
		return output, nil
	}
	
	// If socat failed, try direct with nc (netcat)
	log.Printf("Socat failed, trying netcat: %v", err)
	ncCmd := exec.Command("nc", "-U", socketPath)
	ncCmd.Stdin = strings.NewReader(command + "\n")
	
	output, err = ncCmd.CombinedOutput()
	if err == nil {
		return output, nil
	}
	
	// As a last resort, try direct file operations
	log.Printf("Netcat failed, trying direct file io: %v", err)
	socket, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to socket: %v", err)
	}
	defer socket.Close()
	
	// Write the command
	if _, err := socket.Write([]byte(command + "\n")); err != nil {
		return nil, fmt.Errorf("failed to write to socket: %v", err)
	}
	
	// Read the response
	buf := make([]byte, 1024)
	n, err := socket.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read from socket: %v", err)
	}
	
	return buf[:n], nil
}

func NewMPVPlayer() (*MPVPlayer, error) {
	// Geçici dizin oluştur
	socketDir, err := os.MkdirTemp("", "mpv-socket-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create socket directory: %v", err)
	}

	// Benzersiz bir socket ID oluştur
	socketID := fmt.Sprintf("mpvsocket_%d", time.Now().UnixNano())
	socketPath := socketDir + "/" + socketID

	player := &MPVPlayer{
		socketDir:   socketDir,
		socketID:    socketID,
		socketPath:  socketPath,
		isActive:    false,
		commandCh:   make(chan func(), 10),  // Buffer for commands
		stateCh:     make(chan bool, 1),     // Channel for state updates
		done:        make(chan struct{}),    // Channel to signal worker shutdown
		autoRestart: true,                   // Enable auto-restart by default
		manualStop:  false,                  // Initialize manual stop flag
	}
	
	// Start the worker goroutine
	go player.processCommands()
	
	return player, nil
}

// Worker goroutine to process commands sequentially
func (p *MPVPlayer) processCommands() {
	for {
		select {
		case cmd := <-p.commandCh:
			cmd() // Execute the command function
		case state := <-p.stateCh:
			p.isActive = state
		case <-p.done:
			return // Exit the goroutine
		}
	}
}

func (p *MPVPlayer) Play(url string) error {
	resultCh := make(chan error, 1)
	
	// Queue the play command
	p.commandCh <- func() {
		p.currentURL = url    // Store current URL
		p.manualStop = false  // Reset manual stop flag
		
		// Check if MPV is already running and active
		if p.isActive && p.cmd != nil && p.cmd.Process != nil {
			// MPV is running, try to use loadfile to change the URL instead of restarting
			log.Printf("MPV already running, trying to change URL with loadfile command")
			
			// Use sendCommand to change URL
			cmd := MPVCommand{
				Command: []interface{}{"loadfile", url, "replace"},
			}
			
			if err := p.sendCommand(cmd); err == nil {
				log.Printf("Successfully changed URL to: %s", url)
				resultCh <- nil
				return
			} else {
				log.Printf("Failed to change URL with loadfile, will restart MPV: %v", err)
				// Fall through to restart MPV
			}
		}
		
		// If we get here, we need to start or restart MPV
		
		// Check if we need to stop an existing player
		if p.cmd != nil && p.cmd.Process != nil {
			log.Printf("Stopping existing player before starting new one")
			p.doStop(resultCh)
			select {
			case err := <-resultCh:
				if err != nil {
					log.Printf("Warning: error stopping existing player: %v", err)
				}
			case <-time.After(2 * time.Second):
				log.Printf("Warning: timeout waiting for player to stop")
			}
		}
		
		logFile := "/tmp/mpv_debug.log"
		log.Printf("Starting MPV with URL: %s", url)
		log.Printf("Debug logs will be saved to: %s", logFile)

		args := []string{
			"--no-config",
			"--terminal=no",
			"--msg-level=all=debug",
			"--log-file=" + logFile,
			"--audio-channels=stereo",
			"--ao=pulse,alsa,coreaudio",
			"--volume=100",
			"--audio-device=auto",
			"--vo=gpu",
			"--cache=yes",
			"--cache-secs=60",
			"--demuxer-max-bytes=500M",
			"--demuxer-max-back-bytes=100M",
			"--no-ytdl",
			"--ytdl=no",
			"--force-seekable=yes",
			"--network-timeout=30",
			"--user-agent=Tivimate",
			"--stream-lavf-o=reconnect=1",
			"--stream-lavf-o=reconnect_at_eof=1",
			"--stream-lavf-o=reconnect_streamed=1",
			"--stream-lavf-o=reconnect_delay_max=5",
			"--hls-bitrate=max",
			// Add IPC socket support for communication
			"--input-ipc-server=/tmp/mpvsocket",
			url,
		}

		p.cmd = exec.Command("mpv", args...)
		
		stderr, err := p.cmd.StderrPipe()
		if err != nil {
			log.Printf("Error creating stderr pipe: %v", err)
			resultCh <- fmt.Errorf("could not create stderr pipe: %w", err)
			return
		}

		// Log tüm argümanları
		log.Printf("MPV command: %s %s", p.cmd.Path, strings.Join(args, " "))

		// MPV'yi başlat
		if err := p.cmd.Start(); err != nil {
			log.Printf("Error starting MPV: %v", err)
			resultCh <- fmt.Errorf("could not start mpv: %w", err)
			return
		}

		// Hata çıktısını oku ve logla
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				log.Printf("MPV stderr: %s", scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				log.Printf("Error reading from MPV stderr: %v", err)
			}
		}()

		// Set active state
		p.stateCh <- true
		
		// MPV işlemini arka planda izle
		go func() {
			if err := p.cmd.Wait(); err != nil {
				log.Printf("MPV process ended with error: %v", err)
			} else {
				log.Printf("MPV process ended normally")
			}
			
			// İşlem bittikten sonra log dosyasını kontrol et
			time.Sleep(500 * time.Millisecond) // Dosyanın tamamen yazılması için kısa bir bekleme
			logBytes, err := os.ReadFile(logFile)
			if err != nil {
				log.Printf("Error reading MPV log file: %v", err)
			} else {
				log.Printf("MPV log file contents (last 500 bytes): \n%s", lastNBytes(string(logBytes), 500))
			}
			
			// Update state in thread-safe manner
			p.stateCh <- false

			// Auto-restart logic if not manually stopped
			if p.autoRestart && !p.manualStop && p.currentURL != "" {
				log.Printf("Auto-restarting MPV with URL: %s", p.currentURL)
				time.Sleep(1 * time.Second) // Small delay before restart
				p.Play(p.currentURL)
			}
		}()

		log.Printf("MPV started with PID: %d", p.cmd.Process.Pid)
		resultCh <- nil
	}
	
	// Wait for the result with timeout
	select {
	case err := <-resultCh:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout starting player, command queue might be blocked")
	}
}

// doStop is the internal implementation of Stop
func (p *MPVPlayer) doStop(resultCh chan<- error) {
	log.Printf("Stopping MPV player")
	
	if p.cmd == nil || p.cmd.Process == nil {
		log.Printf("No active MPV process to stop")
		p.stateCh <- false
		resultCh <- nil
		return
	}
	
	// SIGTERM sinyali gönder
	log.Printf("Sending SIGTERM to MPV process (PID: %d)", p.cmd.Process.Pid)
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("Error sending SIGTERM to MPV: %v", err)
		
		// SIGTERM başarısız olursa, SIGKILL dene
		log.Printf("Trying SIGKILL as fallback")
		if killErr := p.cmd.Process.Kill(); killErr != nil {
			log.Printf("Error killing MPV process: %v", killErr)
			resultCh <- fmt.Errorf("could not kill MPV process: %w", killErr)
			return
		}
	}
	
	// Force kill after timeout if needed
	go func() {
		time.Sleep(2 * time.Second)
		if p.cmd != nil && p.cmd.Process != nil {
			// Check if still alive
			if err := p.cmd.Process.Signal(syscall.Signal(0)); err == nil {
				log.Printf("Forcing kill of MPV process after timeout")
				p.cmd.Process.Kill()
			}
		}
	}()
	
	p.stateCh <- false
	resultCh <- nil
}

func (p *MPVPlayer) Stop() error {
	resultCh := make(chan error, 1)
	
	// Queue the stop command
	p.commandCh <- func() {
		p.manualStop = true  // Set manual stop flag
		p.doStop(resultCh)
	}
	
	// Wait for the result with timeout
	select {
	case err := <-resultCh:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout stopping player, command queue might be blocked")
	}
}

// isRunning checks if MPV process is active
func (p *MPVPlayer) isRunning() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	
	// Process'e 0 sinyali göndermeyi dene (UNIX tabanlı sistemlerde sadece varlığını kontrol eder)
	err := p.cmd.Process.Signal(os.Signal(syscall.Signal(0)))
	return err == nil
}

func (p *MPVPlayer) IsActive() bool {
	return p.isActive
}

func (p *MPVPlayer) Cleanup() {
	// Signal the worker to shut down
	close(p.done)
	
	// Stop the player if active
	if p.isActive {
		p.Stop()
	}
	
	// Clean up resources
	os.RemoveAll(p.socketDir)
}

// lastNBytes returns the last n bytes of a string
func lastNBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func (p *MPVPlayer) sendCommand(cmd MPVCommand) error {
	if !p.isActive {
		return fmt.Errorf("player is not active")
	}

	cmdData, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %v", err)
	}
	
	log.Printf("Sending MPV command: %s", string(cmdData))
	
	// Check if socket exists
	if _, err := os.Stat("/tmp/mpvsocket"); err != nil {
		return fmt.Errorf("IPC socket not available: %v", err)
	}
	
	// Use our helper function
	output, err := sendToMPVSocket("/tmp/mpvsocket", string(cmdData))
	if err != nil {
		log.Printf("Socket command failed: %v", err)
		return fmt.Errorf("failed to send command via socket: %v", err)
	}
	
	log.Printf("Command response: %s", string(output))
	
	// Check if we got a valid response
	if len(output) > 0 {
		var response MPVResponse
		if err := json.Unmarshal(output, &response); err != nil {
			log.Printf("Failed to parse response: %v", err)
			// Even if we can't parse, we succeeded in sending the command
			return nil
		}
		
		if response.Error != "" && response.Error != "success" {
			return fmt.Errorf("mpv error: %s", response.Error)
		}
	}
	
	return nil
}

func (p *MPVPlayer) GetMediaTitle() (string, error) {
	if !p.isActive {
		return "", fmt.Errorf("player is not active")
	}
	
	cmd := MPVCommand{
		Command: []interface{}{"get_property", "media-title"},
	}
	
	cmdData, err := json.Marshal(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to marshal command: %v", err)
	}
	
	// Check if socket exists
	if _, err := os.Stat("/tmp/mpvsocket"); err != nil {
		return "", fmt.Errorf("IPC socket not available: %v", err)
	}
	
	// Use our helper function
	output, err := sendToMPVSocket("/tmp/mpvsocket", string(cmdData))
	if err != nil {
		log.Printf("Socket command failed: %v", err)
		return "", fmt.Errorf("failed to get media title: %v", err)
	}
	
	log.Printf("Media title response: %s", string(output))
	
	// Parse the JSON response
	var response MPVResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return "", fmt.Errorf("failed to parse media title response: %v", err)
	}
	
	if response.Error != "" && response.Error != "success" {
		return "", fmt.Errorf("mpv error: %s", response.Error)
	}
	
	// Extract the title
	if response.Data == nil {
		return "", nil // No title available
	}
	
	// Handle different response formats
	switch v := response.Data.(type) {
	case string:
		return v, nil
	case map[string]interface{}:
		if title, ok := v["data"].(string); ok {
			return title, nil
		}
	}
	
	// If we can't parse the exact format, just convert to string
	titleJSON, _ := json.Marshal(response.Data)
	return string(titleJSON), nil
}

func (p *MPVPlayer) IsProcessAlive() (bool, error) {
	if p.cmd == nil || p.cmd.Process == nil {
		return false, nil
	}
	
	err := p.cmd.Process.Signal(os.Signal(syscall.Signal(0)))
	if err != nil {
		return false, nil
	}
	
	return true, nil
} 