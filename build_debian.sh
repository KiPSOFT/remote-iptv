#!/bin/bash
set -e

echo "Starting Remote IPTV build process for Debian..."

# Define variables
APP_NAME="remote-iptv"
BUILD_DIR="./dist"
VERSION="1.0.0"
TIMESTAMP=$(date +"%Y%m%d%H%M")
INSTALL_DIR="/opt/${APP_NAME}"
PACKAGE_NAME="${APP_NAME}_${VERSION}-${TIMESTAMP}"
ARCH="amd64"

# Make sure required tools are installed
echo "Checking for required build tools..."
command -v go >/dev/null 2>&1 || { echo "Go is required but not installed. Aborting."; exit 1; }
command -v npm >/dev/null 2>&1 || { echo "Node.js/npm is required but not installed. Aborting."; exit 1; }
command -v pkg-config >/dev/null 2>&1 || { echo "pkg-config is required but not installed. Aborting."; exit 1; }

# Create build directory
echo "Creating build directory..."
rm -rf ${BUILD_DIR}
mkdir -p ${BUILD_DIR}
mkdir -p ${BUILD_DIR}/${APP_NAME}
mkdir -p ${BUILD_DIR}/${APP_NAME}/bin
mkdir -p ${BUILD_DIR}/${APP_NAME}/web
mkdir -p ${BUILD_DIR}/${APP_NAME}/data
mkdir -p ${BUILD_DIR}/${APP_NAME}/resources

