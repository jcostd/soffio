// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package site

import (
	"embed"
	"html/template"
	"io/fs"
	"os"

	"github.com/jcostd/soffio"
)

//go:embed templates
var embedded embed.FS

var patterns = []string{"*.html", "*.xml", "*.txt"}

// funcs are soffio's functions for the templates, what only it knows:
// doc, the page of an ID, or nil; size, an image's, or nil; today, the
// day of the build, or "". What a site wants more, Main adds.
func (s *site) funcs() template.FuncMap {
	return template.FuncMap{
		"doc":   func(id string) *soffio.Document { return s.docs[id] },
		"size":  s.size,
		"today": func() string { return s.today },
	}
}

// loadTemplates parses the templates in dir, or the built-in ones if
// dir is "", with funcs. The two are never mixed: what a site has is
// what is in its directory.
func loadTemplates(dir string, funcs template.FuncMap) (*template.Template, error) {
	fsys, _ := fs.Sub(embedded, "templates")
	if dir != "" {
		fsys = os.DirFS(dir)
	}
	// a key a header doesn't hold is "", as one that holds nothing: a
	// function given .Meta.by gets a string either way
	tmpl := template.New("").Option("missingkey=zero").Funcs(funcs)
	for _, p := range patterns {
		// ParseFS fails on a pattern matching nothing
		if m, _ := fs.Glob(fsys, p); m != nil {
			if _, err := tmpl.ParseFS(fsys, p); err != nil {
				return nil, err
			}
		}
	}
	return tmpl, nil
}
