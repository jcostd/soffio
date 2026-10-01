package main

import (
	"html/template"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"soffio"
)

func TestWriteDoc(t *testing.T) {
	outDir := t.TempDir()

	tmplStr := `<!DOCTYPE html><html><head><title>{{.Title}}</title></head><body>{{.Content}}</body></html>`
	tmpl, err := template.New("layout.html").Parse(tmplStr)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	doc := &soffio.Document{
		ID:    "test-doc",
		Title: "Test Title",
		Meta:  map[string]string{"layout": "layout.html"},
		Sections: []soffio.Section{
			{
				Level: 2,
				ID:    "sec-1",
				Title: "Section Title",
				Blocks: []soffio.Block{
					soffio.TextBlock{
						Line: 1,
						Elements: []soffio.Inline{
							soffio.PlainText{Content: "Hello World"},
						},
					},
				},
			},
		},
	}

	s := &site{
		baseURL: "http://localhost",
		langs:   []string{"en"},
		outDir:  outDir,
		tmpl:    tmpl,
		docs:    map[string]*soffio.Document{"test-doc": doc},
		ids:     []string{"test-doc"},
	}

	err = s.writeDoc(doc)
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

func TestWriteDocAlternatesChildren(t *testing.T) {
	tmpl := template.Must(template.New("layout.html").Parse(
		`{{range .Alternates}}{{.Lang}}={{.URL}};{{end}}|{{range .Children}}{{.ID}};{{end}}`))
	page := func(id string) *soffio.Document { return &soffio.Document{ID: id, Meta: map[string]string{}} }
	docs := map[string]*soffio.Document{}
	for _, id := range []string{"en/x", "it/x", "it/x/c", "it/x/a", "it/x/b", "blog/x"} {
		docs[id] = page(id)
	}
	s := &site{
		baseURL: "https://example.org",
		langs:   []string{"it", "en"},
		outDir:  t.TempDir(),
		tmpl:    tmpl,
		docs:    docs,
		ids:     slices.Sorted(maps.Keys(docs)),
	}

	tests := []struct{ id, want string }{
		{"it/x", "it=https://example.org/it/x.html;en=https://example.org/en/x.html;|it/x/a;it/x/b;it/x/c;"},
		// blog is no language: no alternates in en/ or it/
		{"blog/x", "|"},
	}
	for _, tt := range tests {
		if err := s.writeDoc(docs[tt.id]); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(s.outDir, tt.id+".html"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tt.want {
			t.Errorf("%s: got %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestCheckBaseURL(t *testing.T) {
	for u, ok := range map[string]bool{
		"https://example.org":     true,
		"https://example.org/sub": true,
		"":                        true,
		"https://example.org/":    false,
		"/":                       false,
	} {
		if err := checkBaseURL(u); (err == nil) != ok {
			t.Errorf("checkBaseURL(%q) = %v", u, err)
		}
	}
}
