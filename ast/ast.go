// Package ast defines the abstract syntax tree for the soffio markup.
package ast

// BlockType identifies the kind of content a block holds.
type BlockType int

const (
	BlockTypeText BlockType = iota
	BlockTypeImage
	BlockTypeNote
	BlockTypeList
)

// Block is the interface implemented by all document blocks.
type Block interface {
	Type() BlockType
	Inlines() []Inline
}

// Inline is the interface for all elements within a line of text.
type Inline interface {
	isInline()
}

type (
	PlainText    struct{ Content string }
	Bold         struct{ Elements []Inline }
	Italic       struct{ Elements []Inline }
	FootnoteRef  struct{ Target string }
	InternalLink struct {
		Target string
		Label  []Inline
	}
	ExternalLink struct {
		Target string
		Label  []Inline
	}
)

func (PlainText) isInline()    {}
func (Bold) isInline()         {}
func (Italic) isInline()       {}
func (InternalLink) isInline() {}
func (ExternalLink) isInline() {}
func (FootnoteRef) isInline()  {}

type TextBlock struct {
	Elements []Inline
}

func (TextBlock) Type() BlockType     { return BlockTypeText }
func (t TextBlock) Inlines() []Inline { return t.Elements }

type ImageBlock struct {
	Path    string
	Caption []Inline
}

func (ImageBlock) Type() BlockType     { return BlockTypeImage }
func (i ImageBlock) Inlines() []Inline { return i.Caption }

type NoteBlock struct {
	ID       string
	Elements []Inline
}

func (NoteBlock) Type() BlockType     { return BlockTypeNote }
func (n NoteBlock) Inlines() []Inline { return n.Elements }

type ListBlock struct {
	Items [][]Inline
}

func (ListBlock) Type() BlockType { return BlockTypeList }
func (l ListBlock) Inlines() []Inline {
	var all []Inline
	for _, item := range l.Items {
		all = append(all, item...)
	}
	return all
}

type Section struct {
	Level  int // 2 for ==, 3 for ===, etc.
	ID     string
	Title  string
	Blocks []Block
}

type Document struct {
	ID       string
	Title    string
	Meta     map[string]string
	Sections []Section
}
