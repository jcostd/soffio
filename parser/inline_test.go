package parser

import (
	"reflect"
	"testing"

	"soffio/ast"
)

func TestParseInline(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []ast.Inline
	}{
		{
			name:  "plain text",
			input: "testo normale",
			want:  []ast.Inline{ast.PlainText{Content: "testo normale"}},
		},
		{
			name:  "bold",
			input: "testo *grassetto*",
			want: []ast.Inline{
				ast.PlainText{Content: "testo "},
				ast.Bold{Elements: []ast.Inline{ast.PlainText{Content: "grassetto"}}},
			},
		},
		{
			name:  "italic",
			input: "testo _corsivo_",
			want: []ast.Inline{
				ast.PlainText{Content: "testo "},
				ast.Italic{Elements: []ast.Inline{ast.PlainText{Content: "corsivo"}}},
			},
		},
		{
			name:  "internal link",
			input: "(Link interno -> doc-id)",
			want: []ast.Inline{
				ast.InternalLink{
					Target: "doc-id",
					Label:  []ast.Inline{ast.PlainText{Content: "Link interno"}},
				},
			},
		},
		{
			name:  "external link",
			input: "(Sito -> https://example.com)",
			want: []ast.Inline{
				ast.ExternalLink{
					Target: "https://example.com",
					Label:  []ast.Inline{ast.PlainText{Content: "Sito"}},
				},
			},
		},
		{
			name:  "footnote",
			input: "testo (*nota1)",
			want: []ast.Inline{
				ast.PlainText{Content: "testo "},
				ast.FootnoteRef{Target: "nota1"},
			},
		},
		{
			name:  "escaping",
			input: `questo \*non è\* grassetto e un backslash \\ fine`,
			want: []ast.Inline{
				ast.PlainText{Content: `questo *non è* grassetto e un backslash \ fine`},
			},
		},
		{
			name:  "nested formatting",
			input: "*_corsivo nel grassetto_*",
			want: []ast.Inline{
				ast.Bold{Elements: []ast.Inline{
					ast.Italic{Elements: []ast.Inline{ast.PlainText{Content: "corsivo nel grassetto"}}},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInline(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("\ngot:  %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func BenchmarkParseInline(b *testing.B) {
	input := "Un testo con *grassetto*, un _corsivo_ e un (Link -> doc-id) per stressare il parser."
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parseInline(input)
	}
}
