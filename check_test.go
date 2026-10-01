package soffio

import (
	"strings"
	"testing"
)

func TestCheckLink(t *testing.T) {
	allDocs := map[string]*Document{
		"it/home": {
			ID:       "it/home",
			Sections: []Section{{ID: "intro"}},
		},
		"it/about": {
			ID:       "it/about",
			Sections: []Section{{ID: "team"}},
		},
		"en/home": {
			ID:       "en/home",
			Sections: []Section{{ID: "intro"}},
		},
		"private/secret": {
			ID:       "private/secret",
			Sections: []Section{{ID: "data"}},
		},
	}

	// activeDocs simula la "vista" pubblica, omettendo il file privato
	activeDocs := map[string]*Document{
		"it/home":  allDocs["it/home"],
		"it/about": allDocs["it/about"],
		"en/home":  allDocs["en/home"],
	}

	const (
		ok      = ""
		missing = "broken link points to missing target"
		leak    = "PRIVACY LEAK: document references excluded/private target"
	)
	tests := []struct {
		name     string
		sourceID string
		target   string
		want     string
	}{
		{"absolute existing", "it/home", "/it/about", ok},
		{"relative same folder", "it/home", "about", ok},
		{"relative with section", "it/home", "about#team", ok},
		{"relative failed (different folder)", "en/home", "about", missing},
		{"internal section absolute", "it/home", "/it/home#intro", ok},
		{"internal section relative (hash only)", "it/home", "#intro", ok},
		{"missing section", "it/home", "#nope", missing},
		{"empty section", "it/home", "about#", missing},
		{"missing target", "it/home", "privacy", missing},
		{"privacy leak target", "it/home", "/private/secret", leak},
		{"external", "it/home", "https://example.org/x", ok},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkLink(allDocs, activeDocs, tt.sourceID, tt.target)
			if got != tt.want {
				t.Errorf("checkLink(source=%q, target=%q) = %q; want %q", tt.sourceID, tt.target, got, tt.want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	docs := map[string]*Document{}
	docs["it/doc1"] = &Document{
		ID: "it/doc1",
		Sections: []Section{
			{
				ID: "sec1",
				Blocks: []Block{
					TextBlock{
						Elements: []Inline{
							Link{
								Target: "doc2",
								Label:  []Inline{PlainText{Content: "Vai a doc2"}},
							},
							Link{
								Target: "broken",
								Label:  []Inline{PlainText{Content: "Link rotto"}},
							},
							FootnoteRef{Target: "n1"},
						},
					},
					NoteBlock{ID: "n1"},
				},
			},
		},
	}

	docs["it/doc2"] = &Document{
		ID:       "it/doc2",
		Sections: []Section{{ID: "intro"}},
	}

	err := Check(docs, docs, "static")

	if err == nil {
		t.Fatalf("expected an error, got nil")
	}

	if !strings.Contains(err.Error(), "broken") {
		t.Fatalf("expected error about 'broken' link, got: %v", err)
	}
}

func TestPrivacyLeak(t *testing.T) {
	docs := map[string]*Document{}
	docs["public/post"] = &Document{
		ID: "public/post",
		Sections: []Section{
			{
				ID: "sec1",
				Blocks: []Block{
					TextBlock{
						Elements: []Inline{
							Link{
								Target: "/private/secret",
								Label:  []Inline{PlainText{Content: "Nota Segreta"}},
							},
						},
					},
				},
			},
		},
	}

	docs["private/secret"] = &Document{
		ID:       "private/secret",
		Sections: []Section{{ID: "sec1"}},
	}

	// Simuliamo una build pubblica dove private/secret viene escluso
	activeDocs := map[string]*Document{
		"public/post": docs["public/post"],
	}

	err := Check(docs, activeDocs, "static")
	if err == nil || !strings.Contains(err.Error(), "PRIVACY LEAK") {
		t.Fatalf("expected privacy leak error, got: %v", err)
	}
}

func BenchmarkCheck(b *testing.B) {
	doc := &Document{
		ID: "bench",
		Sections: []Section{
			{
				ID: "s1",
				Blocks: []Block{
					TextBlock{
						Elements: []Inline{
							Link{
								Target: "bench#s1",
								Label:  []Inline{PlainText{Content: "Self ref"}},
							},
						},
					},
				},
			},
		},
	}
	docs := map[string]*Document{"bench": doc}

	b.ReportAllocs()

	for b.Loop() {
		_ = Check(docs, docs, "static")
	}
}
