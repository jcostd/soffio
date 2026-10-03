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
	langs   []string
	outDir  string
	dry     bool // -n: execute, write nothing
	tmpl    *template.Template
	docs    map[string]*soffio.Document
	ids     []string // docs' keys, sorted
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

	// IDs use '/' everywhere, so no filepath here: on Windows it
	// would turn en/x into en\x and find nothing
	var alternates []alternate
	if lang, slug, ok := strings.Cut(doc.ID, "/"); ok && slices.Contains(s.langs, lang) {
		for _, l := range s.langs {
			if s.docs[l+"/"+slug] != nil {
				alternates = append(alternates, alternate{l, s.baseURL + "/" + l + "/" + slug + ".html"})
			}
		}
	}

	// by ID, or a map would shuffle them on every build; sorted, the
	// IDs under id/ are one run
	var children []*soffio.Document
	prefix := doc.ID + "/"
	i, _ := slices.BinarySearch(s.ids, prefix)
	for _, id := range s.ids[i:] {
		if !strings.HasPrefix(id, prefix) {
			break
		}
		children = append(children, s.docs[id])
	}

	return s.write(doc.ID+".html", layout, map[string]any{
		"Title":      doc.Title,
		"Meta":       doc.Meta,
		"Content":    template.HTML(soffio.Render(doc)),
		"BaseURL":    s.baseURL,
		"Permalink":  s.baseURL + "/" + doc.ID + ".html",
		"Alternates": alternates,
		"Children":   children,
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

// copyDir copies the tree src into dst, but hidden files; if dry, it
// only checks that it could. A link to a file is copied as the file; a
// link to a directory is refused, as it may loop. So are src and dst
// overlapping: a file copied onto itself comes out empty. os.SameFile
// sees through links and letter case.
func copyDir(src, dst string, dry bool) error {
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
		case dry || !fi.IsDir() && !fi.Mode().IsRegular():
			return nil
		case fi.IsDir():
			return os.MkdirAll(path, 0o755)
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		return os.WriteFile(path, b, 0o666)
	})
}
