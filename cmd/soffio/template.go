// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"cmp"
	"embed"
	"html/template"
	"io/fs"
	"os"
	"slices"

	"soffio"
)

//go:embed templates
var embedded embed.FS

var patterns = []string{"*.html", "*.xml", "*.txt", "*.json"}

// sortBy returns docs sorted by the meta key, highest first, then by
// ID; docs is not changed.
func sortBy(docs []*soffio.Document, key string) []*soffio.Document {
	sorted := slices.Clone(docs)
	slices.SortFunc(sorted, func(a, b *soffio.Document) int {
		return cmp.Or(cmp.Compare(b.Meta[key], a.Meta[key]), cmp.Compare(a.ID, b.ID))
	})
	return sorted
}

// loadTemplates parses the embedded templates, then those in dir, if
// there is one: a template there replaces its namesake.
func loadTemplates(dir string) (*template.Template, error) {
	tmpl := template.New("base").Funcs(template.FuncMap{"sortBy": sortBy})
	sub, _ := fs.Sub(embedded, "templates")
	tmpl = template.Must(tmpl.ParseFS(sub, patterns...))

	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return tmpl, nil
	}
	local := os.DirFS(dir)
	for _, p := range patterns {
		// ParseFS fails on a pattern matching nothing
		if m, _ := fs.Glob(local, p); m != nil {
			if _, err := tmpl.ParseFS(local, p); err != nil {
				return nil, err
			}
		}
	}
	return tmpl, nil
}
