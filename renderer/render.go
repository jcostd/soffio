// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

// Package renderer emits semantic HTML from a Soffio AST.
package renderer

import (
	"fmt"
	"html"
	"io"
	"log"
	"net/url"
	"path"
	"strings"

	"soffio/ast"
)

// renderer tracks state during document emission.
type renderer struct {
	w           io.Writer
	err         error
	docID       string
	assetPrefix string // The global prefix for static assets (e.g., "/static")
	notes       map[string]ast.NoteBlock
	refs        []string
	refsIdx     map[string]int
	refCount    map[string]int
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

// Render emits doc to w. assetPrefix is prepended to relative asset paths.
func Render(w io.Writer, doc *ast.Document, assetPrefix string) error {
	r := renderer{
		w:           w,
		docID:       doc.ID,
		assetPrefix: assetPrefix,
		notes:       make(map[string]ast.NoteBlock),
		refs:        make([]string, 0, 8),
		refsIdx:     make(map[string]int),
		refCount:    make(map[string]int),
	}

	for _, s := range doc.Sections {
		r.renderSection(s)
	}

	if len(r.refs) > 0 {
		notesTitle := "Notes"
		if customTitle, ok := doc.Meta["notes_title"]; ok && customTitle != "" {
			notesTitle = customTitle
		}

		r.writef("\n<section role=\"doc-endnotes\" aria-labelledby=\"footnotes-%[1]s\">\n\t<h2 id=\"footnotes-%[1]s\">%s</h2>\n\t<ol>\n", doc.ID, html.EscapeString(notesTitle))
		for _, ref := range r.refs {
			if note, ok := r.notes[ref]; ok {
				r.writef("\t\t<li id=\"fn-%s\" role=\"doc-endnote\">", ref)
				r.renderInlines(note.Elements)
				r.writef(" <a href=\"#fnref-%s-1\" aria-label=\"back to reference\">↩</a></li>\n", ref)
			}
		}
		r.write("\t</ol>\n</section>\n")
	}

	for noteID := range r.notes {
		if _, used := r.refsIdx[noteID]; !used {
			log.Printf("soffio: warning: unused footnote ':: note: %s' in document '%s'", noteID, doc.ID)
		}
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
		src := v.Path
		isAbs := strings.HasPrefix(src, "/")
		isExt := strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")

		// Apply prefix only if it's a relative path and a prefix is provided
		if !isAbs && !isExt && r.assetPrefix != "" {
			src = path.Join(r.assetPrefix, src)
		}

		altText := extractPlainText(v.Caption)
		r.write("<figure>\n")
		r.writef("\t<img src=\"%s\" alt=\"%s\" loading=\"lazy\">\n",
			html.EscapeString(src),
			html.EscapeString(altText),
		)
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

	case ast.Link:
		href := resolveURL(r.docID, v.Target)
		u, _ := url.Parse(href)
		if u != nil && (u.Scheme == "http" || u.Scheme == "https") {
			r.writef("<a href=\"%s\" target=\"_blank\" rel=\"noopener noreferrer\">", html.EscapeString(href))
		} else {
			r.writef("<a href=\"%s\">", html.EscapeString(href))
		}
		r.renderInlines(v.Label)
		r.write("</a>")

	case ast.FootnoteRef:
		if _, seen := r.refsIdx[v.Target]; !seen {
			r.refs = append(r.refs, v.Target)
			r.refsIdx[v.Target] = len(r.refs)
		}
		r.refCount[v.Target]++
		r.writef("<sup id=\"fnref-%[1]s-%[2]d\"><a href=\"#fn-%[1]s\" role=\"doc-noteref\">%[3]d</a></sup>",
			html.EscapeString(v.Target), r.refCount[v.Target], r.refsIdx[v.Target])
	}
}

func extractPlainText(elements []ast.Inline) string {
	var b strings.Builder
	for _, el := range elements {
		switch v := el.(type) {
		case ast.PlainText:
			b.WriteString(v.Content)
		case ast.Bold:
			b.WriteString(extractPlainText(v.Elements))
		case ast.Italic:
			b.WriteString(extractPlainText(v.Elements))
		case ast.Link:
			b.WriteString(extractPlainText(v.Label))
		}
	}
	return b.String()
}
