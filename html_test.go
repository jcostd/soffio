package soffio

import (
	"strings"
	"testing"
)

func TestRender_BlocksAndInlines(t *testing.T) {
	doc := &Document{
		ID:    "doc-id",
		Title: "Document Title",
		Sections: []Section{
			{
				ID:    "sec1",
				Level: 2,
				Title: "The Section",
				Blocks: []Block{
					TextBlock{
						Elements: []Inline{
							PlainText{Content: "Some "},
							Bold{Elements: []Inline{PlainText{Content: "bold"}}},
							PlainText{Content: " and "},
							Italic{Elements: []Inline{PlainText{Content: "italic"}}},
							PlainText{Content: " text."},
						},
					},
					ListBlock{
						Items: [][]Inline{
							{PlainText{Content: "First item"}},
							{PlainText{Content: "Second item"}},
						},
					},
					ImageBlock{
						Path:    "/static/img/test.jpg",
						Caption: []Inline{PlainText{Content: "Caption"}},
					},
				},
			},
		},
	}

	got := Render(doc)

	expectedParts := []string{
		`<section id="sec1">`,
		`<h2>The Section</h2>`,
		`<p>Some <strong>bold</strong> and <em>italic</em> text.</p>`,
		`<ul>`,
		`<li>First item</li>`,
		`<li>Second item</li>`,
		`</ul>`,
		`<figure>`,
		`<img src="static/img/test.jpg" alt="Caption" loading="lazy">`,
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
	// We set the ID to a subfolder path to properly test relative URL resolution
	doc := &Document{
		ID: "it/about",
		Sections: []Section{
			{
				Level: 2,
				Title: "Links",
				Blocks: []Block{
					TextBlock{
						Elements: []Inline{
							Link{
								Target: "https://plan9.io",
								Label:  []Inline{PlainText{Content: "Plan 9"}},
							},
							// logical link: cross-folder (should resolve to ../en/home.html#intro)
							Link{
								Target: "/en/home#intro",
								Label:  []Inline{PlainText{Content: "Home"}},
							},
							// logical link: same-folder (should resolve to contact.html)
							Link{
								Target: "/it/contact",
								Label:  []Inline{PlainText{Content: "Contact"}},
							},
							// physical asset: cross-folder (should NOT append .html)
							Link{
								Target: "/static/docs/manual.pdf",
								Label:  []Inline{PlainText{Content: "Manual"}},
							},
							// an ID with a dot is still a page
							Link{
								Target: "v1.2",
								Label:  []Inline{PlainText{Content: "V"}},
							},
							// link: section only (same page)
							Link{
								Target: "#history",
								Label:  []Inline{PlainText{Content: "History"}},
							},
						},
					},
				},
			},
		},
	}

	got := Render(doc)

	expectedLinks := []string{
		`<a href="https://plan9.io" target="_blank" rel="noopener noreferrer">Plan 9</a>`,
		`<a href="../en/home.html#intro">Home</a>`,
		`<a href="contact.html">Contact</a>`,
		`<a href="../static/docs/manual.pdf">Manual</a>`, // Notice the missing .html extension!
		`<a href="#history">History</a>`,
		`<a href="v1.2.html">V</a>`,
	}

	for _, link := range expectedLinks {
		if !strings.Contains(got, link) {
			t.Errorf("Wrong or missing link.\nExpected: %s\nGot:\n%s", link, got)
		}
	}
}

func TestRender_Footnotes(t *testing.T) {
	doc := &Document{
		ID: "doc-note",
		Sections: []Section{
			{
				Level: 2,
				Title: "Text",
				Blocks: []Block{
					TextBlock{
						Elements: []Inline{
							PlainText{Content: "A statement"},
							FootnoteRef{Target: "n1"},
							PlainText{Content: " and another"},
							FootnoteRef{Target: "n2"},
							PlainText{Content: " and back to n1"},
							FootnoteRef{Target: "n1"},
						},
					},
					NoteBlock{
						ID:       "n1",
						Elements: []Inline{PlainText{Content: "Note one."}},
					},
					NoteBlock{
						ID:       "n2",
						Elements: []Inline{PlainText{Content: "Note two."}},
					},
				},
			},
		},
	}

	got := Render(doc)

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

func TestRenderRelativeImage(t *testing.T) {
	doc := &Document{ID: "it/opere/toro", Sections: []Section{{Level: 2, ID: "s", Blocks: []Block{
		ImageBlock{Path: "/static/img/toro.webp", Caption: []Inline{PlainText{Content: "Toro"}}},
	}}}}
	// relative, as links are: the site works under any base
	if got, want := Render(doc), `<img src="../../static/img/toro.webp"`; !strings.Contains(got, want) {
		t.Errorf("missing %s in:\n%s", want, got)
	}
}

func TestRenderNoteInNote(t *testing.T) {
	doc := &Document{ID: "d", Sections: []Section{{Level: 2, ID: "s", Blocks: []Block{
		TextBlock{Elements: []Inline{FootnoteRef{Target: "a"}}},
		NoteBlock{ID: "a", Elements: []Inline{PlainText{Content: "see "}, FootnoteRef{Target: "b"}}},
		NoteBlock{ID: "b", Elements: []Inline{PlainText{Content: "B"}}},
	}}}}
	// b is referenced only from note a, yet it is an endnote too
	if got, want := Render(doc), `<li id="fn-b" role="doc-endnote">B`; !strings.Contains(got, want) {
		t.Errorf("missing %s in:\n%s", want, got)
	}
}

func BenchmarkRender(b *testing.B) {
	doc := &Document{
		ID:    "bench",
		Title: "Bench",
		Sections: []Section{
			{
				Level: 2,
				Title: "Section",
				Blocks: []Block{
					TextBlock{
						Elements: []Inline{
							PlainText{Content: "Some "},
							Bold{Elements: []Inline{PlainText{Content: "bold"}}},
							Link{Target: "other-doc", Label: []Inline{PlainText{Content: "link"}}},
						},
					},
				},
			},
		},
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = Render(doc)
	}
}