# Build the React frontend
echo "Building React frontend..."
cd web
npm install
npm run build
cd ..
cp -r web/build/* ${BUILD_DIR}/${APP_NAME}/web/

# Create Go GTK wrapper
echo "Creating GTK system tray application wrapper..."
cat > gtk_wrapper.go << EOF
package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

var appPath string
var serverProcess *exec.Cmd

func main() {
	// Initialize GTK
	gtk.Init(nil)

	// Get application path
	executable, err := os.Executable()
	if err != nil {
		log.Fatal("Failed to get executable path:", err)
	}
	appPath = filepath.Dir(executable)

	// Create status icon
	statusIcon, err := gtk.StatusIconNew()
	if err != nil {
		log.Fatal("Failed to create status icon:", err)
	}

	// Set icon from resource or use fallback
	iconPath := filepath.Join(appPath, "resources", "app-icon.png")
	if _, err := os.Stat(iconPath); err == nil {
		statusIcon.SetFromFile(iconPath)
	} else {
		// Fallback to a stock icon
		statusIcon.SetFromIconName("video-display")
	}

	statusIcon.SetTitle("Remote IPTV Player")
	statusIcon.SetTooltipText("Remote IPTV Player")
	statusIcon.SetVisible(true)

	// Create menu
	menu, err := gtk.MenuNew()
	if err != nil {
		log.Fatal("Failed to create menu:", err)
	}

	// Create Open menu item
	openItem, err := gtk.MenuItemNewWithLabel("Open IPTV Interface")
	if err != nil {
		log.Fatal("Failed to create menu item:", err)
	}
	openItem.Connect("activate", func() {
		openBrowser("http://localhost:8080")
	})
	menu.Append(openItem)

	// Add separator
	separator, err := gtk.SeparatorMenuItemNew()
	if err != nil {
		log.Fatal("Failed to create separator:", err)
	}
	menu.Append(separator)

	// Create Quit menu item
	quitItem, err := gtk.MenuItemNewWithLabel("Quit")
	if err != nil {
		log.Fatal("Failed to create menu item:", err)
	}
	quitItem.Connect("activate", func() {
		stopServer()
		gtk.MainQuit()
	})
	menu.Append(quitItem)

	// Show all menu items
	menu.ShowAll()

	// Connect the menu to the status icon
	statusIcon.Connect("popup-menu", func(icon *gtk.StatusIcon, button uint, activateTime uint32) {
		menu.PopupAtStatusIcon(icon, button, activateTime)
	})

	// Start the server
	startServer()

	// Open browser after a short delay
	go func() {
		time.Sleep(2 * time.Second)
		glib.IdleAdd(func() {
			openBrowser("http://localhost:8080")
		})
	}()

	// Run the GTK main loop
	gtk.Main()
}

func startServer() {
	// Check if MPV is installed
	_, err := exec.LookPath("mpv")
	if err != nil {
		dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, 
			"MPV is required but not installed. Please install with: sudo apt install mpv")
		dialog.Run()
		dialog.Destroy()
		gtk.MainQuit()
		return
	}

	// Start the server
	serverPath := filepath.Join(appPath, "bin", "remote-iptv")
	serverProcess = exec.Command(serverPath)
	
	// Set working directory
	serverProcess.Dir = appPath
	
	// Set environment variables
	serverProcess.Env = append(os.Environ(), 
		"PORT=8080",
		"WEB_ROOT="+filepath.Join(appPath, "web"),
		"PWD="+appPath,
	)

	// Redirect stdout and stderr to log files
	logFile, err := os.Create(filepath.Join(appPath, "remote-iptv.log"))
	if err != nil {
		log.Printf("Failed to create log file: %v", err)
		serverProcess.Stdout = os.Stdout
		serverProcess.Stderr = os.Stderr
	} else {
		serverProcess.Stdout = logFile
		serverProcess.Stderr = logFile
	}

	err = serverProcess.Start()
	if err != nil {
		dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, 
			fmt.Sprintf("Failed to start server: %v", err))
		dialog.Run()
		dialog.Destroy()
		gtk.MainQuit()
	}
}

func stopServer() {
	if serverProcess != nil && serverProcess.Process != nil {
		// Try to gracefully terminate
		serverProcess.Process.Signal(syscall.SIGTERM)
		
		// Wait for up to 1 second for the process to exit
		done := make(chan error)
		go func() {
			done <- serverProcess.Wait()
		}()
		
		select {
		case <-done:
			// Process exited
		case <-time.After(1 * time.Second):
			// Process didn't exit, force kill
			serverProcess.Process.Kill()
		}
	}
}

func openBrowser(url string) {
	var err error

	// Try xdg-open first (most common on Linux)
	if _, err = exec.LookPath("xdg-open"); err == nil {
		exec.Command("xdg-open", url).Start()
		return
	}

	// Try other browsers if xdg-open doesn't exist
	for _, browser := range []string{"firefox", "google-chrome", "chromium-browser"} {
		if _, err = exec.LookPath(browser); err == nil {
			exec.Command(browser, url).Start()
			return
		}
	}

	log.Printf("Could not find a browser to open %s", url)
	dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_INFO, gtk.BUTTONS_OK, 
		fmt.Sprintf("Please open your browser and navigate to: %s", url))
	dialog.Run()
	dialog.Destroy()
}
EOF

# Create app icon for GTK app
mkdir -p ${BUILD_DIR}/${APP_NAME}/resources
# Create a simple icon (you can replace with a better one later)
cat > create_icon.py << EOF
from PIL import Image, ImageDraw
import os

# Create a 128x128 image with an IPTV icon
img = Image.new('RGBA', (128, 128), (0, 0, 0, 0))
draw = ImageDraw.Draw(img)

# Draw a TV-like shape
draw.rectangle((24, 32, 104, 88), outline=(30, 144, 255), width=4)
draw.rectangle((32, 40, 96, 80), fill=(30, 144, 255, 64))

# Draw TV stand
draw.polygon([(48, 88), (80, 88), (72, 104), (56, 104)], fill=(30, 144, 255))

# Save icon
img.save('${BUILD_DIR}/${APP_NAME}/resources/app-icon.png')
EOF

# Try to generate icon with Python PIL
if command -v python3 >/dev/null 2>&1; then
    if python3 -c "import PIL" &>/dev/null; then
        echo "Generating app icon..."
        python3 create_icon.py
    else
        echo "PIL not found, skipping icon generation"
    fi
    rm create_icon.py
else
    echo "Python not found, skipping icon generation"
    rm create_icon.py
fi

# Build the Go GTK wrapper
echo "Building Go GTK wrapper..."
cat > go.mod.gtk << EOF
module gtk-wrapper

go 1.21

require (
	github.com/gotk3/gotk3 v0.6.2
)
EOF

mkdir -p gtk-build
cp gtk_wrapper.go gtk-build/
cp go.mod.gtk gtk-build/go.mod
cd gtk-build

# Build GTK wrapper
echo "Compiling GTK wrapper..."
go mod tidy
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o ../${BUILD_DIR}/${APP_NAME}/bin/remote-iptv-gtk gtk_wrapper.go
cd ..
rm -rf gtk-build
rm gtk_wrapper.go go.mod.gtk

# Build the Go backend
echo "Building Go backend..."
go mod tidy
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o ${BUILD_DIR}/${APP_NAME}/bin/remote-iptv ./cmd/server

# Create a run script
cat > ${BUILD_DIR}/${APP_NAME}/run.sh << EOF
#!/bin/bash
# Run the GTK application
cd "\$(dirname "\$0")"
./bin/remote-iptv-gtk
EOF

# Make the run script executable
chmod +x ${BUILD_DIR}/${APP_NAME}/run.sh

# Make the binaries executable
chmod +x ${BUILD_DIR}/${APP_NAME}/bin/remote-iptv-gtk
chmod +x ${BUILD_DIR}/${APP_NAME}/bin/remote-iptv

# Create README file with instructions
cat > ${BUILD_DIR}/${APP_NAME}/README.txt << EOF
REMOTE IPTV APPLICATION
-----------------------

This is a web-based IPTV control system that allows you to manage IPTV channels
through a web interface without installing a desktop application.

REQUIREMENTS:
- MPV media player (install with: sudo apt install mpv)
- GTK3 libraries (usually pre-installed on Debian)
- SQLite3 (usually pre-installed on Debian)

USAGE:
1. Run the application from your application menu
2. The application will appear in your system tray
3. Click on the icon to access the menu options

The application will automatically open your default web browser to the interface.
If the browser does not open automatically, you can select "Open IPTV Interface" 
from the system tray menu or visit: http://localhost:8080
EOF

# -------------------------------------------
# DEB PACKAGE CREATION
# -------------------------------------------
echo "Creating Debian package..."

# Create directory structure for the DEB package
DEB_ROOT="${BUILD_DIR}/deb"
DEB_PATH="${DEB_ROOT}/DEBIAN"
DEB_APP_PATH="${DEB_ROOT}${INSTALL_DIR}"
DEB_DESKTOP_PATH="${DEB_ROOT}/usr/share/applications"
mkdir -p "${DEB_PATH}"
mkdir -p "${DEB_APP_PATH}"
mkdir -p "${DEB_DESKTOP_PATH}"

# Create control file
cat > "${DEB_PATH}/control" << EOF
Package: ${APP_NAME}
Version: ${VERSION}-${TIMESTAMP}
Section: video
Priority: optional
Architecture: ${ARCH}
Depends: mpv, libgtk-3-0, libsqlite3-0
Maintainer: Serkan KOCAMAN <github.com/KiPSOFT>
Description: Remote IPTV Player
 A web-based IPTV control system that allows you to 
 manage IPTV channels through a web interface without 
 installing a desktop application.
EOF

# Create postinst script to handle post-installation tasks
cat > "${DEB_PATH}/postinst" << EOF
#!/bin/bash
# Make sure all files are executable
chmod +x ${INSTALL_DIR}/bin/remote-iptv-gtk
chmod +x ${INSTALL_DIR}/bin/remote-iptv
chmod +x ${INSTALL_DIR}/run.sh
exit 0
EOF

# Make postinst script executable
chmod 755 "${DEB_PATH}/postinst"

# Copy application files
cp -r "${BUILD_DIR}/${APP_NAME}/"* "${DEB_APP_PATH}/"

# Create desktop entry
cat > "${DEB_DESKTOP_PATH}/remote-iptv.desktop" << EOF
[Desktop Entry]
Name=Remote IPTV
Comment=Control IPTV streams through a web interface
Exec=${INSTALL_DIR}/run.sh
Icon=${INSTALL_DIR}/resources/app-icon.png
Terminal=false
Type=Application
Categories=AudioVideo;Video;
EOF

# Create the deb package
echo "Building DEB package..."
cd "${BUILD_DIR}"
dpkg-deb --build deb "${PACKAGE_NAME}_${ARCH}.deb"

if [ $? -eq 0 ]; then
    echo "DEB package created: ${BUILD_DIR}/${PACKAGE_NAME}_${ARCH}.deb"
else
    echo "Failed to create DEB package."
    echo "Falling back to tarball creation..."
    
    # Create a tarball as backup
    tar -czvf "${APP_NAME}-${VERSION}-${TIMESTAMP}.tar.gz" ${APP_NAME}
    
    echo "Tarball created: ${BUILD_DIR}/${APP_NAME}-${VERSION}-${TIMESTAMP}.tar.gz"
fi

cd ..

echo ""
echo "Build completed successfully!"
echo ""
echo "To install on a Debian system:"
echo "1. Copy the .deb package to the target system"
echo "2. Install with: sudo apt install ./remote-iptv_*.deb"
echo "  or: sudo dpkg -i remote-iptv_*.deb && sudo apt-get install -f"
echo ""
echo "After installation, you can run the application from your application menu." 