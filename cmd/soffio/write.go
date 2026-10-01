// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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
	IDs            []string // AllDocs' keys, sorted
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

	permalink := ctx.BaseURL + "/" + id + ".html"

	type Alternate struct {
		Lang string
		URL  string
	}
	var alternates []Alternate

	// IDs use '/' everywhere, so no filepath here: on Windows it
	// would turn en/x into en\x and find nothing
	if lang, slug, ok := strings.Cut(id, "/"); ok && slices.Contains(ctx.SupportedLangs, lang) {
		for _, l := range ctx.SupportedLangs {
			if _, ok := ctx.AllDocs[l+"/"+slug]; ok {
				alternates = append(alternates, Alternate{
					Lang: l,
					URL:  ctx.BaseURL + "/" + l + "/" + slug + ".html",
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

	// by ID, or a map would shuffle them on every build; sorted, the
	// IDs under id/ are one run
	var children []*ast.Document
	i, _ := slices.BinarySearch(ctx.IDs, id+"/")
	for _, cid := range ctx.IDs[i:] {
		if !strings.HasPrefix(cid, id+"/") {
			break
		}
		children = append(children, ctx.AllDocs[cid])
	}

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
		"BaseURL":   ctx.BaseURL,
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
		"BaseURL":   ctx.BaseURL,
		"Docs":      ctx.AllDocs,
		"XMLHeader": template.HTML("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"),
	})
}

func (ctx *SiteContext) writeRobots() error {
	outPath := filepath.Join(ctx.OutDir, "robots.txt")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return ctx.Template.ExecuteTemplate(f, "robots.txt", map[string]any{
		"BaseURL": ctx.BaseURL,
	})
}

func (ctx *SiteContext) write404() error {
	outPath := filepath.Join(ctx.OutDir, "404.html")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return ctx.Template.ExecuteTemplate(f, "404.html", map[string]any{
		"BaseURL": ctx.BaseURL,
	})
}

func (ctx *SiteContext) writeManifest() error {
	outPath := filepath.Join(ctx.OutDir, "manifest.json")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return ctx.Template.ExecuteTemplate(f, "manifest.json", map[string]any{
		"BaseURL": ctx.BaseURL,
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
