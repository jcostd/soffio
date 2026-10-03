// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

// Command preview serves the public directory and opens it in the browser.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// dirHeader carries the absolute path of the served directory, so that a
// second preview of the same directory can recognise the first one.
const dirHeader = "Soffio-Preview"

func main() {
	log.SetFlags(0)
	dir := flag.String("d", "public", "directory to serve")
	port := flag.String("p", "8080", "port to listen on")

	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), "usage: preview [-d dir] [-p port] [dir]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	switch flag.NArg() {
	case 0:
	case 1:
		*dir = flag.Arg(0)
	default:
		flag.Usage()
		os.Exit(2)
	}

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("preview: %v", err)
	}
	// serving nothing would look like a broken site
	if fi, err := os.Stat(absDir); err != nil {
		log.Fatalf("preview: %v", err)
	} else if !fi.IsDir() {
		log.Fatalf("preview: %s: not a directory", *dir)
	}

	addr := "127.0.0.1:" + *port
	url := "http://" + addr

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// the port is taken: if it is a preview of this same directory,
		// it already serves the fresh build, so just show it again
		if servesDir(url, absDir) {
			log.Printf("preview: already serving %s at %s", *dir, url)
			openBrowser(url)
			return
		}
		log.Fatalf("preview: %v", err)
	}
	log.Printf("preview: serving %s at %s; Ctrl+C stops it", *dir, url)
	openBrowser(url)
	if err := http.Serve(listener, handler(absDir)); err != nil {
		log.Fatalf("preview: %v", err)
	}
}

// handler serves dir and labels every response with it.
func handler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(dirHeader, dir)
		files.ServeHTTP(w, r)
	})
}

// servesDir reports whether the server at url is a preview of dir.
func servesDir(url, dir string) bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Head(url)
	if err != nil {
		return false
	}
	resp.Body.Close()

	// SameFile, not string equality: Windows paths are case-insensitive;
	// no header, no file
	a, errA := os.Stat(resp.Header.Get(dirHeader))
	b, errB := os.Stat(dir)
	return errA == nil && errB == nil && os.SameFile(a, b)
}

// openBrowser opens url in the default browser, or says why it can't:
// the site is served all the same.
func openBrowser(url string) {
	cmd := []string{"xdg-open", url} // Linux and the BSDs
	switch runtime.GOOS {
	case "darwin":
		cmd = []string{"open", url}
	case "windows":
		cmd = []string{"cmd", "/c", "start", "", url}
	}
	if err := exec.Command(cmd[0], cmd[1:]...).Start(); err != nil {
		log.Printf("preview: %v", err)
	}
}
