package corpus

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"soffio/ast"
)

func TestGet(t *testing.T) {
	c := New()
	c.Docs["test-doc"] = &ast.Document{ID: "test-doc", Title: "Test"}

	// Caso di successo
	doc, err := c.Get("test-doc")
	if err != nil {
		t.Fatalf("unexpected error getting document: %v", err)
	}
	if doc.Title != "Test" {
		t.Errorf("expected Title 'Test', got '%s'", doc.Title)
	}

	// Caso documento non trovato
	_, err = c.Get("missing-doc")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Funzione helper per creare file finti nella cartella temporanea
	writeFile := func(name, content string) {
		dir := filepath.Dir(name)
		if dir != "." {
			os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		}
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 1. File valido con ID esplicito nel frontmatter
	writeFile("doc1.txt", "ID: explicit-id\nTitle: Doc 1\n\n== s1 | S1\nText")

	// 2. File valido senza ID (dovrebbe usare il nome del file 'doc2')
	writeFile("doc2.txt", "Title: Doc 2\n\n== s1 | S1\nText")

	// 3. File in una sottocartella senza ID (dovrebbe diventare 'sub/doc3')
	writeFile("sub/doc3.txt", "Title: Doc 3\n\n== s1 | S1\nText")

	// 4. File duplicato (usa l'ID esplicito già preso da doc1.txt)
	writeFile("dup.txt", "ID: explicit-id\nTitle: Dup\n\n== s1 | S1\nText")

	// 5. File con sintassi non valida per scatenare un errore del parser
	writeFile("bad.txt", "Title: Bad\n\nTesto senza sezione dichiarata")

	// 6. File dentro una cartella 'static' (dovrebbe essere ignorato da WalkDir)
	writeFile("static/ignored.txt", "Title: Ignored\n\n== s1 | S1\nText")

	c := New()
	fsys := os.DirFS(tmpDir)

	err := c.Load(fsys, "*.txt", "static")

	if err == nil {
		t.Fatal("expected Load to return errors for duplicates and bad syntax, got nil")
	}

	if !strings.Contains(err.Error(), "duplicate document ID") {
		t.Errorf("expected duplicate ID error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "found block content outside any section") {
		t.Errorf("expected parser error, got: %v", err)
	}

	if _, err := c.Get("explicit-id"); err != nil {
		t.Errorf("missing explicitly named doc 'explicit-id'")
	}
	if _, err := c.Get("doc2"); err != nil {
		t.Errorf("missing fallback named doc 'doc2'")
	}
	if _, err := c.Get("sub/doc3"); err != nil {
		t.Errorf("missing subfolder doc 'sub/doc3'")
	}
	if _, err := c.Get("static/ignored"); err == nil {
		t.Errorf("document inside 'static' dir should have been skipped")
	}
}

func TestCheckTarget(t *testing.T) {
	allDocs := map[string]*ast.Document{
		"it/home": {
			ID:       "it/home",
			Sections: []ast.Section{{ID: "intro"}},
		},
		"it/about": {
			ID:       "it/about",
			Sections: []ast.Section{{ID: "team"}},
		},
		"en/home": {
			ID:       "en/home",
			Sections: []ast.Section{{ID: "intro"}},
		},
		"private/secret": {
			ID:       "private/secret",
			Sections: []ast.Section{{ID: "data"}},
		},
	}

	// activeDocs simula la "vista" pubblica, omettendo il file privato
	activeDocs := map[string]*ast.Document{
		"it/home":  allDocs["it/home"],
		"it/about": allDocs["it/about"],
		"en/home":  allDocs["en/home"],
	}

	tests := []struct {
		name     string
		sourceID string
		target   string
		want     targetResult
	}{
		{"absolute existing", "it/home", "/it/about", targetOK},
		{"relative same folder", "it/home", "about", targetOK},
		{"relative with section", "it/home", "about#team", targetOK},
		{"relative failed (different folder)", "en/home", "about", targetNotFound},
		{"internal section absolute", "it/home", "/it/home#intro", targetOK},
		{"internal section relative (hash only)", "it/home", "#intro", targetOK},
		{"missing target", "it/home", "privacy", targetNotFound},
		{"privacy leak target", "it/home", "/private/secret", targetPrivacyLeak},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkTarget(allDocs, activeDocs, tt.sourceID, tt.target)
			if got != tt.want {
				t.Errorf("checkTarget(source=%q, target=%q) = %v; want %v", tt.sourceID, tt.target, got, tt.want)
			}
		})
	}
}

func TestValidateLinks(t *testing.T) {
	c := New()

	c.Docs["it/doc1"] = &ast.Document{
		ID: "it/doc1",
		Sections: []ast.Section{
			{
				ID: "sec1",
				Blocks: []ast.Block{
					ast.TextBlock{
						Elements: []ast.Inline{
							ast.Link{
								Target: "doc2",
								Label:  []ast.Inline{ast.PlainText{Content: "Vai a doc2"}},
							},
							ast.Link{
								Target: "broken",
								Label:  []ast.Inline{ast.PlainText{Content: "Link rotto"}},
							},
							ast.FootnoteRef{Target: "n1"},
						},
					},
					ast.NoteBlock{ID: "n1"},
				},
			},
		},
	}

	c.Docs["it/doc2"] = &ast.Document{
		ID:       "it/doc2",
		Sections: []ast.Section{{ID: "intro"}},
	}

	// Passiamo c.Docs come activeDocs per testare la validazione standard
	err := c.ValidateLinks(c.Docs, "static")

	if err == nil {
		t.Fatalf("expected an error, got nil")
	}

	if !strings.Contains(err.Error(), "broken") {
		t.Fatalf("expected error about 'broken' link, got: %v", err)
	}
}

func TestPrivacyLeak(t *testing.T) {
	c := New()

	c.Docs["public/post"] = &ast.Document{
		ID: "public/post",
		Sections: []ast.Section{
			{
				ID: "sec1",
				Blocks: []ast.Block{
					ast.TextBlock{
						Elements: []ast.Inline{
							ast.Link{
								Target: "/private/secret",
								Label:  []ast.Inline{ast.PlainText{Content: "Nota Segreta"}},
							},
						},
					},
				},
			},
		},
	}

	c.Docs["private/secret"] = &ast.Document{
		ID:       "private/secret",
		Sections: []ast.Section{{ID: "sec1"}},
	}

	// Simuliamo una build pubblica dove private/secret viene escluso
	activeDocs := map[string]*ast.Document{
		"public/post": c.Docs["public/post"],
	}

	err := c.ValidateLinks(activeDocs, "static")
	if err == nil {
		t.Fatalf("expected privacy leak error, got nil")
	}

	var leakErr *PrivacyLeakError
	if !errors.As(err, &leakErr) {
		t.Fatalf("expected error of type *PrivacyLeakError, got: %v", err)
	}
}

func BenchmarkValidateLinks(b *testing.B) {
	c := New()
	doc := &ast.Document{
		ID: "bench",
		Sections: []ast.Section{
			{
				ID: "s1",
				Blocks: []ast.Block{
					ast.TextBlock{
						Elements: []ast.Inline{
							ast.Link{
								Target: "bench#s1",
								Label:  []ast.Inline{ast.PlainText{Content: "Self ref"}},
							},
						},
					},
				},
			},
		},
	}
	c.Docs["bench"] = doc

	b.ReportAllocs()

	for b.Loop() {
		_ = c.ValidateLinks(c.Docs, "static")
	}
}
