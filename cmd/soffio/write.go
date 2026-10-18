// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"soffio"
)

// site is what the pages are made from.
type site struct {
	baseURL string
	today   string // -today, "" if not given
	langs   []string
	outDir  string
	dry     bool // -n: execute, write nothing
	tmpl    *template.Template
	docs    map[string]*soffio.Document
	ids     []string                      // docs' keys, sorted
	back    map[string][]*soffio.Document // the pages linking to each, by ID
}

// prepare gives every page its language: the first directory of its
// ID, if it is one of -langs, or else the first of them (IDs use '/'
// everywhere, so no filepath: on Windows it would turn en/x into en\x).
// It finds, for every page, the pages that link to it, by ID, as s.ids
// is sorted; and it gives the templates doc, the page of an ID, today,
// the day of the build or "", and past, whether a date has passed.
func (s *site) prepare() {
	s.back = map[string][]*soffio.Document{}
	for _, id := range s.ids {
		doc := s.docs[id]
		doc.Lang = s.langs[0]
		if dir, _, ok := strings.Cut(id, "/"); ok && slices.Contains(s.langs, dir) {
			doc.Lang = dir
		}
		for _, to := range soffio.Links(doc) {
			s.back[to] = append(s.back[to], doc)
		}
	}
	s.tmpl.Funcs(template.FuncMap{
		"doc":   func(id string) *soffio.Document { return s.docs[id] },
		"today": func() string { return s.today },
		"past":  func(date string) bool { return past(date, s.today) },
	})
}

type alternate struct{ Lang, URL string }

// writeDoc renders doc through its layout into <outDir>/<id>.html.
func (s *site) writeDoc(doc *soffio.Document) error {
	layout := "layout.html"
	if l := doc.Meta["layout"]; l != "" {
		layout = l + ".html"
	}
	if s.tmpl.Lookup(layout) == nil {
		return fmt.Errorf("no layout %s in the templates", layout)
	}

	var alternates []alternate
	if dir, slug, ok := strings.Cut(doc.ID, "/"); ok && dir == doc.Lang {
		for _, l := range s.langs {
			if s.docs[l+"/"+slug] != nil {
				alternates = append(alternates, alternate{l, s.baseURL + "/" + l + "/" + slug + ".html"})
			}
		}
	}

	return s.write(doc.ID+".html", layout, map[string]any{
		"ID":         doc.ID,
		"Title":      doc.Title,
		"Lang":       doc.Lang,
		"Meta":       doc.Meta,
		"Content":    template.HTML(soffio.Render(doc)),
		"BaseURL":    s.baseURL,
		"Permalink":  s.baseURL + "/" + doc.ID + ".html",
		"Alternates": alternates,
		"Backlinks":  s.back[doc.ID],
		"Docs":       s.docs,
	})
}

// write executes the template name into <outDir>/<file> in one write:
// html/template writes in small pieces, one syscall each.
func (s *site) write(file, name string, data any) error {
	var b bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&b, name, data); err != nil || s.dry {
		return err
	}
	path := filepath.Join(s.outDir, filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o666)
}

// copyDir copies the trees srcs into dst, but hidden files; if dry, it
// only checks that it could. A link to a file is copied as the file; a
// link to a directory is refused, as it may loop. So are a src and dst
// overlapping: a file copied onto itself comes out empty. os.SameFile
// sees through links and letter case. So is one path in two srcs, or
// twice in letter case only: one address, two files.
func copyDir(srcs []string, dst string, dry bool) error {
	// dst, or the nearest directory above it that is there: if src
	// holds that, dst would be inside src
	top, err := os.Stat(dst)
	for d := dst; errors.Is(err, fs.ErrNotExist) && d != filepath.Dir(d); {
		d = filepath.Dir(d)
		top, err = os.Stat(d)
	}
	if err != nil {
		return err
	}
	seen := map[string]string{} // path in dst, folded, to its file
	for _, src := range srcs {
		if err := copyTree(src, dst, top, dry, seen); err != nil {
			return err
		}
	}
	return nil
}

func copyTree(src, dst string, top fs.FileInfo, dry bool, seen map[string]string) error {
	return filepath.WalkDir(src, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fi, err := os.Stat(file)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, file)
		path := filepath.Join(dst, rel)
		if out, err := os.Stat(path); err == nil && os.SameFile(fi, out) || fi.IsDir() && os.SameFile(fi, top) {
			return fmt.Errorf("%s would be copied onto itself: -s and -o overlap", file)
		}
		switch {
		case file != src && strings.HasPrefix(d.Name(), "."):
			// hidden, as for ls: .DS_Store, .git
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		case fi.IsDir() && d.Type()&fs.ModeSymlink != 0:
			return fmt.Errorf("%s: a link to a directory", file)
		case dry && fi.IsDir(), !fi.IsDir() && !fi.Mode().IsRegular():
			return nil
		case fi.IsDir():
			return os.MkdirAll(path, 0o755)
		}
		key := strings.ToLower(filepath.ToSlash(rel))
		if other := seen[key]; other != "" {
			return fmt.Errorf("%s and %s are both /static/%s", other, file, filepath.ToSlash(rel))
		}
		seen[key] = file
		if dry {
			return nil
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		return os.WriteFile(path, b, 0o666)
	})
}
