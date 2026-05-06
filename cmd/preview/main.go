// Command preview serves the public directory and opens it in the browser.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"
)

func main() {
	var (
		dir  string
		port string
	)

	flag.StringVar(&dir, "dir", "public", "directory to serve")
	flag.StringVar(&dir, "d", "public", "directory to serve (shorthand)")

	flag.StringVar(&port, "port", "8080", "port to listen on")
	flag.StringVar(&port, "p", "8080", "port to listen on (shorthand)")

	flag.Parse()

	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser("http://localhost:" + port)
	}()

	log.Printf("serve: %s at http://localhost:%s", dir, port)
	log.Printf("interrupt to exit")

	if err := http.ListenAndServe(":"+port, http.FileServer(http.Dir(dir))); err != nil {
		log.Fatalf("fatal: server: %v", err)
	}
}
