package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"remote-iptv/internal/api"
	"remote-iptv/internal/db"
	"remote-iptv/internal/player"


)

func main() {
	runServer()
}

func runServer() {
	// MPV player setup
	player, err := player.NewMPVPlayer()
	if err != nil {
		log.Fatalf("Failed to initialize MPV player: %v", err)
	}
	defer player.Cleanup()

	// Database setup
	dbPath := "iptv.db"
	if os.Getenv("PWD") != "" {
		dbPath = filepath.Join(os.Getenv("PWD"), "data", "iptv.db")
	}
	log.Printf("Database path: %s\n", dbPath)
	database, err := db.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// API handlers setup
	handler := api.NewHandler(player, database, nil)

	// Router setup
	r := mux.NewRouter()
	handler.RegisterRoutes(r)

	// Determine static files location
	staticPath := "web/build"
	if webRoot := os.Getenv("WEB_ROOT"); webRoot != "" {
		staticPath = webRoot
		log.Printf("Using web root from environment: %s\n", staticPath)
	}

	// Static file server for web UI
	spa := spaHandler{staticPath: staticPath, indexPath: "index.html"}
	r.PathPrefix("/").Handler(spa)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s...\n", port)
	log.Printf("Static files served from: %s\n", staticPath)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}

// SPA handler for serving React frontend
type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(h.staticPath, r.URL.Path)

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}

// API handlers (to be implemented)
func getChannels(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement channel list retrieval
}

func playChannel(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement channel playback
}

func stopChannel(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement playback stop
}

func getFavorites(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement favorites retrieval
}

func addFavorite(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement adding to favorites
} 