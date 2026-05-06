package parser

import (
	"reflect"
	"strings"
	"testing"

	"soffio/ast"
)

// stripLines resets the Line field in all blocks to 0 for robust AST comparison.
func stripLines(doc *ast.Document) {
	for i := range doc.Sections {
		for j, block := range doc.Sections[i].Blocks {
			switch b := block.(type) {
			case ast.TextBlock:
				b.Line = 0
				doc.Sections[i].Blocks[j] = b
			case ast.ImageBlock:
				b.Line = 0
				doc.Sections[i].Blocks[j] = b
			case ast.NoteBlock:
				b.Line = 0
				doc.Sections[i].Blocks[j] = b
			case ast.ListBlock:
				b.Line = 0
				doc.Sections[i].Blocks[j] = b
			}
		}
	}
}

func TestParse(t *testing.T) {
	input := `ID: test-doc
Title: The Title

== intro | Introduction

This is the first paragraph.
It continues here naturally.

- List item 1
list continuation
- List item 2

:: img: photo.jpg | Photo caption`

	want := ast.Document{
		ID:    "test-doc",
		Title: "The Title",
		Meta:  map[string]string{},
		Sections: []ast.Section{
			{
				Level: 2,
				ID:    "intro",
				Title: "Introduction",
				Blocks: []ast.Block{
					ast.TextBlock{
						Elements: []ast.Inline{
							ast.PlainText{Content: "This is the first paragraph.\nIt continues here naturally."},
						},
					},
					ast.ListBlock{
						Items: [][]ast.Inline{
							{ast.PlainText{Content: "List item 1\nlist continuation"}},
							{ast.PlainText{Content: "List item 2"}},
						},
					},
					ast.ImageBlock{
						Path:    "photo.jpg",
						Caption: []ast.Inline{ast.PlainText{Content: "Photo caption"}},
					},
				},
			},
		},
	}

	r := strings.NewReader(input)
	got, err := Parse(r)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	stripLines(&got)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("AST mismatch.\ngot:  %#v\nwant: %#v", got, want)
	}
}

func BenchmarkParse(b *testing.B) {
	input := `ID: bench
Title: Bench

== sec | Section

Text with *bold*.

- Item 1
- Item 2
`
	b.ReportAllocs()

	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = Parse(r)
	}
}
