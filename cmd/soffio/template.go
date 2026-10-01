// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"cmp"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"slices"
	"time"

	"soffio"
)

//go:embed templates
var embedded embed.FS

var patterns = []string{"*.html", "*.xml", "*.txt", "*.json"}

var funcs = template.FuncMap{"sortBy": sortBy, "rfc822": rfc822}

// sortBy returns docs sorted by the meta key, highest first, then by
// ID; docs is not changed.
func sortBy(docs []*soffio.Document, key string) []*soffio.Document {
	sorted := slices.Clone(docs)
	slices.SortFunc(sorted, func(a, b *soffio.Document) int {
		return cmp.Or(cmp.Compare(b.Meta[key], a.Meta[key]), cmp.Compare(a.ID, b.ID))
	})
	return sorted
}

// rfc822 turns a YYYY-MM-DD date into the form RSS wants. It is HTML,
// or html/template would write its '+' as &#43;.
func rfc822(date string) (template.HTML, error) {
	t, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return "", err
	}
	return template.HTML(t.Format(time.RFC1123Z)), nil
}

// loadTemplates parses the templates in dir, or the built-in ones if
// dir is "". The two are never mixed: what a site has is what is in
// its directory.
func loadTemplates(dir string) (*template.Template, error) {
	fsys, _ := fs.Sub(embedded, "templates")
	if dir != "" {
		fi, err := os.Stat(dir)
		if err == nil && !fi.IsDir() {
			err = fmt.Errorf("-t %s: not a directory", dir)
		}
		if err != nil {
			return nil, err
		}
		fsys = os.DirFS(dir)
	}
	tmpl := template.New("").Funcs(funcs)
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
