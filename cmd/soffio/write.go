// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
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

// copyDir copies the tree src into dst. A link to a file is copied as
// the file; a link to a directory is refused, as it may loop.
func copyDir(src, dst string) error {
	fsys := os.DirFS(src)
	return fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fi, err := fs.Stat(fsys, name)
		if err != nil {
			return err
		}
		path := filepath.Join(dst, filepath.FromSlash(name))
		switch {
		case fi.IsDir() && d.Type()&fs.ModeSymlink != 0:
			return fmt.Errorf("%s: a link to a directory", filepath.Join(src, name))
		case fi.IsDir():
			return os.MkdirAll(path, 0o755)
		case !fi.Mode().IsRegular():
			return nil
		}

		in, err := fsys.Open(name)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}
