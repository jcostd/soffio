// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

// Command preview serves a directory, public if none is named, and
// opens it in the browser.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// dirHeader carries the absolute path of the served directory, so that a
// second preview of the same directory can recognise the first one.
const dirHeader = "Soffio-Preview"

func main() {
	log.SetFlags(0)
	port := flag.String("p", "8080", "the `port` to listen on")
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), "usage: preview [-p port] [dir]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	dir := "public"
	switch flag.NArg() {
	case 0:
	case 1:
		dir = flag.Arg(0)
	default:
		flag.Usage()
		os.Exit(2)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		log.Fatalf("preview: %v", err)
	}
	// serving nothing would look like a broken site
	if fi, err := os.Stat(absDir); err != nil {
		log.Fatalf("preview: %v", err)
	} else if !fi.IsDir() {
		log.Fatalf("preview: %s: not a directory", dir)
	}

	addr := "127.0.0.1:" + *port
	url := "http://" + addr

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// the port is taken: if it is a preview of this same directory,
		// it already serves the fresh build, so just show it again
		if servesDir(url, absDir) {
			log.Printf("preview: already serving %s at %s", dir, url)
			openBrowser(url)
			return
		}
		log.Fatalf("preview: %v", err)
	}
	log.Printf("preview: serving %s at %s; Ctrl+C stops it", dir, url)
	openBrowser(url)
	log.Fatalf("preview: %v", http.Serve(listener, handler(absDir)))
}

// types are the media types of what a site holds. Go asks the system
// for them, and the system may be wrong: on Windows a registry that says
// text/plain for .css gets the stylesheet refused. Here they are fixed.
var types = map[string]string{
	".html":  "text/html; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".json":  "application/json",
	".xml":   "application/xml; charset=utf-8",
	".txt":   "text/plain; charset=utf-8",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".webp":  "image/webp",
	".avif":  "image/avif",
	".ico":   "image/x-icon",
	".woff2": "font/woff2",
	".woff":  "font/woff",
	".pdf":   "application/pdf",
}

// handler serves dir and labels every response with it. Nothing is
// kept: the browser shows the last build, not one it cached before
// the site was built again. What the files are is said, not guessed, and the
// address of a private page never leaves for a site it links to.
func handler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set(dirHeader, dir)
		h.Set("Cache-Control", "no-store")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		ext := strings.ToLower(path.Ext(r.URL.Path))
		if strings.HasSuffix(r.URL.Path, "/") {
			ext = ".html"
		}
		if t, ok := types[ext]; ok {
			h.Set("Content-Type", t)
		}
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

// browsers are the commands that open an address in the default
// browser, by system; the others, Linux and the BSDs, have xdg-open.
var browsers = map[string][]string{
	"darwin":  {"open"},
	"windows": {"cmd", "/c", "start", ""},
}

// openBrowser opens url in the default browser, or says why it can't:
// the site is served all the same.
func openBrowser(url string) {
	cmd, ok := browsers[runtime.GOOS]
	if !ok {
		cmd = []string{"xdg-open"}
	}
	if err := exec.Command(cmd[0], append(cmd[1:], url)...).Start(); err != nil {
		log.Printf("preview: %v", err)
	}
}
