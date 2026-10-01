package soffio

import (
	"reflect"
	"strings"
	"testing"
)

// stripLines resets the Line field in all blocks to 0 for robust AST comparison.
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

	want := Document{
		ID:    "test-doc",
		Title: "The Title",
		Meta: map[string]string{
			"layout": "custom",
		},
		Sections: []Section{
			{
				Level: 2,
				ID:    "intro",
				Title: "Introduction",
				Blocks: []Block{
					TextBlock{
						Elements: []Inline{
							PlainText{Content: "This is the first paragraph.\nIt continues here naturally."},
						},
					},
					ListBlock{
						Items: [][]Inline{
							{PlainText{Content: "List item 1\nlist continuation"}},
							{PlainText{Content: "List item 2"}},
						},
					},
					ImageBlock{
						Path:    "photo.jpg",
						Caption: []Inline{PlainText{Content: "Photo caption"}},
					},
				},
			},
			{
				Level: 2,
				ID:    "extra",
				Title: "Extra Notes",
				Blocks: []Block{
					NoteBlock{
						ID:       "n1",
						Elements: []Inline{PlainText{Content: "This is a footnote"}},
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

	wantBlocksMain := []Block{
		TextBlock{Elements: []Inline{PlainText{Content: "Questo è un paragrafo attaccato"}}},
	}
	if !reflect.DeepEqual(got.Sections[0].Blocks, wantBlocksMain) {
		t.Errorf("Implicit flush section mismatch in Main")
	}

	wantBlocksNext := []Block{
		TextBlock{Elements: []Inline{PlainText{Content: "E questo è testo attaccato a un comando"}}},
		ImageBlock{Path: "p.jpg", Caption: []Inline{PlainText{Content: "cap"}}},
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
			name:          "ID con spazio",
			input:         "id: scultura bronzo\n",
			expectedError: `invalid id "scultura bronzo": it contains a space`,
		},
		{
			name:          "ID con accento",
			input:         "id: città\n",
			expectedError: `'à' is not a plain ASCII character`,
		},
		{
			name:          "ID con carattere vietato",
			input:         "id: opere/toro\n",
			expectedError: `'/' is not allowed`,
		},
		{
			name:          "ID che inizia con un punto",
			input:         "id: .nascosto\n",
			expectedError: "it can't start with '.'",
		},
		{
			name:          "Data non ISO",
			input:         "date: 12-03-1960\n",
			expectedError: `invalid date "12-03-1960": expected a real date as YYYY-MM-DD`,
		},
		{
			name:          "Data inesistente",
			input:         "updated: 2026-02-30\n",
			expectedError: `invalid updated "2026-02-30"`,
		},
		{
			name:          "Data evento solo anno",
			input:         "event_date: 1960\n",
			expectedError: `invalid event_date "1960"`,
		},
		{
			name: "Sezione con ID non valido",
			input: `
== la tecnica | La Tecnica`,
			expectedError: `invalid section id "la tecnica": it contains a space`,
		},
		{
			name: "Nota con ID non valido",
			input: `
== sec | Sec
:: note: nota#1 | testo`,
			expectedError: `invalid note id "nota#1": '#' is not allowed`,
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
