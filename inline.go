// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// inline decodes *bold*, _italic_, (label -> target), (*note) and \
// escapes; *why says what it refuses. Every marker is ASCII, so s is
// scanned by byte: a byte of a multibyte rune is never one. inLabel is
// true in a link label, where a link or a note ref is refused, as an
// <a> can't hold an <a>. There a link's own label stays text, and
// parsing goes no deeper.
func inline(s string, inLabel bool, why *string) []Inline {
	var out []Inline
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			out = append(out, PlainText{Content: text.String()})
			text.Reset()
		}
	}

	// a closer is good or not whatever opener it is for, so once none
	// is found from i on, none will be from further on: each search
	// for a marker scans the rest of s at most once
	none := [2]int{len(s), len(s)} // for '*' and '_'
	match := parens(s)

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			if i+1 < len(s) {
				i++
				c = s[i]
			}
		case '*', '_':
			k := strings.IndexByte("*_", c)
			if !opens(s, i) || i >= none[k] {
				break
			}
			if end := closer(s, i); end < 0 {
				none[k] = i
			} else {
				flush()
				in := inline(s[i+1:end], inLabel, why)
				if c == '*' {
					out = append(out, Bold{Elements: in})
				} else {
					out = append(out, Italic{Elements: in})
				}
				i = end
				continue
			}
		case '(':
			end, ok := match[i]
			if !ok {
				break
			}
			if in, ok := linkOrNote(s[i+1:end], inLabel, why); ok {
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

// opens reports whether the marker at i can open: at the start of a
// word, followed by something else than a space or itself.
func opens(s string, i int) bool {
	if i+1 >= len(s) || s[i+1] == s[i] {
		return false
	}
	before, _ := utf8.DecodeLastRuneInString(s[:i])
	after, _ := utf8.DecodeRuneInString(s[i+1:])
	return !isWord(before) && !unicode.IsSpace(after)
}

// closer finds the marker closing the one at start, at the end of a
// word, or returns -1.
func closer(s string, start int) int {
	m := s[start]
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
			return i
		}
	}
	return -1
}

// linkOrNote decodes what is between the parentheses of
// (label -> target) or (*note).
func linkOrNote(inner string, inLabel bool, why *string) (Inline, bool) {
	inner = strings.TrimSpace(inner)

	// (*id) is a note only with a valid ID: (*bold*) is prose
	if id, ok := strings.CutPrefix(inner, "*"); ok {
		if id = strings.TrimSpace(id); id != "" && CheckID(id) == "" {
			if inLabel {
				*why = "a note inside a link"
			}
			return FootnoteRef{Target: id}, true
		}
	}

	i := strings.LastIndex(inner, " -> ")
	if i < 0 {
		return nil, false
	}
	label, target := strings.TrimSpace(inner[:i]), unescape(strings.TrimSpace(inner[i+len(" -> "):]))
	switch {
	case target == "":
		return nil, false
	case inLabel:
		*why = "a link inside a link"
		return Link{Target: target, Label: []Inline{PlainText{Content: label}}}, true
	}
	return Link{Target: target, Label: inline(label, true, why)}, true
}

// parens maps each '(' of s to the ')' closing it, in one pass.
func parens(s string) map[int]int {
	if strings.IndexByte(s, '(') < 0 {
		return nil
	}
	match := map[int]int{}
	var open []int
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '(':
			open = append(open, i)
		case ')':
			if n := len(open); n > 0 {
				match[open[n-1]] = i
				open = open[:n-1]
			}
		}
	}
	return match
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
