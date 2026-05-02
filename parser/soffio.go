// Package parser implements a parser for the soffio markup language.
package parser

import (
	"bufio"
	"io"
	"strings"

	"soffio/ast"
)

type state int

const (
	stateHeader state = iota
	stateBody
)

type parser struct {
	scan  *bufio.Scanner
	doc   ast.Document
	buf   strings.Builder
	state state
}

// Parse reads a soffio formatted document from r and returns its AST.
func Parse(r io.Reader) (ast.Document, error) {
	p := &parser{
		scan:  bufio.NewScanner(r),
		state: stateHeader,
	}

	for p.scan.Scan() {
		p.step(strings.TrimSpace(p.scan.Text()))
	}

	p.flush()
	return p.doc, p.scan.Err()
}

func (p *parser) step(line string) {
	switch p.state {
	case stateHeader:
		if line == "" {
			p.state = stateBody
			return
		}
		p.parseHeader(line)

	case stateBody:
		if line == "" {
			p.flush()
			return
		}
		if cmd, ok := strings.CutPrefix(line, ":: "); ok {
			p.flush()
			p.parseCommand(cmd)
			return
		}
		p.appendLine(line)
	}
}

func (p *parser) parseHeader(line string) {
	key, val, ok := strings.Cut(line, ":")
	if !ok {
		return
	}

	switch strings.TrimSpace(key) {
	case "ID":
		p.doc.ID = strings.TrimSpace(val)
	case "Title", "Titolo":
		p.doc.Title = strings.TrimSpace(val)
	case "Author", "Autore":
		p.doc.Author = strings.TrimSpace(val)
	case "Year", "Anno":
		p.doc.Year = strings.TrimSpace(val)
	case "Technique", "Tecnica":
		p.doc.Technique = strings.TrimSpace(val)
	case "Language", "Lingua":
		p.doc.Language = strings.TrimSpace(val)
	}
}

func (p *parser) parseCommand(cmd string) {
	if strings.HasPrefix(strings.ToLower(cmd), "media: ") {
		_, payload, _ := strings.Cut(cmd, ": ")
		path, caption, _ := strings.Cut(payload, "|")

		if len(p.doc.Sections) > 0 {
			p.addBlock(ast.MediaBlock{
				Path:    strings.TrimSpace(path),
				Caption: strings.TrimSpace(caption),
			})
		}
		return
	}

	id, title, hasPipe := strings.Cut(cmd, "|")
	if !hasPipe {
		id, title = cmd, cmd
	}

	p.doc.Sections = append(p.doc.Sections, ast.Section{
		ID:    strings.TrimSpace(id),
		Title: strings.TrimSpace(title),
	})
}

func (p *parser) appendLine(line string) {
	if len(p.doc.Sections) == 0 {
		return
	}
	if p.buf.Len() > 0 {
		p.buf.WriteByte('\n')
	}
	p.buf.WriteString(line)
}

func (p *parser) flush() {
	if p.buf.Len() == 0 || len(p.doc.Sections) == 0 {
		return
	}
	p.addBlock(ast.TextBlock{Content: p.buf.String()})
	p.buf.Reset()
}

func (p *parser) addBlock(b ast.Block) {
	last := len(p.doc.Sections) - 1
	p.doc.Sections[last].Blocks = append(p.doc.Sections[last].Blocks, b)
}
