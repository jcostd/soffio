// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
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
	doc          Document
	buf          strings.Builder
	currentBlock string
	blockMeta    string
	state        state
	lineCount    int
	blockStart   int
	errors       []error
}

// Parse decodes r strictly.
func Parse(r io.Reader) (Document, error) {
	p := &parser{
		scan:  bufio.NewScanner(r),
		state: stateHeader,
		doc: Document{
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
		if why := checkID(val); why != "" {
			p.addError(fmt.Sprintf("invalid id %q: %s", val, why))
			return
		}
		p.doc.ID = val
	case "title":
		p.doc.Title = val
	default:
		if isDateKey(key) && val != "" {
			if _, err := time.Parse("2006-01-02", val); err != nil {
				p.addError(fmt.Sprintf("invalid %s %q: expected a real date as YYYY-MM-DD", key, val))
				return
			}
		}
		p.doc.Meta[key] = val
	}
}

// isDateKey reports whether a frontmatter key holds a date: "date",
// "updated" and any "*_date". They sort pages, so a date that is not
// YYYY-MM-DD would silently misplace one.
func isDateKey(key string) bool {
	return key == "date" || key == "updated" || strings.HasSuffix(key, "_date")
}

// idForbidden are the ASCII characters an ID can't hold: path separators
// and what Windows forbids in a file name, then '#' (section anchor), '%'
// (URL escape) and parentheses (they close a link).
const idForbidden = `/\:*?"<>|#%()`

// checkID says what makes id unusable as a file name, page address or
// anchor, or "" if nothing does. An ID is printable ASCII: no spaces, no
// accents. The same rule holds for documents, sections and notes.
func checkID(id string) string {
	if strings.HasPrefix(id, ".") {
		return "it can't start with '.'"
	}
	for _, r := range id {
		switch {
		case r == ' ' || r == '\t':
			return "it contains a space"
		case r < 0x21 || r > 0x7e:
			return fmt.Sprintf("%q is not a plain ASCII character", r)
		case strings.ContainsRune(idForbidden, r):
			return fmt.Sprintf("%q is not allowed", r)
		}
	}
	return ""
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
		if strings.HasPrefix(line, "-") {
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
	if why := checkID(id); why != "" {
		p.addError(fmt.Sprintf("invalid section id %q: %s", id, why))
		return true
	}

	p.doc.Sections = append(p.doc.Sections, Section{
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

	meta = strings.TrimSpace(meta)
	if cmd == "note" {
		if why := checkID(meta); why != "" {
			p.addError(fmt.Sprintf("invalid note id %q: %s", meta, why))
			return true
		}
	}

	p.currentBlock = cmd
	p.blockMeta = meta
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
	var block Block

	switch p.currentBlock {
	case "img":
		block = ImageBlock{
			Line:    p.blockStart,
			Path:    p.blockMeta,
			Caption: parseInline(content),
		}
	case "note":
		block = NoteBlock{
			Line:     p.blockStart,
			ID:       p.blockMeta,
			Elements: parseInline(content),
		}
	case "list":
		var items [][]Inline
		var currentItem strings.Builder

		for itemLine := range strings.SplitSeq(content, "\n") {
			trimmedLine := strings.TrimSpace(itemLine)

			if strings.HasPrefix(trimmedLine, "-") {
				if currentItem.Len() > 0 {
					items = append(items, parseInline(currentItem.String()))
					currentItem.Reset()
				}
				after := strings.TrimSpace(trimmedLine[1:])
				currentItem.WriteString(after)
			} else if currentItem.Len() > 0 && trimmedLine != "" {
				currentItem.WriteByte('\n')
				currentItem.WriteString(trimmedLine)
			}
		}

		// Flush the final accumulated item
		if currentItem.Len() > 0 {
			items = append(items, parseInline(currentItem.String()))
		}
		block = ListBlock{
			Line:  p.blockStart,
			Items: items,
		}

	default:
		block = TextBlock{
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
