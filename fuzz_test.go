package soffio

import (
	"regexp"
	"strings"
	"testing"
)

// tags are the tags Render writes; anything else in its output is text
// that was not escaped.
var tags = regexp.MustCompile(`</?(section|h[2-6]|p|ul|ol|li|figure|figcaption|strong|em|a|sup)( [^<>]*)?>|<img [^<>]*>`)

// FuzzParse checks that no text makes Parse or Render panic, nests a
// link in a link, or gets markup through unescaped.
func FuzzParse(f *testing.F) {
	for _, s := range []string{
		"title: T\n\n== s | S\n*bold* _it_ (a -> b) (*n)\n\n:: note: n | note\n",
		"\n== s | S\n- a\n- b\n:: img: /static/a.png | <b>cap</b>\n",
		"\n== s | S\n(a (b -> c) -> d) \\* <script>x</script> & \"q\"\n",
		"\n=== t | T\n(*bold*) snake_case 2*3*4 (x -> #t) (y -> https://x.org/a_b)\n",
		"notes_title: <N>\n\n== s | S\n(*a)\n\n:: note: a | (*b)\n\n:: note: b | (*a)\n",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		doc, err := Parse("f", strings.NewReader(src))
		if err != nil {
			return
		}
		_ = CheckDoc(doc)
		out := Render(doc)

		depth := 0
		for i := range len(out) {
			switch {
			case strings.HasPrefix(out[i:], "<a "):
				if depth++; depth > 1 {
					t.Fatalf("a link inside a link:\n%s", out)
				}
			case strings.HasPrefix(out[i:], "</a>"):
				depth--
			}
		}
		if depth != 0 {
			t.Fatalf("unbalanced <a>:\n%s", out)
		}
		if text := tags.ReplaceAllString(out, ""); strings.ContainsAny(text, "<>") {
			t.Fatalf("markup not escaped:\n%s", out)
		}
	})
}
