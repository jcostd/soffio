package soffio

import (
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
)

// stripLines zeroes the Line of every block, to compare trees alone.
func stripLines(doc *Document) {
	for i := range doc.Sections {
		for j, block := range doc.Sections[i].Blocks {
			switch b := block.(type) {
			case TextBlock:
				b.Line = 0
				doc.Sections[i].Blocks[j] = b
			case ImageBlock:
				b.Line = 0
				doc.Sections[i].Blocks[j] = b
			case NoteBlock:
				b.Line = 0
				doc.Sections[i].Blocks[j] = b
			case ListBlock:
				b.Line = 0
				doc.Sections[i].Blocks[j] = b
			}
		}
	}
}

func TestParse(t *testing.T) {
	input := `Title: The Title
Layout: custom

== intro | Introduction

This is the first paragraph.
It continues here naturally.

- List item 1
list continuation
- List item 2

:: img: /static/photo.jpg | Photo caption

== extra | Extra Notes

:: note: n1 | This is a footnote`

	want := &Document{
		File:  "t.soffio",
		Title: "The Title",
		Meta:  map[string]string{"layout": "custom"},
		lines: map[string]int{"title": 1, "layout": 2},
		Sections: []Section{
			{
				Level: 2,
				ID:    "intro",
				Title: "Introduction",
				Blocks: []Block{
					TextBlock{Elements: []Inline{PlainText{Content: "This is the first paragraph.\nIt continues here naturally."}}},
					ListBlock{Items: [][]Inline{
						{PlainText{Content: "List item 1\nlist continuation"}},
						{PlainText{Content: "List item 2"}},
					}},
					ImageBlock{Path: "/static/photo.jpg", Caption: []Inline{PlainText{Content: "Photo caption"}}},
				},
			},
			{
				Level: 2,
				ID:    "extra",
				Title: "Extra Notes",
				Blocks: []Block{
					NoteBlock{ID: "n1", Elements: []Inline{PlainText{Content: "This is a footnote"}}},
				},
			},
		},
	}

	got, err := Parse("t.soffio", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	stripLines(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got:  %#v\nwant: %#v", got, want)
	}
}

func TestParseBlockEnds(t *testing.T) {
	// a section or command line ends the block before it, blank line or not
	input := `
== main | Main
Questo è un paragrafo attaccato
== next | Next
E questo è testo attaccato a un comando
:: img: /static/p.jpg | cap`

	got, err := Parse("t.soffio", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	want := []Section{
		{Level: 2, ID: "main", Title: "Main", Blocks: []Block{
			TextBlock{Line: 3, Elements: []Inline{PlainText{Content: "Questo è un paragrafo attaccato"}}},
		}},
		{Level: 2, ID: "next", Title: "Next", Blocks: []Block{
			TextBlock{Line: 5, Elements: []Inline{PlainText{Content: "E questo è testo attaccato a un comando"}}},
			ImageBlock{Line: 6, Path: "/static/p.jpg", Caption: []Inline{PlainText{Content: "cap"}}},
		}},
	}
	if !reflect.DeepEqual(got.Sections, want) {
		t.Errorf("got:  %#v\nwant: %#v", got.Sections, want)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"title: x\n\ntext", `t.soffio:3: text before the first section`},
		{"\n== id Title", `t.soffio:2: "== id Title" is not == id | Title`},
		{"\n== | Title", `t.soffio:2: section "== | Title" needs an id and a title`},
		{"\n======= a | A", `t.soffio:2: section level 7: want 2 to 6`},
		{"\n== a | A\n\n== a | Again", `t.soffio:4: duplicate section id "a"`},
		{"\n== a b | A", `t.soffio:2: invalid section id "a b": it contains a space`},
		{"\n== s | S\n:: gallery: a | b", `t.soffio:3: unknown command "gallery": want img or note`},
		{"\n== s | S\n::img: /static/a.png | a", `t.soffio:3: "::img: /static/a.png | a" is not :: cmd: arg | text`},
		{"\n== s | S\ntext\n::", `t.soffio:4: "::" is not :: cmd: arg | text`},
		{"\n== s | S\n:: img: path caption", `t.soffio:3: ":: img: path caption" is not :: cmd: arg | text`},
		{"\n== s | S\n:: img: img/a.png | a", `t.soffio:3: image "img/a.png" is not under /static/`},
		{"\n== s | S\n:: img: /img/a.png | a", `t.soffio:3: image "/img/a.png" is not under /static/`},
		{"\n== s | S\n:: note: n#1 | x", `t.soffio:3: invalid note id "n#1": '#' is not allowed`},
		{"\n== s | S\n:: note: n | x\n\n:: note: n | y", `t.soffio:5: duplicate note id "n"`},
		{"\n== s | S\n- a\n-\n- c", `t.soffio:4: empty list item`},
		{"bad header\n", `t.soffio:1: "bad header" is not key: value`},
		{": x\n", `t.soffio:1: empty header key`},
		{"title: a\ntitle: b\n", `t.soffio:2: duplicate header key "title"`},
		{"id: toro\n", `t.soffio:1: id: a text's ID is its file's name: rename the file instead`},
		{"visibility: pubic\n", `t.soffio:1: invalid visibility "pubic": want public or private`},
		{"date: 12-03-1960\n", `t.soffio:1: invalid date "12-03-1960": want a real date, YYYY-MM-DD`},
		{"updated: 2026-02-30\n", `invalid updated "2026-02-30"`},
		{"event_date: 196\n", `invalid event_date "196"`},
		{"when: 1960-13\n", `invalid when "1960-13"`},
		{"until: 29-03-2027\n", `invalid until "29-03-2027"`},
		{"until: -> x\n", `invalid until "-> x"`},
		{"title: T\n== s | Title: sub\n", `t.soffio:2: invalid header key "== s | title": letters, digits and _ only`},
		{"my-key: x\n", `invalid header key "my-key"`},
		{"image: img/a.webp\n", `t.soffio:1: image "img/a.webp" is not under /static/`},
		{"title: T\ncover_image: /img/a.webp\n", `t.soffio:2: cover_image "/img/a.webp" is not under /static/`},
		{"title: x\xff\n", `t.soffio:1: invalid UTF-8: save the file as UTF-8`},
		{"\n== s | S\n(a (b -> c) -> d)", `t.soffio:3: a link inside a link`},
		{"\n== s | S\n(*a (b -> c)* -> d)", `t.soffio:3: a link inside a link`},
		{"\n== s | S\n(_x (*n)_ -> d)\n\n:: note: n | n", `t.soffio:3: a note inside a link`},
		{"\n== s | S\n\n- (x (*n) -> d)\n\n:: note: n | n", `t.soffio:4: a note inside a link`},
		{"\n== s | S\n:: img: /static/a.png | (a (b -> c) -> d)", `t.soffio:3: a link inside a link`},
		{"\n== s | S\n- a\n- ", `t.soffio:4: empty list item`},
		{"title: T\nartist: ->\n", `t.soffio:2: artist: a link with no target`},
		{"event_date: -> x\n", `invalid event_date "-> x"`},
		{"image: -> x\n", `image "-> x" is not under /static/`},
	}
	for _, tt := range tests {
		_, err := Parse("t.soffio", strings.NewReader(tt.input))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Parse(%q) = %v, want %q", tt.input, err, tt.want)
		}
	}
}

