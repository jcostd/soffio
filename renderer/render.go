// Package renderer emits semantic HTML from a Soffio AST.
package renderer

import (
	"fmt"
	"html"
	"io"
	"strings"

	"soffio/ast"
)

// renderer tracks state during document emission.
type renderer struct {
	w       io.Writer
	err     error
	notes   map[string]ast.NoteBlock
	refs    []string
	refsIdx map[string]int
}

func (r *renderer) write(s string) {
	if r.err != nil {
		return
	}
	_, r.err = io.WriteString(r.w, s)
}

func (r *renderer) writef(format string, args ...any) {
	if r.err != nil {
		return
	}
	_, r.err = fmt.Fprintf(r.w, format, args...)
}

// Render emits doc to w.
func Render(w io.Writer, doc *ast.Document) error {
	r := renderer{
		w:       w,
		notes:   make(map[string]ast.NoteBlock),
		refs:    make([]string, 0, 8),
		refsIdx: make(map[string]int),
	}

	for _, s := range doc.Sections {
		r.renderSection(s)
	}

	if len(r.refs) > 0 {
		r.writef("\n<hr>\n<section role=\"doc-endnotes\" aria-labelledby=\"footnotes-%[1]s\">\n\t<h2 id=\"footnotes-%[1]s\">Note</h2>\n\t<ol>\n", doc.ID)
		for _, ref := range r.refs {
			if note, ok := r.notes[ref]; ok {
				r.writef("\t\t<li id=\"fn-%s\" role=\"doc-endnote\">", ref)
				r.renderInlines(note.Elements)
				r.writef(" <a href=\"#fnref-%s\" aria-label=\"back to reference\">↩</a></li>\n", ref)
			}
		}
		r.write("\t</ol>\n</section>\n")
	}

	return r.err
}

func (r *renderer) renderSection(sec ast.Section) {
	r.writef("<section id=\"%s\">\n", sec.ID)
	r.writef("<h%d>%s</h%d>\n", sec.Level, html.EscapeString(sec.Title), sec.Level)
	for _, b := range sec.Blocks {
		r.renderBlock(b)
	}
	r.write("</section>\n")
}

func (r *renderer) renderBlock(b ast.Block) {
	switch v := b.(type) {
	case ast.TextBlock:
		r.write("<p>")
		r.renderInlines(v.Elements)
		r.write("</p>\n")

	case ast.ListBlock:
		r.write("<ul>\n")
		for _, el := range v.Items {
			r.write("<li>")
			r.renderInlines(el)
			r.write("</li>\n")
		}
		r.write("</ul>\n")

	case ast.ImageBlock:
		r.write("<figure>\n")
		r.writef("\t<img src=\"%s\" alt=\"\">\n", html.EscapeString(v.Path))
		r.write("\t<figcaption>")
		r.renderInlines(v.Caption)
		r.write("</figcaption>\n</figure>\n")

	case ast.NoteBlock:
		r.notes[v.ID] = v
	}
}

func (r *renderer) renderInlines(elements []ast.Inline) {
	for _, el := range elements {
		r.renderInline(el)
	}
}

func (r *renderer) renderInline(in ast.Inline) {
	switch v := in.(type) {
	case ast.PlainText:
		r.write(html.EscapeString(v.Content))

	case ast.Bold:
		r.write("<strong>")
		r.renderInlines(v.Elements)
		r.write("</strong>")

	case ast.Italic:
		r.write("<em>")
		r.renderInlines(v.Elements)
		r.write("</em>")

	case ast.InternalLink:
		baseID, hash, hasHash := strings.Cut(v.Target, "#")
		href := baseID
		if baseID != "" {
			href += ".html"
		}
		if hasHash {
			href += "#" + hash
		}

		r.writef("<a href=\"%s\">", html.EscapeString(href))
		r.renderInlines(v.Label)
		r.write("</a>")

	case ast.ExternalLink:
		r.writef("<a href=\"%s\" target=\"_blank\" rel=\"noopener noreferrer\">", html.EscapeString(v.Target))
		r.renderInlines(v.Label)
		r.write("</a>")

	case ast.FootnoteRef:
		if _, seen := r.refsIdx[v.Target]; !seen {
			r.refs = append(r.refs, v.Target)
			r.refsIdx[v.Target] = len(r.refs)
		}
		r.writef("<sup id=\"fnref-%[1]s\"><a href=\"#fn-%[1]s\" role=\"doc-noteref\">%d</a></sup>",
			html.EscapeString(v.Target), r.refsIdx[v.Target])
	}
}
