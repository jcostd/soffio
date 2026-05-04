// Package parser decodes Soffio markup.
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
	scan         *bufio.Scanner
	doc          ast.Document
	buf          strings.Builder
	currentBlock string
	blockMeta    string
	state        state
}

// Parse decodes r.
func Parse(r io.Reader) (ast.Document, error) {
	p := &parser{
		scan:  bufio.NewScanner(r),
		state: stateHeader,
		doc: ast.Document{
			Meta: make(map[string]string),
		},
	}

	for p.scan.Scan() {
		p.step(strings.TrimSpace(p.scan.Text()))
	}

	p.flush()
	return p.doc, p.scan.Err()
}

func (p *parser) step(line string) {
	if p.state == stateHeader {
		p.stepHeader(line)
	} else {
		p.stepBody(line)
	}
}

func (p *parser) stepHeader(line string) {
	if line == "" {
		p.state = stateBody
		return
	}

	key, val, ok := strings.Cut(line, ":")
	if !ok {
		return
	}

	key = strings.ToLower(strings.TrimSpace(key))
	val = strings.TrimSpace(val)

	switch key {
	case "id":
		p.doc.ID = val
	case "title", "titolo":
		p.doc.Title = val
	default:
		p.doc.Meta[key] = val
	}
}

func (p *parser) stepBody(line string) {
	if line == "" {
		p.flush()
		return
	}

	if p.buf.Len() == 0 {
		if p.tryParseSection(line) {
			return
		}
		if p.tryParseCommand(line) {
			return
		}
		if strings.HasPrefix(line, "- ") {
			p.currentBlock = "list"
		}
	}

	if p.buf.Len() > 0 {
		p.buf.WriteByte('\n')
	}
	p.buf.WriteString(line)
}

func (p *parser) tryParseSection(line string) bool {
	if !strings.HasPrefix(line, "==") {
		return false
	}

	level := 0
	for _, ch := range line {
		if ch == '=' {
			level++
		} else {
			break
		}
	}

	if level < 2 || level > 6 {
		return false
	}

	payload := strings.TrimSpace(line[level:])
	id, title, ok := strings.Cut(payload, " | ")
	if !ok {
		return false
	}

	p.doc.Sections = append(p.doc.Sections, ast.Section{
		Level: level,
		ID:    strings.TrimSpace(id),
		Title: strings.TrimSpace(title),
	})
	return true
}

func (p *parser) tryParseCommand(line string) bool {
	if !strings.HasPrefix(line, ":: ") {
		return false
	}

	cmd, payload, ok := strings.Cut(line[3:], ": ")
	if !ok {
		return false
	}

	meta, content, ok := strings.Cut(payload, " | ")
	if !ok {
		return false
	}

	p.currentBlock = strings.TrimSpace(cmd)
	p.blockMeta = strings.TrimSpace(meta)
	p.buf.WriteString(strings.TrimSpace(content))
	return true
}

func (p *parser) flush() {
	if p.buf.Len() == 0 || len(p.doc.Sections) == 0 {
		return
	}

	content := p.buf.String()
	var block ast.Block

	switch p.currentBlock {
	case "img":
		block = ast.ImageBlock{
			Path:    p.blockMeta,
			Caption: parseInline(content),
		}
	case "nota":
		block = ast.NoteBlock{
			ID:       p.blockMeta,
			Elements: parseInline(content),
		}
	case "list":
		var items [][]ast.Inline
		for itemLine := range strings.SplitSeq(content, "\n") {
			if after, ok := strings.CutPrefix(itemLine, "- "); ok {
				items = append(items, parseInline(after))
			}
		}
		block = ast.ListBlock{Items: items}
	default:
		block = ast.TextBlock{Elements: parseInline(content)}
	}

	last := len(p.doc.Sections) - 1
	p.doc.Sections[last].Blocks = append(p.doc.Sections[last].Blocks, block)

	p.buf.Reset()
	p.currentBlock = ""
	p.blockMeta = ""
}
