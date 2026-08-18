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
	"sort"
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

func (ctx *SiteContext) writeDoc(id string, doc *ast.Document) error {
	layout := doc.Meta["layout"]
	if layout == "" || ctx.Template.Lookup(layout+".html") == nil {
		layout = "layout"
	}

	var buf strings.Builder
	if err := renderer.Render(&buf, doc); err != nil {
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

	var children []*ast.Document
	prefix := filepath.ToSlash(id) + "/"
	for cid, cdoc := range ctx.AllDocs {
		if strings.HasPrefix(filepath.ToSlash(cid), prefix) {
			children = append(children, cdoc)
		}
	}
	sort.Slice(children, func(i, j int) bool { return children[i].Title < children[j].Title })

	return ctx.Template.ExecuteTemplate(f, layout+".html", map[string]any{
		"Title":      doc.Title,
		"Meta":       doc.Meta,
		"Content":    template.HTML(buf.String()),
		"BaseURL":    ctx.BaseURL,
		"Permalink":  permalink,
		"Alternates": alternates,
		"Children":   children,
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
// It handles symbolic links transparently.
func copyDir(src, dst string) error {
	realSrc, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}

	return filepath.WalkDir(realSrc, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(realSrc, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		info, err := os.Stat(path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		if !info.Mode().IsRegular() {
			return nil
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