func TestParseBOMAndDashes(t *testing.T) {
	// a byte order mark is ignored; "-5" is text, "- 5" an item
	doc, err := Parse("t.soffio", strings.NewReader("\uFEFFtitle: x\n\n== s | S\n-5 gradi\n\n- a\n-b goes on"))
	if err != nil {
		t.Fatal(err)
	}
	stripLines(doc)
	want := []Block{
		TextBlock{Elements: []Inline{PlainText{Content: "-5 gradi"}}},
		ListBlock{Items: [][]Inline{{PlainText{Content: "a\n-b goes on"}}}},
	}
	if doc.Title != "x" || !reflect.DeepEqual(doc.Sections[0].Blocks, want) {
		t.Errorf("title %q, blocks %#v", doc.Title, doc.Sections[0].Blocks)
	}
}

func TestParseExternalImage(t *testing.T) {
	doc, err := Parse("t.soffio", strings.NewReader("\n== s | S\n:: img: https://example.org/a.png | a"))
	if err != nil {
		t.Fatal(err)
	}
	if b := doc.Sections[0].Blocks[0].(ImageBlock); b.Path != "https://example.org/a.png" {
		t.Errorf("path %q", b.Path)
	}
}

func BenchmarkParse(b *testing.B) {
	input := `Title: Bench

== sec | Section

Text with *bold*.

- Item 1
- Item 2
`
	b.ReportAllocs()
	for b.Loop() {
		_, _ = Parse("bench", strings.NewReader(input))
	}
}

