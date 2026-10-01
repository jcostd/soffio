// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// parseInline decodes *bold*, _italic_, (label -> target), (*note) and
// \ escapes. Every marker is ASCII, so s is scanned by byte: a byte of
// a multibyte rune is never one.
func parseInline(s string) []Inline {
	var out []Inline
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			out = append(out, PlainText{Content: text.String()})
			text.Reset()
		}
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			if i+1 < len(s) {
				i++
				c = s[i]
			}
		case '*', '_':
			if end, ok := closing(s, i); ok {
				flush()
				in := parseInline(s[i+1 : end])
				if c == '*' {
					out = append(out, Bold{Elements: in})
				} else {
					out = append(out, Italic{Elements: in})
				}
				i = end
				continue
			}
		case '(':
			if in, end, ok := linkOrNote(s, i); ok {
				flush()
				out = append(out, in)
				i = end
				continue
			}
		}
		text.WriteByte(c)
	}
	flush()
	return out
}

// isWord reports whether r is part of a word: a marker inside one,
// as in snake_case, is just a character.
func isWord(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// closing finds the marker closing the one at start. A marker opens
// only at the start of a word and closes only at its end.
func closing(s string, start int) (int, bool) {
	m := s[start]
	if start+1 >= len(s) || s[start+1] == m {
		return 0, false
	}
	before, _ := utf8.DecodeLastRuneInString(s[:start])
	after, _ := utf8.DecodeRuneInString(s[start+1:])
	if isWord(before) || unicode.IsSpace(after) {
		return 0, false
	}
	for i := start + 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] != m {
			continue
		}
		before, _ := utf8.DecodeLastRuneInString(s[:i])
		after, _ := utf8.DecodeRuneInString(s[i+1:])
		if !unicode.IsSpace(before) && !isWord(after) {
			return i, true
		}
	}
	return 0, false
}

// linkOrNote decodes (label -> target) or (*note) at start, and says
// where it ends.
func linkOrNote(s string, start int) (Inline, int, bool) {
	end := closeParen(s, start)
	if end < 0 {
		return nil, 0, false
	}
	inner := strings.TrimSpace(s[start+1 : end])

	// (*id) is a note only with a valid ID: (*bold*) is prose
	if id, ok := strings.CutPrefix(inner, "*"); ok {
		if id = strings.TrimSpace(id); id != "" && checkID(id) == "" {
			return FootnoteRef{Target: id}, end, true
		}
	}

	i := strings.LastIndex(inner, " -> ")
	if i < 0 {
		return nil, 0, false
	}
	target := unescape(strings.TrimSpace(inner[i+len(" -> "):]))
	if target == "" {
		return nil, 0, false
	}
	return Link{Target: target, Label: parseInline(strings.TrimSpace(inner[:i]))}, end, true
}

// closeParen returns where the ')' matching the '(' at start is, or -1.
func closeParen(s string, start int) int {
	depth := 0
	for i := start + 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
