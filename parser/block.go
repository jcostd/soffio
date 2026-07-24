// Package parser decodes Soffio markup.
package parser

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"soffio/ast"
)

type state int

const (
	stateHeader state = iota
	stateBody
)

type ParseError struct {
	Line    int
	Message string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("parser error at line %d: %s", e.Line, e.Message)
}

type parser struct {
	scan         *bufio.Scanner
	doc          ast.Document
	buf          strings.Builder
	currentBlock string
	blockMeta    string
	state        state
	lineCount    int
	blockStart   int
	errors       []error
}

// Parse decodes r strictly.
func Parse(r io.Reader) (ast.Document, error) {
	p := &parser{
		scan:  bufio.NewScanner(r),
		state: stateHeader,
		doc: ast.Document{
			Meta: make(map[string]string),
		},
	}

	for p.scan.Scan() {
		p.lineCount++
		line := strings.TrimSpace(p.scan.Text())
		p.step(line)
	}

	p.flush()

	if err := p.scan.Err(); err != nil {
		p.errors = append(p.errors, err)
	}
	if len(p.errors) > 0 {
		return p.doc, errors.Join(p.errors...)
	}
	return p.doc, nil
}

func (p *parser) addError(msg string) {
	p.errors = append(p.errors, ParseError{
		Line:    p.lineCount,
		Message: msg,
	})
}

func (p *parser) step(line string) {
	if p.state == stateHeader {
		p.stepHeader(line)
	} else {
		p.stepBody(line)
	}
}

func (p *parser) stepHeader(line string) {
	// RFC 822 Mail style: an empty line terminates the header.
	if line == "" {
		p.state = stateBody
		return
	}

	key, val, ok := strings.Cut(line, ":")
	if !ok {
		// invalid metadata!
		p.addError(fmt.Sprintf("invalid syntax (expected 'key: value'), found: %q", line))
		return
	}

	key = strings.ToLower(strings.TrimSpace(key))
	val = strings.TrimSpace(val)

	if key == "" {
		p.addError("empty header key found")
		return
	}

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

	if strings.HasPrefix(line, "==") || strings.HasPrefix(line, ":: ") {
		p.flush()
	}

	if p.buf.Len() == 0 {
		p.blockStart = p.lineCount
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
		p.addError(fmt.Sprintf("invalid section level (%d), must be between 2 and 6.", level))
		return true
	}

	payload := line[level:]
	rawID, rawTitle, ok := strings.Cut(payload, "|")
	if !ok {
		p.addError(fmt.Sprintf("malformed section (expected '== id | Title'), found: %q", line))
		return true
	}

	id := strings.TrimSpace(rawID)
	title := strings.TrimSpace(rawTitle)

	if id == "" || title == "" {
		p.addError(fmt.Sprintf("malformed section (both ID and Title must be non-empty), found: %q", line))
		return true
	}

	p.doc.Sections = append(p.doc.Sections, ast.Section{
		Level: level,
		ID:    id,
		Title: title,
	})
	return true
}

func (p *parser) tryParseCommand(line string) bool {
	if !strings.HasPrefix(line, ":: ") {
		return false
	}

	// syntax: :: cmd: meta | content
	raw := line[3:]
	cmd, payload, ok := strings.Cut(raw, ": ")
	if !ok {
		p.addError(fmt.Sprintf("malformed command (expected ':: cmd: ...'), found: %q", line))
		return true
	}

	cmd = strings.TrimSpace(cmd)
	if cmd != "img" && cmd != "note" {
		p.addError(fmt.Sprintf("unknown command %q (expected 'img' or 'note')", cmd))
		return true
	}

	meta, content, ok := strings.Cut(payload, " | ")
	if !ok {
		p.addError(fmt.Sprintf("malformed command (expected ':: cmd: meta | content'), found: %q", line))
		return true
	}

	p.currentBlock = strings.TrimSpace(cmd)
	p.blockMeta = strings.TrimSpace(meta)
	p.buf.WriteString(strings.TrimSpace(content))
	return true
}

func (p *parser) flush() {
	if p.buf.Len() == 0 {
		return
	}

	// found text but no section created, error
	if len(p.doc.Sections) == 0 {
		p.addError("found block content outside any section (no '== id | Title' declared)")
		p.buf.Reset()
		p.currentBlock = ""
		p.blockMeta = ""
		return
	}

	content := p.buf.String()
	var block ast.Block

	switch p.currentBlock {
	case "img":
		block = ast.ImageBlock{
			Line:    p.blockStart,
			Path:    p.blockMeta,
			Caption: parseInline(content),
		}
	case "note":
		block = ast.NoteBlock{
			Line:     p.blockStart,
			ID:       p.blockMeta,
			Elements: parseInline(content),
		}
	case "list":
		var items [][]ast.Inline
		var currentItem strings.Builder

		for itemLine := range strings.SplitSeq(content, "\n") {
			if after, ok := strings.CutPrefix(itemLine, "- "); ok {
				// New item starts: flush the previous one if it exists
				if currentItem.Len() > 0 {
					items = append(items, parseInline(currentItem.String()))
					currentItem.Reset()
				}
				currentItem.WriteString(after)
			} else if currentItem.Len() > 0 {
				// Natural Continuation: append to the current item
				currentItem.WriteByte('\n')
				currentItem.WriteString(itemLine)
			}
		}

		// Flush the final accumulated item
		if currentItem.Len() > 0 {
			items = append(items, parseInline(currentItem.String()))
		}
		block = ast.ListBlock{
			Line:  p.blockStart,
			Items: items,
		}

	default:
		block = ast.TextBlock{
			Line:     p.blockStart,
			Elements: parseInline(content),
		}
	}

	last := len(p.doc.Sections) - 1
	p.doc.Sections[last].Blocks = append(p.doc.Sections[last].Blocks, block)

	p.buf.Reset()
	p.currentBlock = ""
	p.blockMeta = ""
}
