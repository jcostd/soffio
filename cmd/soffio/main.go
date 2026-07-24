// Command soffio converts a content directory of .soffio files into a static site.
package main

import (
	"bytes"
	"embed"
	"flag"
	"html/template"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"soffio/ast"
	"soffio/corpus"
	"soffio/parser"
	"soffio/renderer"
)

//go:embed templates/*.html
var embeddedAssets embed.FS

func main() {
	outDir := flag.String("o", "public", "output directory for size generation")
	tmplDir := flag.String("t", "templates", "local templates direcotry")
	visFlag := flag.String("v", "public", "visibility filter (public, private, all)")
	flag.Parse()

	// pipe mode: (stdin -> stdout)
	if flag.NArg() == 0 {
		src, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("soffio: read stdin: %v", err)
		}
		doc, err := parser.Parse(bytes.NewReader(src))
		if err != nil {
			log.Fatalf("soffio: parse: %v", err)
		}

		if err := renderer.Render(os.Stdout, &doc); err != nil {
			log.Fatalf("soffio: render: %v", err)
		}
		return
	}

	// site generator mode: arg is src directory
	inDir := flag.Arg(0)

	c := corpus.New()
	if err := c.Load(os.DirFS(inDir), "*.soffio"); err != nil {
		log.Fatalf("soffio: load: %v", err)
	}

	if err := c.ValidateLinks(); err != nil {
		log.Fatalf("soffio: link verification failed:\n%v", err)
	}

	tmpl := loadTemplates(*tmplDir)

	staticSrc := filepath.Join(inDir, "static")
	if stat, err := os.Stat(staticSrc); err == nil && stat.IsDir() {
		_ = copyDir(staticSrc, filepath.Join(*outDir, "static"))
	}

	for id, doc := range c.Docs {
		vis := doc.Meta["visibility"]
		if vis == "" {
			vis = "public"
		}
		if *visFlag != "all" && vis != *visFlag {
			continue
		}

		if err := writeDoc(id, doc, tmpl, *outDir); err != nil {
			log.Printf("soffio: err %s: %v", id, err)
		}
	}

	if tmpl.Lookup("index.html") != nil {
		if err := writeIndex(c.Docs, tmpl, *outDir); err != nil {
			log.Printf("soffio: error index: %v", err)
		}
	}
}

// loadTemplates loads all embedded templates, then overwrites them with any local files
// found in the specified templates directory.
func loadTemplates(dir string) *template.Template {
	tmpl, err := template.ParseFS(embeddedAssets, "templates/*.html")
	if err != nil {
		log.Fatalf("soffio: embedded templates: %v", err)
	}
	if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
		tmpl, err = tmpl.ParseGlob(filepath.Join(dir, "*.html"))
		if err != nil {
			log.Fatalf("soffio: local templates: %v", err)
		}
	}
	return tmpl
}

func writeDoc(id string, doc *ast.Document, tmpl *template.Template, outDir string) error {
	layout := doc.Meta["layout"]
	if layout == "" || tmpl.Lookup(layout+".html") == nil {
		layout = "layout"
	}

	var buf strings.Builder
	if err := renderer.Render(&buf, doc); err != nil {
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
