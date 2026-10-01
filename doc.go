// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

// Package soffio parses, checks and renders Soffio markup.
package soffio

// A Document is one .soffio file.
type Document struct {
	File     string // where it comes from, for messages
	ID       string
	Title    string
	Meta     map[string]string
	Sections []Section
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
		Line    int
		Path    string
		Caption []Inline
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
