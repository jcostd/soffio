// Command preview serves the public directory and opens it in the browser.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	dir := flag.String("d", "public", "directory to serve")
	port := flag.String("p", "8080", "port to listen on")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Soffio Preview - Minimal local web server\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  preview [flags] [dir]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// overwrite -d if positional arg present
	// "preview content_html" is equal to "preview -d content_html"
	if flag.NArg() > 0 {
		*dir = flag.Arg(0)
	}

	addr := "127.0.0.1:" + *port
	url := "http://" + addr

	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := openBrowser(url); err != nil {
			log.Printf("preview: warning: open browser: %v", err)
		}
	}()

	log.Printf("preview: serving %s at %s", *dir, url)
	log.Printf("preview: press Ctrl+C to stop")

	if err := http.ListenAndServe(addr, http.FileServer(http.Dir(*dir))); err != nil {
		log.Fatalf("preview: %v", err)
	}
}
