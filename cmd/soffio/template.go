// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"cmp"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"soffio"
)

//go:embed templates
var embedded embed.FS

var patterns = []string{"*.html", "*.xml", "*.txt", "*.json"}

// doc, the page of an ID, today, the day of the build, and past:
// site.prepare binds them to the site
var funcs = template.FuncMap{"sortBy": sortBy, "where": where, "under": under, "reverse": reverse, "rfc822": rfc822,
	"doc": func(string) *soffio.Document { return nil }, "today": func() string { return "" },
	"past": func(string) bool { return false }}

// past reports whether date, as precise as it is written, has passed by
// today: 2027-03 has not on 2027-03-31, 2027 not until 2028. Nothing has
// passed on a day not given, as nothing is known of it.
func past(date, today string) bool {
	if today == "" || date == "" || len(date) > len(today) {
		return false
	}
	return today[:len(date)] > date
}

// pages is the list of pages v holds: a list, as .Backlinks, or a map
// by ID, as .Docs, which comes by ID.
func pages(v any) ([]*soffio.Document, error) {
	switch v := v.(type) {
	case []*soffio.Document:
		return v, nil
	case map[string]*soffio.Document:
		docs := make([]*soffio.Document, 0, len(v))
		for _, id := range slices.Sorted(maps.Keys(v)) {
			docs = append(docs, v[id])
		}
		return docs, nil
	case nil:
		return nil, nil
	}
	return nil, fmt.Errorf("%T is not a list of pages", v)
}

// sortBy returns the pages sorted by the header key, highest first,
// then by ID; what it was given is not changed.
func sortBy(v any, key string) ([]*soffio.Document, error) {
	docs, err := pages(v)
	if err != nil {
		return nil, err
	}
	sorted := slices.Clone(docs)
	slices.SortFunc(sorted, func(a, b *soffio.Document) int {
		return cmp.Or(cmp.Compare(b.Meta[key], a.Meta[key]), cmp.Compare(a.ID, b.ID))
	})
	return sorted, nil
}

// where returns the pages whose header key is value, in their order.
// The value of a header link is the ID of its page.
func where(v any, key, value string) ([]*soffio.Document, error) {
	docs, err := pages(v)
	if err != nil {
		return nil, err
	}
	var found []*soffio.Document
	for _, d := range docs {
		if d.Meta[key] == value {
			found = append(found, d)
		}
	}
	return found, nil
}

// under returns the pages under the ID id: under .Docs .ID are a page's
// own, under .Docs .Lang every page of a language. id need not be a
// page; "" is the whole site.
func under(v any, id string) ([]*soffio.Document, error) {
	docs, err := pages(v)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return docs, nil
	}
	var found []*soffio.Document
	for _, d := range docs {
		if strings.HasPrefix(d.ID, id+"/") {
			found = append(found, d)
		}
	}
	return found, nil
}

// reverse returns the pages in the opposite order: lowest first, after
// sortBy.
func reverse(v any) ([]*soffio.Document, error) {
	docs, err := pages(v)
	if err != nil {
		return nil, err
	}
	r := slices.Clone(docs)
	slices.Reverse(r)
	return r, nil
}

// rfc822 turns a date, YYYY-MM-DD or less, into the form RSS wants. It
// is HTML, or html/template would write its '+' as &#43;.
func rfc822(date string) (template.HTML, error) {
	t, err := soffio.ParseDate(date)
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
