// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"cmp"
	"fmt"
	"html"
	"log"
	"maps"
	"net/url"
	"slices"
	"strings"
)

type renderer struct {
	w     strings.Builder
	id    string
	notes map[string]NoteBlock
	refs  []string       // note IDs, in order of first reference
	num   map[string]int // note ID to its number
	count map[string]int // note ID to its references so far
}

// Render returns doc as HTML: its sections, then the notes in the
// order they are first referenced.
func Render(doc *Document) string {
	r := renderer{
		id:    doc.ID,
		notes: map[string]NoteBlock{},
		num:   map[string]int{},
		count: map[string]int{},
	}
	for _, s := range doc.Sections {
		r.section(s)
	}

	if len(r.refs) > 0 {
		title := cmp.Or(doc.Meta["notes_title"], "Notes")
		fmt.Fprintf(&r.w, "\n<section role=\"doc-endnotes\" aria-labelledby=\"footnotes-%[1]s\">\n\t<h2 id=\"footnotes-%[1]s\">%s</h2>\n\t<ol>\n", doc.ID, html.EscapeString(title))
		for _, ref := range r.refs {
			if note, ok := r.notes[ref]; ok {
				fmt.Fprintf(&r.w, "\t\t<li id=\"fn-%s\" role=\"doc-endnote\">", ref)
				r.inlines(note.Elements)
				fmt.Fprintf(&r.w, " <a href=\"#fnref-%s-1\" aria-label=\"back to reference\">↩</a></li>\n", ref)
			}
		}
		r.w.WriteString("\t</ol>\n</section>\n")
	}

	for _, id := range slices.Sorted(maps.Keys(r.notes)) {
		if r.num[id] == 0 {
			log.Printf("soffio: warning: unused footnote ':: note: %s' in document '%s'", id, doc.ID)
		}
	}
	return r.w.String()
}

func (r *renderer) section(sec Section) {
	fmt.Fprintf(&r.w, "<section id=\"%s\">\n<h%d>%s</h%[2]d>\n", sec.ID, sec.Level, html.EscapeString(sec.Title))
	for _, b := range sec.Blocks {
		r.block(b)
	}
	r.w.WriteString("</section>\n")
}

func (r *renderer) block(b Block) {
	switch v := b.(type) {
	case TextBlock:
		r.w.WriteString("<p>")
		r.inlines(v.Elements)
		r.w.WriteString("</p>\n")
	case ListBlock:
		r.w.WriteString("<ul>\n")
		for _, item := range v.Items {
			r.w.WriteString("<li>")
			r.inlines(item)
			r.w.WriteString("</li>\n")
		}
		r.w.WriteString("</ul>\n")
	case ImageBlock:
		fmt.Fprintf(&r.w, "<figure>\n\t<img src=\"%s\" alt=\"%s\" loading=\"lazy\">\n\t<figcaption>",
			html.EscapeString(v.Path), html.EscapeString(plainText(v.Caption)))
		r.inlines(v.Caption)
		r.w.WriteString("</figcaption>\n</figure>\n")
	case NoteBlock:
		r.notes[v.ID] = v
	}
}

func (r *renderer) inlines(in []Inline) {
	for _, el := range in {
		r.inline(el)
	}
}

func (r *renderer) inline(in Inline) {
	switch v := in.(type) {
	case PlainText:
		r.w.WriteString(html.EscapeString(v.Content))
	case Bold:
		r.w.WriteString("<strong>")
		r.inlines(v.Elements)
		r.w.WriteString("</strong>")
	case Italic:
		r.w.WriteString("<em>")
		r.inlines(v.Elements)
		r.w.WriteString("</em>")
	case Link:
		h := href(r.id, v.Target)
		if u, err := url.Parse(h); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			fmt.Fprintf(&r.w, "<a href=\"%s\" target=\"_blank\" rel=\"noopener noreferrer\">", html.EscapeString(h))
		} else {
			fmt.Fprintf(&r.w, "<a href=\"%s\">", html.EscapeString(h))
		}
		r.inlines(v.Label)
		r.w.WriteString("</a>")
	case FootnoteRef:
		if r.num[v.Target] == 0 {
			r.refs = append(r.refs, v.Target)
			r.num[v.Target] = len(r.refs)
		}
		r.count[v.Target]++
		fmt.Fprintf(&r.w, "<sup id=\"fnref-%[1]s-%[2]d\"><a href=\"#fn-%[1]s\" role=\"doc-noteref\">%[3]d</a></sup>",
			html.EscapeString(v.Target), r.count[v.Target], r.num[v.Target])
	}
}

// plainText is in without its markup, for an image's alt.
func plainText(in []Inline) string {
	var b strings.Builder
	for _, el := range in {
		switch v := el.(type) {
		case PlainText:
			b.WriteString(v.Content)
		case Bold:
			b.WriteString(plainText(v.Elements))
		case Italic:
			b.WriteString(plainText(v.Elements))
		case Link:
			b.WriteString(plainText(v.Label))
		}
	}
	return b.String()
}
