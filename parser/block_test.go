package parser

import (
	"reflect"
	"strings"
	"testing"

	"soffio/ast"
)

func TestParse(t *testing.T) {
	input := `ID: test-doc
Title: Il Titolo

== intro | Introduzione

Questo è il primo paragrafo.
Continua qui grazie alla continuazione naturale.

- Elemento lista 1
continua lista
- Elemento lista 2

:: img: foto.jpg | Didascalia della foto`

	want := ast.Document{
		ID:    "test-doc",
		Title: "Il Titolo",
		Meta:  map[string]string{},
		Sections: []ast.Section{
			{
				Level: 2,
				ID:    "intro",
				Title: "Introduzione",
				Blocks: []ast.Block{
					ast.TextBlock{
						Elements: []ast.Inline{
							ast.PlainText{Content: "Questo è il primo paragrafo.\nContinua qui grazie alla continuazione naturale."},
						},
					},
					ast.ListBlock{
						Items: [][]ast.Inline{
							{ast.PlainText{Content: "Elemento lista 1\ncontinua lista"}},
							{ast.PlainText{Content: "Elemento lista 2"}},
						},
					},
					ast.ImageBlock{
						Path:    "foto.jpg",
						Caption: []ast.Inline{ast.PlainText{Content: "Didascalia della foto"}},
					},
				},
			},
		},
	}

	r := strings.NewReader(input)
	got, err := Parse(r)
	if err != nil {
		t.Fatalf("Parse fallito: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Mismatch dell'AST.\ngot:  %#v\nwant: %#v", got, want)
	}
}

func BenchmarkParse(b *testing.B) {
	input := `ID: bench
Title: Bench

== sec | Section

Testo con *grassetto*.

- Item 1
- Item 2
`
	b.ReportAllocs()

	for b.Loop() {
		r := strings.NewReader(input)
		_, _ = Parse(r)
	}
}
