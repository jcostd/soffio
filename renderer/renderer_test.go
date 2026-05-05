package renderer

import (
	"strings"
	"testing"

	"soffio/ast"
)

func TestRender_BlocksAndInlines(t *testing.T) {
	doc := &ast.Document{
		ID:    "doc-id",
		Title: "Titolo Documento",
		Sections: []ast.Section{
			{
				ID:    "sec1",
				Level: 2,
				Title: "La Sezione",
				Blocks: []ast.Block{
					ast.TextBlock{
						Elements: []ast.Inline{
							ast.PlainText{Content: "Testo "},
							ast.Bold{Elements: []ast.Inline{ast.PlainText{Content: "grassetto"}}},
							ast.PlainText{Content: " e "},
							ast.Italic{Elements: []ast.Inline{ast.PlainText{Content: "corsivo"}}},
							ast.PlainText{Content: "."},
						},
					},
					ast.ListBlock{
						Items: [][]ast.Inline{
							{ast.PlainText{Content: "Primo elemento"}},
							{ast.PlainText{Content: "Secondo elemento"}},
						},
					},
					ast.ImageBlock{
						Path:    "img/test.jpg",
						Caption: []ast.Inline{ast.PlainText{Content: "Didascalia"}},
					},
				},
			},
		},
	}

	var buf strings.Builder
	if err := Render(&buf, doc); err != nil {
		t.Fatalf("Render fallito: %v", err)
	}

	got := buf.String()

	expectedParts := []string{
		`<section id="sec1">`,
		`<h2>La Sezione</h2>`,
		`<p>Testo <strong>grassetto</strong> e <em>corsivo</em>.</p>`,
		`<ul>`,
		`<li>Primo elemento</li>`,
		`<li>Secondo elemento</li>`,
		`</ul>`,
		`<figure>`,
		`<img src="img/test.jpg" alt="">`,
		`<figcaption>Didascalia</figcaption>`,
		`</figure>`,
		`</section>`,
	}

	for _, part := range expectedParts {
		if !strings.Contains(got, part) {
			t.Errorf("Manca l'HTML atteso:\nAtteso: %s\nOttenuto:\n%s", part, got)
		}
	}
}

func TestRender_Links(t *testing.T) {
	doc := &ast.Document{
		ID: "links",
		Sections: []ast.Section{
			{
				Level: 2,
				Title: "Links",
				Blocks: []ast.Block{
					ast.TextBlock{
						Elements: []ast.Inline{
							// Link esterno
							ast.ExternalLink{
								Target: "https://plan9.io",
								Label:  []ast.Inline{ast.PlainText{Content: "Plan 9"}},
							},
							// Link interno: file e sezione
							ast.InternalLink{
								Target: "it/ernst#storia",
								Label:  []ast.Inline{ast.PlainText{Content: "Ernst"}},
							},
							// Link interno: solo file
							ast.InternalLink{
								Target: "it/cocteau",
								Label:  []ast.Inline{ast.PlainText{Content: "Cocteau"}},
							},
							// Link interno: solo sezione
							ast.InternalLink{
								Target: "#intro",
								Label:  []ast.Inline{ast.PlainText{Content: "Intro"}},
							},
						},
					},
				},
			},
		},
	}

	var buf strings.Builder
	_ = Render(&buf, doc)
	got := buf.String()

	expectedLinks := []string{
		`<a href="https://plan9.io" target="_blank" rel="noopener noreferrer">Plan 9</a>`,
		`<a href="it/ernst.html#storia">Ernst</a>`,
		`<a href="it/cocteau.html">Cocteau</a>`,
		`<a href="#intro">Intro</a>`,
	}

	for _, link := range expectedLinks {
		if !strings.Contains(got, link) {
			t.Errorf("Link errato o mancante:\nAtteso: %s\nOttenuto:\n%s", link, got)
		}
	}
}

func TestRender_Footnotes(t *testing.T) {
	doc := &ast.Document{
		ID: "doc-note",
		Sections: []ast.Section{
			{
				Level: 2,
				Title: "Testo",
				Blocks: []ast.Block{
					ast.TextBlock{
						Elements: []ast.Inline{
							ast.PlainText{Content: "Un'affermazione"},
							ast.FootnoteRef{Target: "n1"},
							ast.PlainText{Content: " e un'altra"},
							ast.FootnoteRef{Target: "n2"},
							ast.PlainText{Content: " e richiamo n1"},
							ast.FootnoteRef{Target: "n1"},
						},
					},
					ast.NoteBlock{
						ID:       "n1",
						Elements: []ast.Inline{ast.PlainText{Content: "Nota uno."}},
					},
					ast.NoteBlock{
						ID:       "n2",
						Elements: []ast.Inline{ast.PlainText{Content: "Nota due."}},
					},
				},
			},
		},
	}

	var buf strings.Builder
	_ = Render(&buf, doc)
	got := buf.String()

	expectedParts := []string{
		// I riferimenti nel testo (n1 deve avere indice 1, n2 indice 2, e il richiamo n1 di nuovo indice 1)
		`<sup id="fnref-n1"><a href="#fn-n1" role="doc-noteref">1</a></sup>`,
		`<sup id="fnref-n2"><a href="#fn-n2" role="doc-noteref">2</a></sup>`,
		// La sezione note a piè di pagina
		`<section role="doc-endnotes" aria-labelledby="footnotes-doc-note">`,
		`<h2 id="footnotes-doc-note">Notes</h2>`,
		`<li id="fn-n1" role="doc-endnote">Nota uno. <a href="#fnref-n1" aria-label="back to reference">↩</a></li>`,
		`<li id="fn-n2" role="doc-endnote">Nota due. <a href="#fnref-n2" aria-label="back to reference">↩</a></li>`,
	}

	for _, part := range expectedParts {
		if !strings.Contains(got, part) {
			t.Errorf("Gestione note errata.\nManca: %s\nOttenuto:\n%s", part, got)
		}
	}
}

func BenchmarkRender(b *testing.B) {
	doc := &ast.Document{
		ID:    "bench",
		Title: "Bench",
		Sections: []ast.Section{
			{
				Level: 2,
				Title: "Section",
				Blocks: []ast.Block{
					ast.TextBlock{
						Elements: []ast.Inline{
							ast.PlainText{Content: "Testo "},
							ast.Bold{Elements: []ast.Inline{ast.PlainText{Content: "grassetto"}}},
							ast.InternalLink{Target: "altro-doc", Label: []ast.Inline{ast.PlainText{Content: "link"}}},
						},
					},
				},
			},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf strings.Builder
		_ = Render(&buf, doc)
	}
}
