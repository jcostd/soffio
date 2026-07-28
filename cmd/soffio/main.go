// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

// Command soffio converts a content directory of .soffio files into a static site.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"soffio/ast"
	"soffio/corpus"
	"soffio/parser"
	"soffio/renderer"
)

// Version is injected at build time via -ldflags "-X main.Version=..."
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
	showVersion := flag.Bool("version", false, "print version and exit")
	showV := flag.Bool("v", false, "print version and exit (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Soffio - Minimalist Static Site Generator\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  soffio [flags] <src_dir>   (Site generator mode)\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  soffio < input.soffio      (Pipe mode: stdin -> stdout)\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// print version and exit
	if *showVersion || *showV {
		fmt.Printf("soffio v%s\n", Version)
		return
	}

	// pipe mode: (stdin -> stdout)
	if flag.NArg() == 0 {
		src, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("soffio: read stdin: %v", err)
		}
		doc, err := parser.Parse(bytes.NewReader(src))
		if err != nil {
			log.Fatalf("soffio: parse: %v", err)
		}

		if err := renderer.Render(os.Stdout, &doc, ""); err != nil {
			log.Fatalf("soffio: render: %v", err)
		}
		return
	}

	// site generator mode: arg is src directory
	inDir := flag.Arg(0)

	c := corpus.New()
	skipDir := filepath.Base(*staticDir)
	if err := c.Load(os.DirFS(inDir), "*.soffio", skipDir); err != nil {
		log.Fatalf("soffio: load: %v", err)
	}

	// filter docs for visibility meta
	visibleDocs := make(map[string]*ast.Document)
	for id, doc := range c.Docs {
		vis := doc.Meta["visibility"]
		if vis == "" {
			vis = "public"
		}
		if *visFlag != "all" && vis != *visFlag {
			continue
		}
		visibleDocs[id] = doc
	}

	if err := c.ValidateLinks(visibleDocs, *staticDir); err != nil {
		log.Fatalf("soffio: link verification failed:\n%v", err)
	}

	tmpl := loadTemplates(*tmplDir)

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("soffio: unable to create the output directory: %v", err)
	}

	supportedLangs := strings.Split(*langs, ",")
	ctx := &SiteContext{
		BaseURL:        *baseURL,
		SupportedLangs: supportedLangs,
		OutDir:         *outDir,
		Template:       tmpl,
		AllDocs:        visibleDocs,
	}

	// html generation
	if *genHTML {
		// copy static dir
		if stat, err := os.Stat(*staticDir); err == nil && stat.IsDir() {
			_ = copyDir(*staticDir, filepath.Join(*outDir, "static"))
		}

		assetPrefix := "/" + filepath.ToSlash(filepath.Base(*staticDir))
		for id, doc := range visibleDocs {
			if err := ctx.writeDoc(id, doc, assetPrefix); err != nil {
				log.Printf("soffio: err %s: %v", id, err)
			}
		}

		if tmpl.Lookup("index.html") != nil {
			if err := ctx.writeIndex(); err != nil {
				log.Printf("soffio: error index: %v", err)
			}
		}
	}

	// rss generation
	if *genRSS && tmpl.Lookup("rss.xml") != nil {
		if err := ctx.writeFeed(); err != nil {
			log.Printf("soffio: error feed: %v", err)
		}
	}

	// sitemap generation
	if *genSitemap && tmpl.Lookup("sitemap.xml") != nil {
		if err := ctx.writeSitemap(); err != nil {
			log.Printf("soffio: error sitemap: %v", err)
		}
	}
}
