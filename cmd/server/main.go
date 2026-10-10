package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"bikegeo/pkg/storage"
	"bikegeo/pkg/web"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data/bikes"
	}

	store, err := storage.NewBikeStore(dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize bike storage: %v", err)
	}

	server := web.NewServer(store)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("🚲 BikeGeo Analyst Server listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("Server exited: %v", err)
	}
}
