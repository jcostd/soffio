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
		sourceID string // Da dove parte il link
		target   string // Dove punta il link
		want     bool
	}{
		{"assoluto esistente", "it/home", "it/about", true},
		{"relativo stessa cartella", "it/home", "about", true},
		{"relativo con sezione", "it/home", "about#team", true},
		{"relativo fallito (cartella diversa)", "en/home", "about", false}, // In en/ non c'è about
		{"sezione interna", "it/home", "it/home#intro", true},
		{"target inesistente", "it/home", "privacy", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Passiamo tt.sourceID come richiesto dalla nuova firma
			got := validTarget(docs, tt.sourceID, tt.target)
			if got != tt.want {
				t.Errorf("validTarget(source=%q, target=%q) = %v; want %v", tt.sourceID, tt.target, got, tt.want)
			}
		})
	}
}

func TestValidateLinks(t *testing.T) {
	c := New()

	// Documento in 'it/'
	c.Docs["it/doc1"] = &ast.Document{
		ID: "it/doc1",
		Sections: []ast.Section{
			{
				ID: "sec1",
				Blocks: []ast.Block{
					ast.TextBlock{
						Elements: []ast.Inline{
							ast.InternalLink{Target: "doc2"},   // Valido (relativo a it/)
							ast.InternalLink{Target: "broken"}, // Errore
							ast.FootnoteRef{Target: "n1"},      // Valido
						},
					},
					ast.NoteBlock{ID: "n1"},
				},
			},
		},
	}

	// L'altro documento in 'it/'
	c.Docs["it/doc2"] = &ast.Document{
		ID:       "it/doc2",
		Sections: []ast.Section{{ID: "intro"}},
	}

	errs := c.ValidateLinks()

	// Ci aspettiamo solo 1 errore (it/doc1 -> broken)
	if len(errs) != 1 {
		t.Fatalf("atteso 1 errore, ottenuti %d: %v", len(errs), errs)
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
