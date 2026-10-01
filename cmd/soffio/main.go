// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

// Command soffio converts a content directory of .soffio files into a static site.
package main

import (
	"cmp"
	"flag"
	"fmt"
	"html/template"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"soffio"
)

// Version is set at build time with -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	baseURL := flag.String("baseurl", "http://localhost:8080", "The absolute base URL of the site")
	langs := flag.String("langs", "en", "Comma-separated list of supported languages")
	outDir := flag.String("o", "public", "output directory for site generation")
	tmplDir := flag.String("t", "templates", "local templates directory")
	staticDir := flag.String("s", "static", "static assets directory")
	visFlag := flag.String("vis", "public", "visibility filter (public, private, all)")
	genHTML := flag.Bool("html", true, "generate HTML pages and index")
	genRSS := flag.Bool("rss", true, "generate RSS feed")
	genSitemap := flag.Bool("sitemap", true, "generate XML sitemap")
	genRobots := flag.Bool("robots", true, "generate robots.txt")
	genErrorPage := flag.Bool("errpage", true, "generate error 404 page")
	genManifest := flag.Bool("manifest", true, "generate manifest.json")
	showVersion := flag.Bool("version", false, "print version and exit")
	showV := flag.Bool("v", false, "print version and exit (shorthand)")

	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), `Soffio - Minimalist Static Site Generator

Usage:
  soffio [flags] <src_dir>   (Site generator mode)
  soffio < input.soffio      (Pipe mode: stdin -> stdout)

Flags:
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion || *showV {
		fmt.Printf("soffio v%s\n", Version)
		return
	}

	// pipe mode: stdin -> stdout
	if flag.NArg() == 0 {
		doc, err := soffio.Parse(os.Stdin)
		if err != nil {
			log.Fatalf("soffio: parse: %v", err)
		}
		if err := soffio.Render(os.Stdout, &doc); err != nil {
			log.Fatalf("soffio: render: %v", err)
		}
		return
	}

	if err := checkBaseURL(*baseURL); err != nil {
		log.Fatalf("soffio: %v", err)
	}
	all, err := soffio.Load(os.DirFS(flag.Arg(0)), filepath.Base(*staticDir))
	if err != nil {
		log.Fatalf("soffio: load: %v", err)
	}
	docs := map[string]*soffio.Document{}
	for id, doc := range all {
		if vis := cmp.Or(doc.Meta["visibility"], "public"); *visFlag == "all" || vis == *visFlag {
			docs[id] = doc
		}
	}
	if err := soffio.Check(all, docs, *staticDir); err != nil {
		log.Fatalf("soffio: link verification failed:\n%v", err)
	}

	tmpl, err := loadTemplates(*tmplDir)
	if err != nil {
		log.Fatalf("soffio: templates: %v", err)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("soffio: unable to create the output directory: %v", err)
	}
	s := &site{
		baseURL: *baseURL,
		langs:   strings.Split(*langs, ","),
		outDir:  *outDir,
		tmpl:    tmpl,
		docs:    docs,
		ids:     slices.Sorted(maps.Keys(docs)),
	}

	// every page and file is attempted, but any that fails fails the
	// build: a site missing a page is a broken site
	failed := false

	if *genHTML {
		if fi, err := os.Stat(*staticDir); err == nil && fi.IsDir() {
			if err := copyDir(*staticDir, filepath.Join(*outDir, "static")); err != nil {
				log.Fatalf("soffio: failed to copy static assets: %v", err)
			}
		}
		for _, id := range s.ids {
			if err := s.writeDoc(docs[id]); err != nil {
				log.Printf("soffio: err %s: %v", id, err)
				failed = true
			}
		}
	}

	data := map[string]any{
		"BaseURL":   s.baseURL,
		"Docs":      docs,
		"XMLHeader": template.HTML(`<?xml version="1.0" encoding="UTF-8"?>` + "\n"),
	}
	for _, f := range []struct {
		name string
		on   bool
	}{
		{"rss.xml", *genRSS},
		{"sitemap.xml", *genSitemap},
		{"robots.txt", *genRobots},
		{"404.html", *genErrorPage},
		{"manifest.json", *genManifest},
	} {
		// a file is made only if its template exists
		if !f.on || tmpl.Lookup(f.name) == nil {
			continue
		}
		if err := s.write(f.name, f.name, data); err != nil {
			log.Printf("soffio: error %s: %v", f.name, err)
			failed = true
		}
	}

	if failed {
		os.Exit(1)
	}
}

// checkBaseURL refuses a base URL ending in '/': every address is
// BaseURL + "/" + path, and x.org//a.html is not x.org/a.html.
func checkBaseURL(u string) error {
	if strings.HasSuffix(u, "/") {
		return fmt.Errorf("-baseurl %q: drop the trailing '/'", u)
	}
	return nil
}
