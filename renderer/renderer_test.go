package renderer

import (
	"strings"
	"testing"

	"soffio/ast"
)

func TestRender_BlocksAndInlines(t *testing.T) {
	doc := &ast.Document{
		ID:    "doc-id",
		Title: "Document Title",
		Sections: []ast.Section{
			{
				ID:    "sec1",
				Level: 2,
				Title: "The Section",
				Blocks: []ast.Block{
					ast.TextBlock{
						Elements: []ast.Inline{
							ast.PlainText{Content: "Some "},
							ast.Bold{Elements: []ast.Inline{ast.PlainText{Content: "bold"}}},
							ast.PlainText{Content: " and "},
							ast.Italic{Elements: []ast.Inline{ast.PlainText{Content: "italic"}}},
							ast.PlainText{Content: " text."},
						},
					},
					ast.ListBlock{
						Items: [][]ast.Inline{
							{ast.PlainText{Content: "First item"}},
							{ast.PlainText{Content: "Second item"}},
						},
					},
					ast.ImageBlock{
						Path:    "img/test.jpg",
						Caption: []ast.Inline{ast.PlainText{Content: "Caption"}},
					},
				},
			},
		},
	}

	var buf strings.Builder
	if err := Render(&buf, doc); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	got := buf.String()

	expectedParts := []string{
		`<section id="sec1">`,
		`<h2>The Section</h2>`,
		`<p>Some <strong>bold</strong> and <em>italic</em> text.</p>`,
		`<ul>`,
		`<li>First item</li>`,
		`<li>Second item</li>`,
		`</ul>`,
		`<figure>`,
		`<img src="img/test.jpg" alt="Caption" loading="lazy">`,
		`<figcaption>Caption</figcaption>`,
		`</figure>`,
		`</section>`,
	}

	for _, part := range expectedParts {
		if !strings.Contains(got, part) {
			t.Errorf("Missing expected HTML part.\nExpected: %s\nGot:\n%s", part, got)
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
							// External link
							ast.ExternalLink{
								Target: "https://plan9.io",
								Label:  []ast.Inline{ast.PlainText{Content: "Plan 9"}},
							},
							// Internal link: file and section
							ast.InternalLink{
								Target: "it/ernst#history",
								Label:  []ast.Inline{ast.PlainText{Content: "Ernst"}},
							},
							// Internal link: file only
							ast.InternalLink{
								Target: "it/cocteau",
								Label:  []ast.Inline{ast.PlainText{Content: "Cocteau"}},
							},
							// Internal link: section only
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
		`<a href="it/ernst.html#history">Ernst</a>`,
		`<a href="it/cocteau.html">Cocteau</a>`,
		`<a href="#intro">Intro</a>`,
	}

	for _, link := range expectedLinks {
		if !strings.Contains(got, link) {
			t.Errorf("Wrong or missing link.\nExpected: %s\nGot:\n%s", link, got)
		}
	}
}

func TestRender_Footnotes(t *testing.T) {
	doc := &ast.Document{
		ID: "doc-note",
		Sections: []ast.Section{
			{
				Level: 2,
				Title: "Text",
				Blocks: []ast.Block{
					ast.TextBlock{
						Elements: []ast.Inline{
							ast.PlainText{Content: "A statement"},
							ast.FootnoteRef{Target: "n1"},
							ast.PlainText{Content: " and another"},
							ast.FootnoteRef{Target: "n2"},
							ast.PlainText{Content: " and back to n1"},
							ast.FootnoteRef{Target: "n1"},
						},
					},
					ast.NoteBlock{
						ID:       "n1",
						Elements: []ast.Inline{ast.PlainText{Content: "Note one."}},
					},
					ast.NoteBlock{
						ID:       "n2",
						Elements: []ast.Inline{ast.PlainText{Content: "Note two."}},
					},
				},
			},
		},
	}

	var buf strings.Builder
	_ = Render(&buf, doc)
	got := buf.String()

	expectedParts := []string{
		// References in text
		`<sup id="fnref-n1-1"><a href="#fn-n1" role="doc-noteref">1</a></sup>`,
		`<sup id="fnref-n2-1"><a href="#fn-n2" role="doc-noteref">2</a></sup>`,
		`<sup id="fnref-n1-2"><a href="#fn-n1" role="doc-noteref">1</a></sup>`,
		// Endnotes section
		`<section role="doc-endnotes" aria-labelledby="footnotes-doc-note">`,
		`<h2 id="footnotes-doc-note">Notes</h2>`,
		`<li id="fn-n1" role="doc-endnote">Note one. <a href="#fnref-n1-1" aria-label="back to reference">↩</a></li>`,
		`<li id="fn-n2" role="doc-endnote">Note two. <a href="#fnref-n2-1" aria-label="back to reference">↩</a></li>`,
	}

	for _, part := range expectedParts {
		if !strings.Contains(got, part) {
			t.Errorf("Footnote handling error.\nMissing: %s\nGot:\n%s", part, got)
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
							ast.PlainText{Content: "Some "},
							ast.Bold{Elements: []ast.Inline{ast.PlainText{Content: "bold"}}},
							ast.InternalLink{Target: "other-doc", Label: []ast.Inline{ast.PlainText{Content: "link"}}},
						},
					},
				},
			},
		},
	}

	b.ReportAllocs()
	for b.Loop() {
		var buf strings.Builder
		_ = Render(&buf, doc)
	}
}
