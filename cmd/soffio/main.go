// Command soffio converts a content directory of .soffio files into a static site.
package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
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

// Version is injected at build time via -ldflags "-X main.Version=..."
var Version = "dev"

//go:embed templates/*.html templates/*.xml
var embeddedAssets embed.FS

func main() {
	outDir := flag.String("o", "public", "output directory for size generation")
	tmplDir := flag.String("t", "templates", "local templates direcotry")
	staticDir := flag.String("s", "static", "static assets directory")
	visFlag := flag.String("vis", "public", "visibility filter (public, private, all)")
	genHTML := flag.Bool("html", true, "generate HTML pages and index")
	genRSS := flag.Bool("rss", true, "generate RSS feed")
	genSitemap := flag.Bool("sitemap", true, "generate XML sitemap")
	showVersion := flag.Bool("version", false, "print version and exit")
	showV := flag.Bool("v", false, "print version and exit (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Soffio - Minimalist Static Site Generator\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  soffio [flags] <src_dir>   (Site generator mode)\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  soffio < input.soffio      (Pipe mode: stdin -> stdout)\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// print version and exit
	if *showVersion || *showV {
		fmt.Printf("soffio v%s\n", Version)
		return
	}

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

	// filter docs for visibility meta
	visibleDocs := make(map[string]*ast.Document)
	for id, doc := range c.Docs {
		vis := doc.Meta["visibility"]
		if vis == "" {
			vis = "public"
		}
		if *visFlag != "all" && vis != *visFlag {
			continue
		}
		visibleDocs[id] = doc
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("soffio: unable to create the output directory: %v", err)
	}

	// html generation
	if *genHTML {
		// copy static dir
		if stat, err := os.Stat(*staticDir); err == nil && stat.IsDir() {
			_ = copyDir(*staticDir, filepath.Join(*outDir, "static"))
		}

		// copy filtered docs
		for id, doc := range visibleDocs {
			if err := writeDoc(id, doc, tmpl, *outDir); err != nil {
				log.Printf("soffio: err %s: %v", id, err)
			}
		}

		if tmpl.Lookup("index.html") != nil {
			if err := writeIndex(visibleDocs, tmpl, *outDir); err != nil {
				log.Printf("soffio: error index: %v", err)
			}
		}
	}

	// rss generation
	if *genRSS && tmpl.Lookup("rss.xml") != nil {
		if err := writeFeed(visibleDocs, tmpl, *outDir); err != nil {
			log.Printf("soffio: error feed: %v", err)
		}
	}

	// sitemap generation
	if *genSitemap && tmpl.Lookup("sitemap.xml") != nil {
		if err := writeSitemap(visibleDocs, tmpl, *outDir); err != nil {
			log.Printf("soffio: error sitemap: %v", err)
		}
	}
}

// loadTemplates loads all embedded templates, then overwrites them with any local files
// found in the specified templates directory.
func loadTemplates(dir string) *template.Template {
	tmpl, err := template.ParseFS(embeddedAssets, "templates/*.html", "templates/*.xml")
	if err != nil {
		log.Fatalf("soffio: embedded templates: %v", err)
	}

	if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
		if matches, _ := filepath.Glob(filepath.Join(dir, "*.html")); len(matches) > 0 {
			tmpl, err = tmpl.ParseGlob(filepath.Join(dir, "*.html"))
			if err != nil {
				log.Fatalf("soffio: local html templates: %v", err)
			}
		}
		if matches, _ := filepath.Glob(filepath.Join(dir, "*.xml")); len(matches) > 0 {
			tmpl, err = tmpl.ParseGlob(filepath.Join(dir, "*.xml"))
			if err != nil {
				log.Fatalf("soffio: local xml templates: %v", err)
			}
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

func writeFeed(docs map[string]*ast.Document, tmpl *template.Template, outDir string) error {
	outPath := filepath.Join(outDir, "rss.xml")
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.ExecuteTemplate(f, "rss.xml", map[string]any{
		"Docs": docs,
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
		"Docs": docs,
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
