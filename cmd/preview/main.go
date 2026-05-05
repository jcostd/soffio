package main

import (
	"flag"
	"log"
	"net/http"
	"time"
)

func main() {
	// Default to "public" but allow override
	dir := flag.String("dir", "public", "directory to serve")
	port := flag.String("port", "8080", "port to listen on")
	flag.Parse()

	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser("http://localhost:" + *port)
	}()

	log.Printf("serve: %s at http://localhost:%s", *dir, *port)
	log.Printf("interrupt to exit")

	if err := http.ListenAndServe(":"+*port, http.FileServer(http.Dir(*dir))); err != nil {
		log.Fatalf("fatal: server: %v", err)
	}
}
