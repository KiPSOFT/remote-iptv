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
	// MPV player setup
	player, err := player.NewMPVPlayer()
	if err != nil {
		log.Fatalf("Failed to initialize MPV player: %v", err)
	}
	defer player.Cleanup()

	// Database setup
	database, err := db.NewDatabase("iptv.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// API handlers setup
	handler := api.NewHandler(player, database, nil)

	// Router setup
	r := mux.NewRouter()
	handler.RegisterRoutes(r)

	// Static file server for web UI
	spa := spaHandler{staticPath: "web/build", indexPath: "index.html"}
	r.PathPrefix("/").Handler(spa)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s...\n", port)
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