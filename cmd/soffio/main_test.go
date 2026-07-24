package main

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"soffio/ast"
)

// TestWriteDoc verifies that a real AST document is correctly rendered
// and injected into the target HTML layout.
func TestWriteDoc(t *testing.T) {
	outDir := t.TempDir()

	// 1. Prepare a dummy template
	tmplStr := `<!DOCTYPE html><html><head><title>{{.Title}}</title></head><body>{{.Content}}</body></html>`
	tmpl, err := template.New("layout.html").Parse(tmplStr)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	// 2. Prepare a mocked AST document using actual concrete types
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

	// 3. Execute writeDoc (which relies on renderer.Render internally)
	err = writeDoc("test-doc", doc, tmpl, outDir)
	if err != nil {
		t.Fatalf("writeDoc failed: %v", err)
	}

	// 4. Verify the output file
	outPath := filepath.Join(outDir, "test-doc.html")
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	got := string(content)
	if !strings.Contains(got, "<title>Test Title</title>") {
		t.Errorf("expected Title in output, got: %s", got)
	}

	// The renderer adds <section> and <h2> tags based on the ast.Section
	if !strings.Contains(got, `<section id="sec-1">`) {
		t.Errorf("expected section tag in output, got: %s", got)
	}
	if !strings.Contains(got, `<h2>Section Title</h2>`) {
		t.Errorf("expected header tag in output, got: %s", got)
	}

	// The TextBlock is rendered as a <p> tag
	if !strings.Contains(got, "<p>Hello World</p>") {
		t.Errorf("expected paragraph HTML in output, got: %s", got)
	}
}

// TestWriteIndex verifies that the site index receives the document map correctly.
func TestWriteIndex(t *testing.T) {
	outDir := t.TempDir()

	// 1. Dummy index template
	tmplStr := `<ul>{{range $id, $doc := .Docs}}<li>{{$id}}: {{$doc.Title}}</li>{{end}}</ul>`
	tmpl, err := template.New("index.html").Parse(tmplStr)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	// 2. Map of mock documents
	docs := map[string]*ast.Document{
		"post1": {Title: "First Post"},
		"post2": {Title: "Second Post"},
	}

	// 3. Execute writeIndex
	err = writeIndex(docs, tmpl, outDir)
	if err != nil {
		t.Fatalf("writeIndex failed: %v", err)
	}

	// 4. Verify the output
	outPath := filepath.Join(outDir, "index.html")
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read index file: %v", err)
	}

	got := string(content)
	if !strings.Contains(got, "post1: First Post") || !strings.Contains(got, "post2: Second Post") {
		t.Errorf("expected docs in index, got: %s", got)
	}
}

// TestCopyDir verifies that directories and static files are copied properly.
func TestCopyDir(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// 1. Create a dummy structure in srcDir (simulating a "static" folder)
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(srcDir, "css"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "css", "style.css"), []byte("body {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Execute copyDir
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// 3. Verify that the files exist and match in dstDir
	if content, err := os.ReadFile(filepath.Join(dstDir, "file.txt")); err != nil || string(content) != "hello" {
		t.Errorf("file.txt not copied correctly")
	}
	if content, err := os.ReadFile(filepath.Join(dstDir, "css", "style.css")); err != nil || string(content) != "body {}" {
		t.Errorf("css/style.css not copied correctly")
	}
}
