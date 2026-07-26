package ast

import "testing"

func TestInterfaces(t *testing.T) {
	blocks := []Block{
		TextBlock{}, ImageBlock{}, NoteBlock{}, ListBlock{},
	}
	for _, b := range blocks {
		b.isBlock()
		_ = b.Type()
	}

	inlines := []Inline{
		PlainText{}, Bold{}, Italic{}, Link{}, FootnoteRef{},
	}
	for _, i := range inlines {
		i.isInline()
	}
}
