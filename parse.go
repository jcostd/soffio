// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// A ParseError is a line Parse refused.
type ParseError struct {
	Line    int
	Message string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("parser error at line %d: %s", e.Line, e.Message)
}

type parser struct {
	doc  Document
	errs []error
}

func (p *parser) errorf(line int, format string, args ...any) {
	p.errs = append(p.errs, ParseError{line, fmt.Sprintf(format, args...)})
}

// Parse decodes r strictly: header lines of key: value, a blank line,
// then the body, cut into blocks. A blank line ends a block; a section
// or a command line starts one.
func Parse(r io.Reader) (Document, error) {
	src, err := io.ReadAll(r)
	if err != nil {
		return Document{}, err
	}
	lines := strings.Split(string(src), "\n")
	p := parser{doc: Document{Meta: map[string]string{}}}

	n := 0
	for n < len(lines) {
		line := strings.TrimSpace(lines[n])
		n++
		if line == "" {
			break
		}
		p.header(n, line)
	}

	var block []string
	start := 0
	for i := n; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "==") || strings.HasPrefix(line, ":: ") {
			p.block(start, block)
			block = nil
		}
		switch {
		case line == "":
		case strings.HasPrefix(line, "=="):
			p.section(i+1, line)
		default:
			if block == nil {
				start = i + 1
			}
			block = append(block, line)
		}
	}
	p.block(start, block)

	return p.doc, errors.Join(p.errs...)
}

func (p *parser) header(n int, line string) {
	key, val, ok := strings.Cut(line, ":")
	if !ok {
		p.errorf(n, "invalid syntax (expected 'key: value'), found: %q", line)
		return
	}
	key = strings.ToLower(strings.TrimSpace(key))
	val = strings.TrimSpace(val)

	switch {
	case key == "":
		p.errorf(n, "empty header key found")
	case key == "id":
		if why := checkID(val); why != "" {
			p.errorf(n, "invalid id %q: %s", val, why)
			return
		}
		p.doc.ID = val
	case key == "title":
		p.doc.Title = val
	case isDateKey(key) && val != "":
		if _, err := time.Parse(time.DateOnly, val); err != nil {
			p.errorf(n, "invalid %s %q: expected a real date as YYYY-MM-DD", key, val)
			return
		}
		fallthrough
	default:
		p.doc.Meta[key] = val
	}
}

// isDateKey reports whether a header key holds a date: "date",
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

// section decodes "== id | Title": as many '=' as the level, 2 to 6.
func (p *parser) section(n int, line string) {
	level := len(line) - len(strings.TrimLeft(line, "="))
	if level < 2 || level > 6 {
		p.errorf(n, "invalid section level (%d), must be between 2 and 6.", level)
		return
	}
	id, title, ok := strings.Cut(line[level:], "|")
	if !ok {
		p.errorf(n, "malformed section (expected '== id | Title'), found: %q", line)
		return
	}
	id, title = strings.TrimSpace(id), strings.TrimSpace(title)
	if id == "" || title == "" {
		p.errorf(n, "malformed section (both ID and Title must be non-empty), found: %q", line)
		return
	}
	if why := checkID(id); why != "" {
		p.errorf(n, "invalid section id %q: %s", id, why)
		return
	}
	p.doc.Sections = append(p.doc.Sections, Section{Level: level, ID: id, Title: title})
}

// block adds the block made of lines, which starts at line n, to the
// last section. Its first line says what it is.
func (p *parser) block(n int, lines []string) {
	if lines == nil {
		return
	}
	var b Block
	switch first := lines[0]; {
	case strings.HasPrefix(first, ":: "):
		if b = p.command(n, first, lines[1:]); b == nil {
			return
		}
	case strings.HasPrefix(first, "-"):
		b = ListBlock{Line: n, Items: listItems(lines)}
	default:
		b = TextBlock{Line: n, Elements: parseInline(strings.Join(lines, "\n"))}
	}
	if len(p.doc.Sections) == 0 {
		p.errorf(n, "found block content outside any section (no '== id | Title' declared)")
		return
	}
	sec := &p.doc.Sections[len(p.doc.Sections)-1]
	sec.Blocks = append(sec.Blocks, b)
}

// command decodes ":: img: path | caption" or ":: note: id | text";
// the lines after it go on with the caption or text.
func (p *parser) command(n int, line string, more []string) Block {
	cmd, payload, ok := strings.Cut(line[len(":: "):], ": ")
	if !ok {
		p.errorf(n, "malformed command (expected ':: cmd: ...'), found: %q", line)
		return nil
	}
	cmd = strings.TrimSpace(cmd)
	if cmd != "img" && cmd != "note" {
		p.errorf(n, "unknown command %q (expected 'img' or 'note')", cmd)
		return nil
	}
	meta, text, ok := strings.Cut(payload, " | ")
	if !ok {
		p.errorf(n, "malformed command (expected ':: cmd: meta | content'), found: %q", line)
		return nil
	}
	meta = strings.TrimSpace(meta)
	text = strings.Join(append([]string{strings.TrimSpace(text)}, more...), "\n")

	if cmd == "img" {
		return ImageBlock{Line: n, Path: meta, Caption: parseInline(text)}
	}
	if why := checkID(meta); why != "" {
		p.errorf(n, "invalid note id %q: %s", meta, why)
		return nil
	}
	return NoteBlock{Line: n, ID: meta, Elements: parseInline(text)}
}

// listItems decodes "- item" lines; a line without '-' goes on with
// the item before it.
func listItems(lines []string) [][]Inline {
	var items [][]Inline
	var item []string
	for _, line := range lines {
		if rest, ok := strings.CutPrefix(line, "-"); ok {
			if item != nil {
				items = append(items, parseInline(strings.Join(item, "\n")))
				item = nil
			}
			if rest = strings.TrimSpace(rest); rest != "" {
				item = []string{rest}
			}
		} else if item != nil {
			item = append(item, line)
		}
	}
	if item != nil {
		items = append(items, parseInline(strings.Join(item, "\n")))
	}
	return items
}
