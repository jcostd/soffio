package soffio

import (
	"reflect"
	"testing"
)

func TestParseInline(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Inline
	}{
		{
			name:  "plain text",
			input: "plain text",
			want:  []Inline{PlainText{Content: "plain text"}},
		},
		{
			name:  "bold",
			input: "some *bold* text",
			want: []Inline{
				PlainText{Content: "some "},
				Bold{Elements: []Inline{PlainText{Content: "bold"}}},
				PlainText{Content: " text"},
			},
		},
		{
			name:  "italic",
			input: "some _italic_ text",
			want: []Inline{
				PlainText{Content: "some "},
				Italic{Elements: []Inline{PlainText{Content: "italic"}}},
				PlainText{Content: " text"},
			},
		},
		{
			name:  "internal link",
			input: "(link -> doc-id)",
			want: []Inline{
				Link{
					Target: "doc-id",
					Label:  []Inline{PlainText{Content: "link"}},
				},
			},
		},
		{
			name:  "external link",
			input: "(Website -> https://example.com)",
			want: []Inline{
				Link{
					Target: "https://example.com",
					Label:  []Inline{PlainText{Content: "Website"}},
				},
			},
		},
		{
			name:  "footnote",
			input: "text (*note1)",
			want: []Inline{
				PlainText{Content: "text "},
				FootnoteRef{Target: "note1"},
			},
		},
		{
			name:  "escaping",
			input: `this \*is not\* bold and a backslash \\ end`,
			want: []Inline{
				PlainText{Content: `this *is not* bold and a backslash \ end`},
			},
		},
		{
			name:  "nested formatting",
			input: "*_italic in bold_*",
			want: []Inline{
				Bold{Elements: []Inline{
					Italic{Elements: []Inline{PlainText{Content: "italic in bold"}}},
				}},
			},
		},
		{
			name:  "escaping inside link target",
			input: `(Wiki -> https://en.wikipedia.org/wiki/Test_\(disambiguation\))`,
			want: []Inline{
				Link{
					Target: "https://en.wikipedia.org/wiki/Test_(disambiguation)",
					Label:  []Inline{PlainText{Content: "Wiki"}},
				},
			},
		},
		{
			name:  "balanced parentheses in link label",
			input: `(Download the (PDF) attached -> doc-id)`,
			want: []Inline{
				Link{
					Target: "doc-id",
					Label:  []Inline{PlainText{Content: "Download the (PDF) attached"}},
				},
			},
		},
		{
			name:  "balanced parentheses in link target without escaping",
			input: "(Wiki -> https://en.wikipedia.org/wiki/Test_(disambiguation))",
			want: []Inline{
				Link{
					Target: "https://en.wikipedia.org/wiki/Test_(disambiguation)",
					Label:  []Inline{PlainText{Content: "Wiki"}},
				},
			},
		},
		{
			name:  "mailto link should be external",
			input: "(Contact -> mailto:hello@soffio.org)",
			want: []Inline{
				Link{
					Target: "mailto:hello@soffio.org",
					Label:  []Inline{PlainText{Content: "Contact"}},
				},
			},
		},
		{
			name:  "closing marker preceded by tab is ignored",
			input: "text *not bold\t*",
			want: []Inline{
				PlainText{Content: "text *not bold\t*"},
			},
		},
		{
			name:  "marker inside a word is a character",
			input: "snake_case_name and 2*3*4",
			want:  []Inline{PlainText{Content: "snake_case_name and 2*3*4"}},
		},
		{
			name:  "closing marker must end a word",
			input: "_a_b and c_",
			want: []Inline{
				Italic{Elements: []Inline{PlainText{Content: "a_b and c"}}},
			},
		},
		{
			name:  "bold in parentheses is no note",
			input: "(*bold*), _it_.",
			want: []Inline{
				PlainText{Content: "("},
				Bold{Elements: []Inline{PlainText{Content: "bold"}}},
				PlainText{Content: "), "},
				Italic{Elements: []Inline{PlainText{Content: "it"}}},
				PlainText{Content: "."},
			},
		},
		{
			name:  "bold label is no note",
			input: "(*Bold* -> doc-id)",
			want: []Inline{
				Link{
					Target: "doc-id",
					Label:  []Inline{Bold{Elements: []Inline{PlainText{Content: "Bold"}}}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, why := parseInline(tt.input)
			if why != "" {
				t.Errorf("refused: %s", why)
			}
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
		_, _ = parseInline(input)
	}
}
