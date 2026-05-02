// Package ast defines the abstract syntax tree for the soffio markup.
package ast

// BlockType identifies the kind of content a block holds.
type BlockType int

const (
	BlockTypeText BlockType = iota
	BlockTypeMedia
)

// Block is the interface implemented by all document blocks.
type Block interface {
	Type() BlockType
}

// TextBlock represents a contiguous block of text.
type TextBlock struct {
	Content string
}

func (TextBlock) Type() BlockType { return BlockTypeText }

// MediaBlock represents embedded media with an optional caption.
type MediaBlock struct {
	Path    string
	Caption string
}

func (MediaBlock) Type() BlockType { return BlockTypeMedia }

// Section is a distinct part of a document, containing blocks.
type Section struct {
	ID     string
	Title  string
	Blocks []Block
}

// Document represents a complete parsed soffio file.
type Document struct {
	ID        string
	Language  string
	Title     string
	Author    string
	Year      string
	Technique string

	Sections []Section
}
