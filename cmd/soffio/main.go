// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

// Command soffio turns a directory of .soffio texts into a static site,
// or one text on standard input into HTML on standard output.
package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"soffio"
)

// Version is set at build time with -ldflags "-X main.Version=...".
var Version = "dev"

// siteFiles are made at the root of the site from the templates of the
// same name, each if its template is there.
var siteFiles = []string{"rss.xml", "sitemap.xml", "robots.txt", "404.html", "manifest.json"}

func main() {
	log.SetFlags(0)
	all := flag.Bool("a", false, "all texts: the private ones too")
	dry := flag.Bool("n", false, "check everything, write nothing")
	baseURL := flag.String("baseurl", "http://localhost:8080", "the site's address, without a final '/'")
	langs := flag.String("langs", "en", "the languages, as the first directories of the IDs")
	outDir := flag.String("o", "public", "the output directory")
	staticDir := flag.String("s", "", "the static files, copied to <o>/static")
	tmplDir := flag.String("t", "", "the templates, instead of the built-in ones")
	version := flag.Bool("v", false, "print the version and exit")
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), `usage: soffio [-a] [-n] [-baseurl url] [-langs l,...] [-o dir] [-s dir] [-t dir] dir
       soffio < text.soffio > text.html
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	switch {
	case *version:
		fmt.Printf("soffio v%s\n", Version)
		return
	case flag.NArg() > 1:
		flag.Usage()
		os.Exit(2)
	case flag.NArg() == 0:
		pipe()
		return
	}

	if err := checkFlags(*baseURL, *langs, *staticDir); err != nil {
		log.Fatalf("soffio: %v", err)
	}
	tmpl, err := loadTemplates(*tmplDir)
	if err != nil {
		log.Fatalf("soffio: %v", err)
	}
	docs, err := soffio.Load(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	active := map[string]*soffio.Document{}
	for id, doc := range docs {
		if *all || doc.Meta["visibility"] != "private" {
			active[id] = doc
		}
	}
	if err := soffio.Check(docs, active, *staticDir); err != nil {
		log.Fatal(err)
	}
	s := &site{
		baseURL: *baseURL,
		langs:   strings.Split(*langs, ","),
		outDir:  *outDir,
		dry:     *dry,
		tmpl:    tmpl,
		docs:    active,
		ids:     slices.Sorted(maps.Keys(active)),
	}
	if *staticDir != "" {
		if err := copyDir(*staticDir, filepath.Join(*outDir, "static"), *dry); err != nil {
			log.Fatalf("soffio: %v", err)
		}
	}

	// every page and file is attempted, but any that fails fails the
	// build: a site missing a page is a broken site
	var errs []error
	for _, id := range s.ids {
		if err := s.writeDoc(active[id]); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", active[id].File, err))
		}
	}
	data := map[string]any{
		"BaseURL":   s.baseURL,
		"Docs":      active,
		"XMLHeader": template.HTML(`<?xml version="1.0" encoding="UTF-8"?>` + "\n"),
	}
	for _, name := range siteFiles {
		if tmpl.Lookup(name) == nil {
			continue
		}
		// 404.soffio would be written, then overwritten
		if id, ok := strings.CutSuffix(name, ".html"); ok && active[id] != nil {
			errs = append(errs, fmt.Errorf("%s: page %s is the site file %s too", active[id].File, id, name))
			continue
		}
		if err := s.write(name, name, data); err != nil {
			errs = append(errs, fmt.Errorf("soffio: %s: %w", name, err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		log.Fatal(err)
	}
}

// pipe turns the text on standard input into HTML on standard output.
// Alone, a text can check only its notes and its own sections.
func pipe() {
	doc, err := soffio.Parse("<stdin>", os.Stdin)
	if err == nil {
		err = soffio.CheckDoc(doc)
	}
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stdout.WriteString(soffio.Render(doc)); err != nil {
		log.Fatalf("soffio: %v", err)
	}
}

// checkFlags refuses what would make a broken site: a base URL that is
// not http(s)://host[/path], or holds what html/template would escape
// in robots.txt or manifest.json, or ends in '/', as every address is
// BaseURL + "/" + path; a language that can't be a directory, or is
// twice; a static directory that is not there.
func checkFlags(baseURL, langs, staticDir string) error {
	if baseURL != "" {
		u, err := url.Parse(baseURL)
		switch {
		case err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" ||
			u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(baseURL, `"'&<>+ `):
			return fmt.Errorf("-baseurl %q: want http(s)://host[/path]", baseURL)
		case strings.HasSuffix(baseURL, "/"):
			return fmt.Errorf("-baseurl %q: drop the final '/'", baseURL)
		}
	}
	seen := map[string]bool{}
	for l := range strings.SplitSeq(langs, ",") {
		if why := soffio.CheckID(l); why != "" {
			return fmt.Errorf("-langs %q: language %q: %s", langs, l, why)
		}
		if seen[l] {
			return fmt.Errorf("-langs %q: language %q twice", langs, l)
		}
		seen[l] = true
	}
	if staticDir == "" {
		return nil
	}
	fi, err := os.Stat(staticDir)
	if err == nil && !fi.IsDir() {
		err = fmt.Errorf("-s %s: not a directory", staticDir)
	}
	return err
}
