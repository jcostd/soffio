package corpus

import (
	"testing"

	"soffio/ast"
)

func TestValidTarget(t *testing.T) {
	docs := map[string]*ast.Document{
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
	}

	tests := []struct {
		name     string
		sourceID string // Origin of the link
		target   string // Link target
		want     bool
	}{
		{"absolute existing", "it/home", "it/about", true},
		{"relative same folder", "it/home", "about", true},
		{"relative with section", "it/home", "about#team", true},
		{"relative failed (different folder)", "en/home", "about", false}, // No 'about' in en/
		{"internal section", "it/home", "it/home#intro", true},
		{"missing target", "it/home", "privacy", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validTarget(docs, tt.sourceID, tt.target)
			if got != tt.want {
				t.Errorf("validTarget(source=%q, target=%q) = %v; want %v", tt.sourceID, tt.target, got, tt.want)
			}
		})
	}
}

func TestValidateLinks(t *testing.T) {
	c := New()

	// Document in 'it/'
	c.Docs["it/doc1"] = &ast.Document{
		ID: "it/doc1",
		Sections: []ast.Section{
			{
				ID: "sec1",
				Blocks: []ast.Block{
					ast.TextBlock{
						Elements: []ast.Inline{
							ast.InternalLink{Target: "doc2"},   // Valid (relative to it/)
							ast.InternalLink{Target: "broken"}, // Error
							ast.FootnoteRef{Target: "n1"},      // Valid
						},
					},
					ast.NoteBlock{ID: "n1"},
				},
			},
		},
	}

	// The other document in 'it/'
	c.Docs["it/doc2"] = &ast.Document{
		ID:       "it/doc2",
		Sections: []ast.Section{{ID: "intro"}},
	}

	errs := c.ValidateLinks()

	// We expect exactly 1 error (it/doc1 -> broken)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
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
							ast.InternalLink{Target: "bench#s1"},
						},
					},
				},
			},
		},
	}
	c.Docs["bench"] = doc

	b.ReportAllocs()

	for b.Loop() {
		_ = c.ValidateLinks()
	}
}
