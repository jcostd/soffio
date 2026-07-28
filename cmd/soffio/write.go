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

// SiteContext holds the global state required to generate the site.
type SiteContext struct {
	BaseURL        string
	SupportedLangs []string
	OutDir         string
	Template       *template.Template
	AllDocs        map[string]*ast.Document
}

func (ctx *SiteContext) writeDoc(id string, doc *ast.Document, assetPrefix string) error {
	layout := doc.Meta["layout"]
	if layout == "" || ctx.Template.Lookup(layout+".html") == nil {
		layout = "layout"
	}

	var buf strings.Builder
	if err := renderer.Render(&buf, doc, assetPrefix); err != nil {
		return err
	}

	permalink := ctx.BaseURL + "/" + filepath.ToSlash(id) + ".html"
	parts := strings.SplitN(filepath.ToSlash(id), "/", 2)

	type Alternate struct {
		Lang string
		URL  string
	}
	var alternates []Alternate

	if len(parts) == 2 {
		slug := parts[1]
		for _, l := range ctx.SupportedLangs {
			altID := l + "/" + slug
			if _, exists := ctx.AllDocs[filepath.FromSlash(altID)]; exists {
				alternates = append(alternates, Alternate{
					Lang: l,
					URL:  ctx.BaseURL + "/" + altID + ".html",
				})
			}
		}
	}

	outPath := filepath.Join(ctx.OutDir, filepath.FromSlash(id)+".html")
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return ctx.Template.ExecuteTemplate(f, layout+".html", map[string]any{
		"Title":       doc.Title,
		"Meta":        doc.Meta,
		"Content":     template.HTML(buf.String()),
		"AssetPrefix": assetPrefix,
		"Permalink":   permalink,
		"Alternates":  alternates,
	})
}

func (ctx *SiteContext) writeIndex() error {
	outPath := filepath.Join(ctx.OutDir, "index.html")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return ctx.Template.ExecuteTemplate(f, "index.html", map[string]any{
		"Title": "Index",
		"Docs":  ctx.AllDocs,
	})
}

func (ctx *SiteContext) writeFeed() error {
	outPath := filepath.Join(ctx.OutDir, "rss.xml")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return ctx.Template.ExecuteTemplate(f, "rss.xml", map[string]any{
		"Docs":      ctx.AllDocs,
		"XMLHeader": template.HTML("<?xml version=\"1.0\" encoding=\"UTF-8\" ?>\n"),
	})
}

func (ctx *SiteContext) writeSitemap() error {
	if err := os.MkdirAll(ctx.OutDir, 0o755); err != nil {
		return err
	}

	outPath := filepath.Join(ctx.OutDir, "sitemap.xml")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return ctx.Template.ExecuteTemplate(f, "sitemap.xml", map[string]any{
		"Docs":      ctx.AllDocs,
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
