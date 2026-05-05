package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("fatal: resolve executable: %v", err)
	}

	publicDir := filepath.Join(filepath.Dir(exe), "public")

	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser("http://localhost:8080")
	}()

	log.Printf("serve: %s at http://localhost:8080", publicDir)
	log.Printf("interrupt to exit")

	if err := http.ListenAndServe(":8080", http.FileServer(http.Dir(publicDir))); err != nil {
		log.Fatalf("fatal: server: %v", err)
	}
}
