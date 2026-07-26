// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"embed"
	"html/template"
	"log"
	"os"
	"path/filepath"
)

//go:embed templates/*.html templates/*.xml
var embeddedAssets embed.FS

// loadTemplates loads all embedded templates, then overwrites them with any local files
// found in the specified templates directory.
func loadTemplates(dir string) *template.Template {
	tmpl, err := template.ParseFS(embeddedAssets, "templates/*.html", "templates/*.xml")
	if err != nil {
		log.Fatalf("soffio: embedded templates: %v", err)
	}

	if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
		if matches, _ := filepath.Glob(filepath.Join(dir, "*.html")); len(matches) > 0 {
			tmpl, err = tmpl.ParseGlob(filepath.Join(dir, "*.html"))
			if err != nil {
				log.Fatalf("soffio: local html templates: %v", err)
			}
		}
		if matches, _ := filepath.Glob(filepath.Join(dir, "*.xml")); len(matches) > 0 {
			tmpl, err = tmpl.ParseGlob(filepath.Join(dir, "*.xml"))
			if err != nil {
				log.Fatalf("soffio: local xml templates: %v", err)
			}
		}
	}
	return tmpl
}