// A refused section line is the error: the text under it is not also
// "before the first section". Text that is before one still is.
func TestParseRefusedSection(t *testing.T) {
	for src, want := range map[string]string{
		"title: T\n\n== storia Storia\nx\n\n- y":       `t.soffio:3: "== storia Storia" is not == id | Title`,
		"title: T\n\n======= s | S\nx":                 `t.soffio:3: section level 7: want 2 to 6`,
		"title: T\n\nx\n\n== s Storia\ny":              "t.soffio:3: text before the first section\nt.soffio:5: \"== s Storia\" is not == id | Title",
		"title: T\n\n== s | S\nx\n\n== s | Again\n(y)": `t.soffio:6: duplicate section id "s"`,
	} {
		_, err := Parse("t.soffio", strings.NewReader(src))
		if err == nil || err.Error() != want {
			t.Errorf("Parse(%q):\n%v\nwant:\n%s", src, err, want)
		}
	}
}

// a header value -> target is a link; layout and dates never are
func TestParseHeaderLinks(t *testing.T) {
	doc, err := Parse("t.soffio", strings.NewReader("title: T\nartist: -> picasso\nplace:->/it/luoghi/murano\nlayout: -> x\nnote: a -> b\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"artist", "place"}; !slices.Equal(doc.links, want) {
		t.Errorf("links %q, want %q", doc.links, want)
	}
	for key, want := range map[string]string{"artist": "picasso", "place": "/it/luoghi/murano", "layout": "-> x", "note": "a -> b"} {
		if doc.Meta[key] != want {
			t.Errorf("Meta[%q] = %q, want %q", key, doc.Meta[key], want)
		}
	}
}

func TestParseDates(t *testing.T) {
	// a date is as precise as what is known of it
	in := "date: 2026-10-17\nwhen: 1954\nuntil: 2027-03\nevent_date: 1912-04-22\n"
	doc, err := Parse("t.soffio", strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"date": "2026-10-17", "when": "1954", "until": "2027-03", "event_date": "1912-04-22"} {
		if doc.Meta[k] != want {
			t.Errorf("%s = %q, want %q", k, doc.Meta[k], want)
		}
	}
}

func TestParseReadError(t *testing.T) {
	if _, err := Parse("t.soffio", iotest.ErrReader(io.ErrUnexpectedEOF)); err != io.ErrUnexpectedEOF {
		t.Errorf("Parse of a reader that fails: %v", err)
	}
}

func TestCheckID(t *testing.T) {
	for id, want := range map[string]string{
		"toro-1954": "",
		"":          "it is empty",
		".git":      "it can't start with '.'",
		"a b":       "it contains a space",
		"città":     `'à' is not a plain ASCII character`,
		"a#b":       `'#' is not allowed`,
	} {
		if got := CheckID(id); got != want {
			t.Errorf("CheckID(%q) = %q, want %q", id, got, want)
		}
	}
}
