// Package ast defines the Soffio abstract syntax tree.
package ast

type BlockType int

const (
	BlockTypeText BlockType = iota
	BlockTypeImage
	BlockTypeNote
	BlockTypeList
)

type Block interface {
	Type() BlockType
	isBlock()
}

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
	Line     int
	Elements []Inline
}

func (TextBlock) Type() BlockType { return BlockTypeText }
func (TextBlock) isBlock()        {}

type ImageBlock struct {
	Line    int
	Path    string
	Caption []Inline
}

func (ImageBlock) Type() BlockType { return BlockTypeImage }
func (ImageBlock) isBlock()        {}

type NoteBlock struct {
	Line     int
	ID       string
	Elements []Inline
}

func (NoteBlock) Type() BlockType { return BlockTypeNote }
func (NoteBlock) isBlock()        {}

type ListBlock struct {
	Line  int
	Items [][]Inline
}

func (ListBlock) Type() BlockType { return BlockTypeList }
func (ListBlock) isBlock()        {}

type Section struct {
	Level  int // Heading depth (e.g., 2 for ==).
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
