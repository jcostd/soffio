// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

// Package site is the command soffio: a directory of .soffio texts
// into a static site, or one text on standard input into HTML on
// standard output. A site that wants more of it, functions for its
// templates or rules for its texts, is a command of its own that calls
// Main with them: soffio itself is Main(nil, nil).
package site

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
	"time"

	"github.com/jcostd/soffio/v5"
)

// siteFiles are made at the root of the site from the templates of the
// same name, each if its template is there.
var siteFiles = []string{"index.html", "404.html", "robots.txt", "sitemap.xml"}

// Main is the command, named as it was run. funcs are added to the
// templates' functions, and check is given the texts to build once
// soffio has found nothing wrong with them: what it returns is wrong
// too. Either may be nil.
func Main(funcs template.FuncMap, check func(docs map[string]*soffio.Document) error) {
	name, _, _ := strings.Cut(filepath.Base(os.Args[0]), ".")
	log.SetFlags(0)
	all := flag.Bool("a", false, "all texts: the private ones too")
	dry := flag.Bool("n", false, "check everything, write nothing")
	baseURL := flag.String("baseurl", "", "the site's `url`, without a final '/'")
	outDir := flag.String("o", "public", "the output `dir`")
	var staticDirs []string
	flag.Func("s", "a `dir` of static files, copied to <o>/static; -s again adds one", func(dir string) error {
		if dir == "" {
			return errors.New("no directory")
		}
		staticDirs = append(staticDirs, dir)
		return nil
	})
	tmplDir := flag.String("t", "", "the templates' `dir`, instead of the built-in ones")
	today := flag.String("today", "", "the day of the build, the templates' today: `YYYY-MM-DD`")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `usage: %[1]s [-a] [-n] [-baseurl url] [-o dir] [-s dir]... [-t dir] [-today day] dir
       %[1]s [-n] < text.soffio > text.html
`, name)
		flag.PrintDefaults()
	}
	flag.Parse()

	switch flag.NArg() {
	case 0:
		// alone, a text has no site: of the flags, only -n means something
		flag.Visit(func(f *flag.Flag) {
			if f.Name != "n" {
				log.Printf("%s: -%s needs a dir", name, f.Name)
				os.Exit(2)
			}
		})
		pipe(name, *dry)
		return
	case 1:
	default:
		flag.Usage()
		os.Exit(2)
	}

	if err := checkFlags(*baseURL, *today, staticDirs, *tmplDir, *outDir); err != nil {
		log.Fatalf("%s: %v", name, err)
	}
	s := &site{baseURL: *baseURL, today: *today, outDir: *outDir, dry: *dry, static: staticDirs}
	fm := s.funcs()
	for f, fn := range funcs {
		if fm[f] != nil {
			log.Fatalf("%s: the templates' function %s is soffio's", name, f)
		}
		fm[f] = fn
	}
	tmpl, err := loadTemplates(*tmplDir, fm)
	if err != nil {
		log.Fatalf("%s: %v", name, err)
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
	if err := soffio.Check(docs, active, staticDirs); err != nil {
		log.Fatal(err)
	}
	if check != nil {
		if err := check(active); err != nil {
			log.Fatal(err)
		}
	}
	s.tmpl, s.docs, s.ids = tmpl, active, slices.Sorted(maps.Keys(active))
	s.prepare()
	if err := copyDir(staticDirs, filepath.Join(*outDir, "static"), *dry); err != nil {
		log.Fatalf("%s: %v", name, err)
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
	for _, f := range siteFiles {
		if tmpl.Lookup(f) == nil {
			continue
		}
		// 404.soffio would be written, then overwritten
		if id, ok := strings.CutSuffix(f, ".html"); ok && active[id] != nil {
			errs = append(errs, fmt.Errorf("%s: page %s is the site file %s too", active[id].File, id, f))
			continue
		}
		if err := s.write(f, f, data); err != nil {
			errs = append(errs, fmt.Errorf("%s: %s: %w", name, f, err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		log.Fatal(err)
	}
}

// pipe turns the text on standard input into HTML on standard output,
// or with -n only checks it. Alone, a text can check only its notes and
// its own sections.
func pipe(name string, dry bool) {
	doc, err := soffio.Parse("<stdin>", os.Stdin)
	if err == nil {
		err = soffio.CheckDoc(doc)
	}
	if err != nil {
		log.Fatal(err)
	}
	if dry {
		return
	}
	if _, err := os.Stdout.WriteString(soffio.Render(doc)); err != nil {
		log.Fatalf("%s: %v", name, err)
	}
}

// checkFlags refuses what would make a broken site: a base URL, if
// there is one, that is not http(s)://host[/path], or holds what
// html/template would escape in robots.txt, or ends in '/', as every
// address is BaseURL + "/" + path; a day that is not YYYY-MM-DD (the
// templates' only clock is what they are told: the same flags make the
// same site); a static or template directory that is not there;
// templates in -o, where the site files would be written over them.
func checkFlags(baseURL, today string, staticDirs []string, tmplDir, outDir string) error {
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
	if _, err := time.Parse(time.DateOnly, today); today != "" && err != nil {
		return fmt.Errorf("-today %q: want YYYY-MM-DD", today)
	}
	dirs := [][2]string{{"-t", tmplDir}}
	for _, d := range staticDirs {
		dirs = append(dirs, [2]string{"-s", d})
	}
	for _, f := range dirs {
		if f[1] == "" {
			continue
		}
		if fi, err := os.Stat(f[1]); err != nil {
			return err
		} else if !fi.IsDir() {
			return fmt.Errorf("%s %s: not a directory", f[0], f[1])
		}
	}
	t, errT := os.Stat(tmplDir)
	o, errO := os.Stat(outDir)
	if tmplDir != "" && errT == nil && errO == nil && os.SameFile(t, o) {
		return fmt.Errorf("-t %s is -o %s: the site would be written over its templates", tmplDir, outDir)
	}
	return nil
}
