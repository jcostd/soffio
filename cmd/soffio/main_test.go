package main

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"soffio/ast"
)

func TestWriteDoc(t *testing.T) {
	outDir := t.TempDir()

	tmplStr := `<!DOCTYPE html><html><head><title>{{.Title}}</title></head><body>{{.Content}}</body></html>`
	tmpl, err := template.New("layout.html").Parse(tmplStr)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	doc := &ast.Document{
		ID:    "test-doc",
		Title: "Test Title",
		Meta:  map[string]string{"layout": "layout.html"},
		Sections: []ast.Section{
			{
				Level: 2,
				ID:    "sec-1",
				Title: "Section Title",
				Blocks: []ast.Block{
					ast.TextBlock{
						Line: 1,
						Elements: []ast.Inline{
							ast.PlainText{Content: "Hello World"},
						},
					},
				},
			},
		},
	}

	ctx := &SiteContext{
		BaseURL:        "http://localhost",
		SupportedLangs: []string{"en"},
		OutDir:         outDir,
		Template:       tmpl,
		AllDocs:        map[string]*ast.Document{"test-doc": doc},
	}

	err = ctx.writeDoc("test-doc", doc)
	if err != nil {
		t.Fatalf("writeDoc failed: %v", err)
	}

	outPath := filepath.Join(outDir, "test-doc.html")
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	got := string(content)
	if !strings.Contains(got, "<title>Test Title</title>") {
		t.Errorf("expected Title in output, got: %s", got)
	}
	if !strings.Contains(got, `<section id="sec-1">`) {
		t.Errorf("expected section tag in output, got: %s", got)
	}
	if !strings.Contains(got, `<h2>Section Title</h2>`) {
		t.Errorf("expected header tag in output, got: %s", got)
	}
	if !strings.Contains(got, "<p>Hello World</p>") {
		t.Errorf("expected paragraph HTML in output, got: %s", got)
	}
}

func TestCopyDir(t *testing.T) {
	srcDir := t.TempDir()

	// Create a nested path in a temp dir to explicitly test the creation logic
	baseDstDir := t.TempDir()
	dstDir := filepath.Join(baseDstDir, "static")

	// 1. Create a deep structure in srcDir
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "css", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "css", "deep", "style.css"), []byte("body {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Simulate the fix in main.go: Ensure the destination folder exists before copyDir
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("failed to setup dstDir: %v", err)
	}

	// 3. Execute copyDir
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// 4. Verify that dstDir is actually a directory (prevents the 0-byte file regression)
	info, err := os.Stat(dstDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("dstDir is not a valid directory! The 0-byte file bug occurred.")
	}

	// 5. Verify that nested files exist and match
	if content, err := os.ReadFile(filepath.Join(dstDir, "file.txt")); err != nil || string(content) != "hello" {
		t.Errorf("file.txt not copied correctly")
	}
	if content, err := os.ReadFile(filepath.Join(dstDir, "css", "deep", "style.css")); err != nil || string(content) != "body {}" {
		t.Errorf("css/deep/style.css not copied correctly")
	}
}
