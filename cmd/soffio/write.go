// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"soffio/ast"
	"soffio/renderer"
)

func writeDoc(id string, doc *ast.Document, tmpl *template.Template, outDir string, assetPrefix string) error {
	layout := doc.Meta["layout"]
	if layout == "" || tmpl.Lookup(layout+".html") == nil {
		layout = "layout"
	}

	var buf strings.Builder
	if err := renderer.Render(&buf, doc, assetPrefix); err != nil {
		return err
	}

	outPath := filepath.Join(outDir, filepath.FromSlash(id)+".html")
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.ExecuteTemplate(f, layout+".html", map[string]any{
		"Title":   doc.Title,
		"Meta":    doc.Meta,
		"Content": template.HTML(buf.String()),
	})
}

func writeIndex(docs map[string]*ast.Document, tmpl *template.Template, outDir string) error {
	outPath := filepath.Join(outDir, "index.html")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.ExecuteTemplate(f, "index.html", map[string]any{
		"Title": "Index",
		"Docs":  docs,
	})
}

func writeFeed(docs map[string]*ast.Document, tmpl *template.Template, outDir string) error {
	outPath := filepath.Join(outDir, "rss.xml")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.ExecuteTemplate(f, "rss.xml", map[string]any{
		"Docs":      docs,
		"XMLHeader": template.HTML("<?xml version=\"1.0\" encoding=\"UTF-8\" ?>\n"),
	})
}

func writeSitemap(docs map[string]*ast.Document, tmpl *template.Template, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	outPath := filepath.Join(outDir, "sitemap.xml")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.ExecuteTemplate(f, "sitemap.xml", map[string]any{
		"Docs":      docs,
		"XMLHeader": template.HTML("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"),
	})
}

// copyDir mirrors src into dst, preserving the directory tree.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		return err
	})
}
