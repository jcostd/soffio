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

type parser struct {
	doc      Document
	errs     []error
	keys     map[string]bool // header keys seen
	sections map[string]bool
	notes    map[string]bool
}

// errorf records an error at line n, as file:n: message.
func (p *parser) errorf(n int, format string, args ...any) {
	p.errs = append(p.errs, fmt.Errorf("%s:%d: %s", p.doc.File, n, fmt.Sprintf(format, args...)))
}

// Parse decodes the text read from r; name is where it comes from, for
// messages. A text is header lines of key: value, a blank line, then
// the body, cut into blocks: a blank line ends a block; a section or a
// command line starts one.
func Parse(name string, r io.Reader) (*Document, error) {
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(src), "\n")
	p := parser{
		doc:      Document{File: name, Meta: map[string]string{}},
		keys:     map[string]bool{},
		sections: map[string]bool{},
		notes:    map[string]bool{},
	}

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

	return &p.doc, errors.Join(p.errs...)
}

func (p *parser) header(n int, line string) {
	key, val, ok := strings.Cut(line, ":")
	if !ok {
		p.errorf(n, "%q is not key: value", line)
		return
	}
	key = strings.ToLower(strings.TrimSpace(key))
	val = strings.TrimSpace(val)

	switch {
	case key == "":
		p.errorf(n, "empty header key")
		return
	case p.keys[key]:
		p.errorf(n, "duplicate header key %q", key)
		return
	}
	p.keys[key] = true

	switch {
	case key == "id":
		if why := CheckID(val); why != "" {
			p.errorf(n, "invalid id %q: %s", val, why)
			return
		}
		p.doc.ID = val
	case key == "title":
		p.doc.Title = val
	case key == "visibility" && val != "public" && val != "private":
		p.errorf(n, "invalid visibility %q: want public or private", val)
	case isDateKey(key) && val != "":
		if _, err := time.Parse(time.DateOnly, val); err != nil {
			p.errorf(n, "invalid %s %q: want a real date, YYYY-MM-DD", key, val)
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

// CheckID says what makes id unusable as a file name, page address or
// anchor, or "" if nothing does. An ID is printable ASCII: no spaces, no
// accents. The same rule holds for documents, their directories,
// sections, notes and languages.
func CheckID(id string) string {
	if id == "" {
		return "it is empty"
	}
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
	if level > 6 {
		p.errorf(n, "section level %d: want 2 to 6", level)
		return
	}
	id, title, ok := strings.Cut(line[level:], "|")
	if !ok {
		p.errorf(n, "%q is not == id | Title", line)
		return
	}
	id, title = strings.TrimSpace(id), strings.TrimSpace(title)
	if id == "" || title == "" {
		p.errorf(n, "section %q needs an id and a title", line)
		return
	}
	if why := CheckID(id); why != "" {
		p.errorf(n, "invalid section id %q: %s", id, why)
		return
	}
	if p.sections[id] {
		p.errorf(n, "duplicate section id %q", id)
		return
	}
	p.sections[id] = true
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
		if b = p.list(n, lines); b == nil {
			return
		}
	default:
		b = TextBlock{Line: n, Elements: parseInline(strings.Join(lines, "\n"))}
	}
	if len(p.doc.Sections) == 0 {
		p.errorf(n, "text before the first section")
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
		p.errorf(n, "%q is not :: cmd: arg | text", line)
		return nil
	}
	cmd = strings.TrimSpace(cmd)
	if cmd != "img" && cmd != "note" {
		p.errorf(n, "unknown command %q: want img or note", cmd)
		return nil
	}
	arg, text, ok := strings.Cut(payload, " | ")
	if !ok {
		p.errorf(n, "%q is not :: cmd: arg | text", line)
		return nil
	}
	arg = strings.TrimSpace(arg)
	text = strings.Join(append([]string{strings.TrimSpace(text)}, more...), "\n")

	if cmd == "img" {
		// an image of the site is a static file: the same path from
		// every page
		if _, _, ok := resolve("", arg); ok && !strings.HasPrefix(arg, "/static/") {
			p.errorf(n, "image %q is not under /static/", arg)
			return nil
		}
		return ImageBlock{Line: n, Path: arg, Caption: parseInline(text)}
	}
	if why := CheckID(arg); why != "" {
		p.errorf(n, "invalid note id %q: %s", arg, why)
		return nil
	}
	if p.notes[arg] {
		p.errorf(n, "duplicate note id %q", arg)
		return nil
	}
	p.notes[arg] = true
	return NoteBlock{Line: n, ID: arg, Elements: parseInline(text)}
}

// list decodes "- item" lines; a line without '-' goes on with the
// item before it. A block has no blank line, so line i is n+i.
func (p *parser) list(n int, lines []string) Block {
	var items [][]string
	for i, line := range lines {
		rest, ok := strings.CutPrefix(line, "-")
		if !ok {
			last := len(items) - 1
			items[last] = append(items[last], line)
			continue
		}
		if rest = strings.TrimSpace(rest); rest == "" {
			p.errorf(n+i, "empty list item")
			return nil
		}
		items = append(items, []string{rest})
	}
	b := ListBlock{Line: n}
	for _, item := range items {
		b.Items = append(b.Items, parseInline(strings.Join(item, "\n")))
	}
	return b
}
