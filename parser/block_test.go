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
Layout: custom

== intro | Introduction

This is the first paragraph.
It continues here naturally.

- List item 1
list continuation
- List item 2

:: img: photo.jpg | Photo caption

== extra | Extra Notes

:: note: n1 | This is a footnote`

	want := ast.Document{
		ID:    "test-doc",
		Title: "The Title",
		Meta: map[string]string{
			"layout": "custom",
		},
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
			{
				Level: 2,
				ID:    "extra",
				Title: "Extra Notes",
				Blocks: []ast.Block{
					ast.NoteBlock{
						ID:       "n1",
						Elements: []ast.Inline{ast.PlainText{Content: "This is a footnote"}},
					},
				},
			},
		},
	}

	r := strings.NewReader(input)
	got, err := Parse(r)
	if err != nil {
		t.Fatalf("I/O error during parse: %v", err)
	}

	stripLines(&got)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("AST mismatch.\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestParse_ImplicitFlush(t *testing.T) {
	// Questo test verifica che la mancanza di una riga vuota
	// prima di un nuovo comando o sezione attivi correttamente il flush.
	input := `
== main | Main
Questo è un paragrafo attaccato
== next | Next
E questo è testo attaccato a un comando
:: img: p.jpg | cap`

	r := strings.NewReader(input)
	got, err := Parse(r)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(got.Sections) != 2 {
		t.Fatalf("Expected 2 sections, got %d", len(got.Sections))
	}

	stripLines(&got)

	wantBlocksMain := []ast.Block{
		ast.TextBlock{Elements: []ast.Inline{ast.PlainText{Content: "Questo è un paragrafo attaccato"}}},
	}
	if !reflect.DeepEqual(got.Sections[0].Blocks, wantBlocksMain) {
		t.Errorf("Implicit flush section mismatch in Main")
	}

	wantBlocksNext := []ast.Block{
		ast.TextBlock{Elements: []ast.Inline{ast.PlainText{Content: "E questo è testo attaccato a un comando"}}},
		ast.ImageBlock{Path: "p.jpg", Caption: []ast.Inline{ast.PlainText{Content: "cap"}}},
	}
	if !reflect.DeepEqual(got.Sections[1].Blocks, wantBlocksNext) {
		t.Errorf("Implicit flush command mismatch in Next")
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "Testo fuori dalla sezione",
			input: `Titolo: Err

Questo testo non ha una sezione dichiarata!`,
			expectedError: "found block content outside any section",
		},
		{
			name: "Sezione malformata (manca pipe)",
			input: `
== id TitoloSbagliato`,
			expectedError: "malformed section (expected '== id | Title')",
		},
		{
			name: "Sezione con ID vuoto",
			input: `
== | Solo Titolo`,
			expectedError: "both ID and Title must be non-empty",
		},
		{
			name: "Comando sconosciuto",
			input: `
== sec | Sec
:: galleria: a | b`,
			expectedError: "unknown command \"galleria\"",
		},
		{
			name: "Comando senza pipe",
			input: `
== sec | Sec
:: img: path caption`,
			expectedError: "malformed command (expected ':: cmd: meta | content')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			_, err := Parse(r) // I/O errors ignoring here

			if err == nil {
				t.Fatalf("Expected an error containing %q, but got no errors", tt.expectedError)
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("Expected error containing %q, got %v", tt.expectedError, err)
			}
		})
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
