// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// site is what the pages are made from.
type site struct {
	baseURL string
	today   string // -today, "" if not given
	outDir  string
	dry     bool // -n: execute, write nothing
	tmpl    *template.Template
	docs    map[string]*Document
	ids     []string                 // docs' keys, sorted
	static  []string                 // the -s dirs
	sizes   map[string]*image.Config // size's answers, by path
}

// prepare gives every image of the texts its size.
func (s *site) prepare() {
	s.sizes = map[string]*image.Config{}
	for _, id := range s.ids {
		doc := s.docs[id]
		for i := range doc.Sections {
			for j, b := range doc.Sections[i].Blocks {
				if img, ok := b.(ImageBlock); ok {
					if sz := s.size(img.Path); sz != nil {
						img.Width, img.Height = sz.Width, sz.Height
						doc.Sections[i].Blocks[j] = img
					}
				}
			}
		}
	}
}

// size returns the size of the image at path, /static/..., in pixels,
// as its Width and Height: with it the browser keeps the image's place
// before it comes. It is read from its file in the -s dirs, once, and
// only as far as its size; nil for an address out of the site, or a
// kind the standard library doesn't read (svg, webp).
func (s *site) size(path string) *image.Config {
	if sz, ok := s.sizes[path]; ok {
		return sz
	}
	var sz *image.Config
	if file, ok := strings.CutPrefix(path, "/static/"); ok {
		for _, dir := range s.static {
			f, err := os.Open(filepath.Join(dir, filepath.FromSlash(file)))
			if err != nil {
				continue
			}
			if c, _, err := image.DecodeConfig(f); err == nil {
				sz = &c
			}
			f.Close()
			break
		}
	}
	s.sizes[path] = sz
	return sz
}

// writeDoc renders doc through its layout into <outDir>/<id>.html, in
// one write: html/template writes in small pieces, one syscall each.
func (s *site) writeDoc(doc *Document) error {
	layout := "layout.html"
	if l := doc.Meta["layout"]; l != "" {
		layout = l + ".html"
	}
	if s.tmpl.Lookup(layout) == nil {
		return fmt.Errorf("no layout %s in the templates", layout)
	}
	var b bytes.Buffer
	err := s.tmpl.ExecuteTemplate(&b, layout, map[string]any{
		"ID":      doc.ID,
		"Meta":    doc.Meta,
		"Content": template.HTML(Render(doc)),
		"BaseURL": s.baseURL,
		"Docs":    s.docs,
	})
	if err != nil || s.dry {
		return err
	}
	path := filepath.Join(s.outDir, filepath.FromSlash(doc.ID+".html"))
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

//go:embed templates
var embedded embed.FS

// funcs are soffio's functions for the templates, what only it knows:
// doc, the page of an ID, or nil; href, the address of a target, as a
// text writes it, from a page, relative as in the texts; size, an
// image's, or nil; today, the day of the build, or "". What a site
// wants more, Main adds.
func (s *site) funcs() template.FuncMap {
	return template.FuncMap{
		"doc":   func(id string) *Document { return s.docs[id] },
		"href":  href,
		"size":  s.size,
		"today": func() string { return s.today },
	}
}

// loadTemplates parses the templates in dir, its .html files, or the
// built-in ones if dir is "", with funcs. The two are never mixed: what
// a site has is what is in its directory.
func loadTemplates(dir string, funcs template.FuncMap) (*template.Template, error) {
	fsys, _ := fs.Sub(embedded, "templates")
	if dir != "" {
		fsys = os.DirFS(dir)
	}
	// a key a header doesn't hold is "", as one that holds nothing: a
	// function given .Meta.by gets a string either way
	return template.New("").Option("missingkey=zero").Funcs(funcs).ParseFS(fsys, "*.html")
}
