// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

// Package soffio parses, checks and renders Soffio markup, and is the
// command soffio: Main makes a directory of texts a static site, a page
// each, or one text on standard input HTML on standard output.
//
// A text goes through Parse, the text alone, then Load, which reads a
// directory of them into Documents by ID and resolves their header
// links; Check says what is wrong with the whole site, and Render makes
// one Document an HTML fragment. Links says which pages a Document
// links to.
//
// Soffio is to a site what Lua's core is to its host: it gives the
// mechanisms, and a site says the rest in four ways and no other:
// functions for its templates and a check of its texts, both given to
// Main; its templates; and programs of its own on this package. Soffio
// itself is Main(nil, nil). The README, a manual page, says what the
// markup and the command are.
package soffio

// A Document is one .soffio file.
type Document struct {
	File     string // where it comes from, for messages
	ID       string
	Meta     map[string]string // the header, every key of it, title too
	Sections []Section
	Lines    map[string]int // header key to its line, for messages
	// HeaderLinks are the header keys that hold a link, in order:
	// Meta holds, for each, the ID of its page
	HeaderLinks []string
}

// A Section is a heading and the blocks under it.
type Section struct {
	Level  int // 2 for ==, up to 6
	ID     string
	Title  string
	Blocks []Block
}

// A Block is a TextBlock, ImageBlock, NoteBlock or ListBlock.
type Block interface{ isBlock() }

// Line is where a block starts in its file.
type (
	TextBlock struct {
		Line     int
		Elements []Inline
	}
	ImageBlock struct {
		Line          int
		Path          string
		Caption       []Inline
		Width, Height int // in pixels, 0 if not known: who has the file says
	}
	NoteBlock struct {
		Line     int
		ID       string
		Elements []Inline
	}
	ListBlock struct {
		Line  int
		Items [][]Inline
	}
)

func (TextBlock) isBlock()  {}
func (ImageBlock) isBlock() {}
func (NoteBlock) isBlock()  {}
func (ListBlock) isBlock()  {}

// An Inline is PlainText, Bold, Italic, Link or FootnoteRef.
type Inline interface{ isInline() }

type (
	PlainText   struct{ Content string }
	Bold        struct{ Elements []Inline }
	Italic      struct{ Elements []Inline }
	FootnoteRef struct{ Target string }
	Link        struct {
		Target string
		Label  []Inline
	}
)

func (PlainText) isInline()   {}
func (Bold) isInline()        {}
func (Italic) isInline()      {}
func (Link) isInline()        {}
func (FootnoteRef) isInline() {}
