// Command soffio converts a content directory of .soffio files into a static site.
package main

import (
	"embed"
	"flag"
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

//go:embed templates/layout.html
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

// loadTemplate returns the user-supplied layout if present, else the embedded fallback.
func loadTemplate() (*template.Template, error) {
	const name = "templates/layout.html"
	if _, err := os.Stat(name); err == nil {
		log.Printf("info: template: local override")
		return template.ParseFiles(name)
	}
	log.Printf("info: template: embedded fallback")
	return template.ParseFS(assets, name)
}

func main() {
	inDir := flag.String("in", "content", "input directory")
	outDir := flag.String("out", "public", "output directory")
	flag.Parse()

	c := corpus.New()
	if err := c.Load(os.DirFS(*inDir), "*.soffio"); err != nil {
		log.Fatalf("fatal: load: %v", err)
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("fatal: mkdir %s: %v", *outDir, err)
	}

	staticDir := filepath.Join(*inDir, "static")
	if stat, err := os.Stat(staticDir); err == nil && stat.IsDir() {
		dst := filepath.Join(*outDir, "static")
		log.Printf("info: static: copying to %s", dst)
		if err := copyDir(staticDir, dst); err != nil {
			log.Fatalf("fatal: copy static: %v", err)
		}
	}

	for _, err := range c.ValidateLinks() {
		log.Printf("warn: %v", err)
	}

	tmpl, err := loadTemplate()
	if err != nil {
		log.Fatalf("fatal: template: %v", err)
	}

	var wg sync.WaitGroup
	for id, doc := range c.Docs {
		wg.Add(1)
		go func(id string, doc *ast.Document) {
			defer wg.Done()

			var buf strings.Builder
			if err := renderer.Render(&buf, doc); err != nil {
				log.Printf("error: render %s: %v", id, err)
				return
			}

			outPath := filepath.Join(*outDir, id+".html")
			f, err := os.Create(outPath)
			if err != nil {
				log.Printf("error: create %s: %v", outPath, err)
				return
			}
			defer f.Close()

			if err := tmpl.ExecuteTemplate(f, "layout.html", map[string]any{
				"Title":   doc.Title,
				"Meta":    doc.Meta,
				"Content": template.HTML(buf.String()),
			}); err != nil {
				log.Printf("error: execute template %s: %v", id, err)
			}
		}(id, doc)
	}

	wg.Wait()
	log.Printf("done: %d documents written to %s", len(c.Docs), *outDir)
}
