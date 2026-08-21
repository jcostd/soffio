// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"cmp"
	"embed"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"slices"

	"soffio/ast"
)

//go:embed templates/*.html templates/*.xml templates/*.txt templates/*.json
var embeddedAssets embed.FS

// sortby returns a cloned slice sorted descending by meta key.
// falls back to id (ascending A-Z) for tie-breaking.
func sortBy(docs []*ast.Document, key string) []*ast.Document {
	sorted := slices.Clone(docs)
	slices.SortFunc(sorted, func(a, b *ast.Document) int {
		if r := cmp.Compare(b.Meta[key], a.Meta[key]); r != 0 {
			return r
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return sorted
}

// loadtemplates parses embedded files, overriding with local templates if present.
func loadTemplates(dir string) *template.Template {
	tmpl := template.New("base").Funcs(template.FuncMap{
		"sortBy": sortBy,
	})

	tmpl, err := tmpl.ParseFS(embeddedAssets, "templates/*.html", "templates/*.xml", "templates/*.txt", "templates/*.json")
	if err != nil {
		log.Fatalf("soffio: embedded templates: %v", err)
	}

	if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
		for _, ext := range []string{"*.html", "*.xml", "*.txt", "*.json"} {
			pattern := filepath.Join(dir, ext)
			if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
				if tmpl, err = tmpl.ParseGlob(pattern); err != nil {
					log.Fatalf("soffio: parse %s: %v", ext, err)
				}
			}
		}
	}
	return tmpl
}
