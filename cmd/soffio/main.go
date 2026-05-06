// Command soffio converts a content directory of .soffio files into a static site.
package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"soffio/ast"
	"soffio/corpus"
	"soffio/renderer"
)

//go:embed templates/*.html
var assets embed.FS

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// copyDir mirrors src into dst, preserving the directory tree.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
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

		return copyFile(path, target)
	})
}

// loadTemplates loads all embedded templates, then overwrites them with any local files
// found in the specified templates directory.
func loadTemplates(dir string) (*template.Template, error) {
	tmpl, err := template.ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("embedded templates: %w", err)
	}

	if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
		log.Printf("info: template: parsing local overrides from %s/", dir)
		pattern := filepath.Join(dir, "*.html")
		tmpl, err = tmpl.ParseGlob(pattern)
		if err != nil {
			return nil, fmt.Errorf("local templates in %s: %w", dir, err)
		}
	} else {
		log.Printf("info: template: directory '%s' not found, using embedded fallbacks", dir)
	}

	return tmpl, nil
}

func main() {
	var (
		inDir   string
		outDir  string
		tmplDir string
	)

	flag.StringVar(&inDir, "in", "content", "input directory")
	flag.StringVar(&inDir, "i", "content", "input directory (shorthand)")

	flag.StringVar(&outDir, "out", "public", "output directory")
	flag.StringVar(&outDir, "o", "public", "output directory (shorthand)")

	flag.StringVar(&tmplDir, "templates", "templates", "templates directory containing layouts")
	flag.StringVar(&tmplDir, "t", "templates", "templates directory containing layouts (shorthand)")

	flag.Parse()

	c := corpus.New()
	if err := c.Load(os.DirFS(inDir), "*.soffio"); err != nil {
		log.Fatalf("fatal: load: %v", err)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("fatal: mkdir %s: %v", outDir, err)
	}

	staticDir := filepath.Join(inDir, "static")
	if stat, err := os.Stat(staticDir); err == nil && stat.IsDir() {
		dst := filepath.Join(outDir, "static")
		log.Printf("info: static: copying to %s", dst)
		if err := copyDir(staticDir, dst); err != nil {
			log.Fatalf("fatal: copy static: %v", err)
		}
	}

	for _, err := range c.ValidateLinks() {
		log.Printf("warn: %v", err)
	}

	tmpl, err := loadTemplates(tmplDir)
	if err != nil {
		log.Fatalf("fatal: load templates: %v", err)
	}

	var wg sync.WaitGroup
	for id, doc := range c.Docs {
		wg.Add(1)
		go func(id string, doc *ast.Document) {
			defer wg.Done()

			layoutName := doc.Meta["layout"]
			if layoutName == "" {
				layoutName = "layout"
			}
			templateTarget := layoutName + ".html"

			var buf strings.Builder
			if err := renderer.Render(&buf, doc); err != nil {
				log.Printf("error: render %s: %v", id, err)
				return
			}

			// Isomorphic output generation.
			outPath := filepath.Join(outDir, filepath.FromSlash(id)+".html")
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				log.Printf("error: mkdir %s: %v", filepath.Dir(outPath), err)
				return
			}

			f, err := os.Create(outPath)
			if err != nil {
				log.Printf("error: create %s: %v", outPath, err)
				return
			}
			defer f.Close()

			if err := tmpl.ExecuteTemplate(f, templateTarget, map[string]any{
				"Title":   doc.Title,
				"Meta":    doc.Meta,
				"Content": template.HTML(buf.String()),
			}); err != nil {
				log.Printf("error: execute template %s for %s: %v", templateTarget, id, err)
			}
		}(id, doc)
	}

	wg.Wait()
	log.Printf("done: %d documents written to %s", len(c.Docs), outDir)
}
