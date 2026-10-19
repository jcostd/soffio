// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"fmt"
	"html"
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
// order they are first referenced, with no heading and no word: it
// reads well in any language, and the roles say what they are.
func Render(doc *Document) string {
	r := renderer{
		id:    doc.ID,
		notes: map[string]NoteBlock{},
		num:   map[string]int{},
		count: map[string]int{},
	}
	// a section holds those of the levels below it, up to the next of
	// its own level or above
	var open []int
	for _, s := range doc.Sections {
		for len(open) > 0 && open[len(open)-1] >= s.Level {
			r.w.WriteString("</section>\n")
			open = open[:len(open)-1]
		}
		r.section(s)
		open = append(open, s.Level)
	}
	r.w.WriteString(strings.Repeat("</section>\n", len(open)))

	if len(r.refs) > 0 {
		r.w.WriteString("<section role=\"doc-endnotes\">\n\t<ol>\n")
		// a note may refer to a later one: r.refs grows as it goes
		for i := 0; i < len(r.refs); i++ {
			ref := html.EscapeString(r.refs[i])
			if note, ok := r.notes[r.refs[i]]; ok {
				fmt.Fprintf(&r.w, "\t\t<li id=\"fn:%s\">", ref)
				r.inlines(note.Elements)
				fmt.Fprintf(&r.w, " <a href=\"#fnref:%s:1\" role=\"doc-backlink\">↩</a></li>\n", ref)
			}
		}
		r.w.WriteString("\t</ol>\n</section>\n")
	}
	return r.w.String()
}

func (r *renderer) section(sec Section) {
	fmt.Fprintf(&r.w, "<section id=\"%s\">\n<h%d>%s</h%[2]d>\n", html.EscapeString(sec.ID), sec.Level, html.EscapeString(sec.Title))
	for _, b := range sec.Blocks {
		r.block(b)
	}
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
		fmt.Fprintf(&r.w, "<figure>\n\t<img src=\"%s\" alt=\"%s\"", html.EscapeString(href(r.id, v.Path)), html.EscapeString(plainText(v.Caption)))
		if v.Width > 0 && v.Height > 0 {
			fmt.Fprintf(&r.w, " width=\"%d\" height=\"%d\"", v.Width, v.Height)
		}
		r.w.WriteString(" loading=\"lazy\">\n\t<figcaption>")
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
		fmt.Fprintf(&r.w, "<a href=\"%s\">", html.EscapeString(href(r.id, v.Target)))
		r.inlines(v.Label)
		r.w.WriteString("</a>")
	case FootnoteRef:
		if r.num[v.Target] == 0 {
			r.refs = append(r.refs, v.Target)
			r.num[v.Target] = len(r.refs)
		}
		r.count[v.Target]++
		fmt.Fprintf(&r.w, "<sup id=\"fnref:%[1]s:%[2]d\"><a href=\"#fn:%[1]s\" role=\"doc-noteref\">%[3]d</a></sup>",
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
