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
			input: "plain text",
			want:  []ast.Inline{ast.PlainText{Content: "plain text"}},
		},
		{
			name:  "bold",
			input: "some *bold* text",
			want: []ast.Inline{
				ast.PlainText{Content: "some "},
				ast.Bold{Elements: []ast.Inline{ast.PlainText{Content: "bold"}}},
				ast.PlainText{Content: " text"},
			},
		},
		{
			name:  "italic",
			input: "some _italic_ text",
			want: []ast.Inline{
				ast.PlainText{Content: "some "},
				ast.Italic{Elements: []ast.Inline{ast.PlainText{Content: "italic"}}},
				ast.PlainText{Content: " text"},
			},
		},
		{
			name:  "internal link",
			input: "(link -> doc-id)",
			want: []ast.Inline{
				ast.Link{
					Target: "doc-id",
					Label:  []ast.Inline{ast.PlainText{Content: "link"}},
				},
			},
		},
		{
			name:  "external link",
			input: "(Website -> https://example.com)",
			want: []ast.Inline{
				ast.Link{
					Target: "https://example.com",
					Label:  []ast.Inline{ast.PlainText{Content: "Website"}},
				},
			},
		},
		{
			name:  "footnote",
			input: "text (*note1)",
			want: []ast.Inline{
				ast.PlainText{Content: "text "},
				ast.FootnoteRef{Target: "note1"},
			},
		},
		{
			name:  "escaping",
			input: `this \*is not\* bold and a backslash \\ end`,
			want: []ast.Inline{
				ast.PlainText{Content: `this *is not* bold and a backslash \ end`},
			},
		},
		{
			name:  "nested formatting",
			input: "*_italic in bold_*",
			want: []ast.Inline{
				ast.Bold{Elements: []ast.Inline{
					ast.Italic{Elements: []ast.Inline{ast.PlainText{Content: "italic in bold"}}},
				}},
			},
		},
		{
			name:  "escaping inside link target",
			input: `(Wiki -> https://en.wikipedia.org/wiki/Test_\(disambiguation\))`,
			want: []ast.Inline{
				ast.Link{
					Target: "https://en.wikipedia.org/wiki/Test_(disambiguation)",
					Label:  []ast.Inline{ast.PlainText{Content: "Wiki"}},
				},
			},
		},
		{
			name:  "balanced parentheses in link label",
			input: `(Download the (PDF) attached -> doc-id)`,
			want: []ast.Inline{
				ast.Link{
					Target: "doc-id",
					Label:  []ast.Inline{ast.PlainText{Content: "Download the (PDF) attached"}},
				},
			},
		},
		{
			name:  "balanced parentheses in link target without escaping",
			input: "(Wiki -> https://en.wikipedia.org/wiki/Test_(disambiguation))",
			want: []ast.Inline{
				ast.Link{
					Target: "https://en.wikipedia.org/wiki/Test_(disambiguation)",
					Label:  []ast.Inline{ast.PlainText{Content: "Wiki"}},
				},
			},
		},
		{
			name:  "mailto link should be external",
			input: "(Contact -> mailto:hello@soffio.org)",
			want: []ast.Inline{
				ast.Link{
					Target: "mailto:hello@soffio.org",
					Label:  []ast.Inline{ast.PlainText{Content: "Contact"}},
				},
			},
		},
		{
			name:  "closing marker preceded by tab is ignored",
			input: "text *not bold\t*",
			want: []ast.Inline{
				ast.PlainText{Content: "text *not bold\t*"},
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
	input := "A text with *bold*, an _italic_ and a (Link -> doc-id) to stress the parser."
	b.ReportAllocs()

	for b.Loop() {
		_ = parseInline(input)
	}
}
