// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package parser

import (
	"strings"
	"unicode"

	"soffio/ast"
)

func parseInline(s string) []ast.Inline {
	var elements []ast.Inline
	var buf strings.Builder
	runes := []rune(s)

	flush := func() {
		if buf.Len() > 0 {
			elements = append(elements, ast.PlainText{Content: buf.String()})
			buf.Reset()
		}
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if r == '\\' {
			if i+1 < len(runes) {
				buf.WriteRune(runes[i+1])
				i++
			} else {
				buf.WriteRune('\\')
			}
			continue
		}

		if r == '*' {
			if endIndex, ok := findClosingMarker(runes, i, '*'); ok {
				flush()

				innerRunes := runes[i+1 : endIndex]
				innerNodes := parseInline(string(innerRunes))
				elements = append(elements, ast.Bold{Elements: innerNodes})

				i = endIndex
				continue
			}
			buf.WriteRune('*')
			continue
		}

		if r == '_' {
			if endIndex, ok := findClosingMarker(runes, i, '_'); ok {
				flush()

				innerRunes := runes[i+1 : endIndex]
				innerNodes := parseInline(string(innerRunes))
				elements = append(elements, ast.Italic{Elements: innerNodes})

				i = endIndex
				continue
			}
			buf.WriteRune('_')
			continue
		}

		if r == '(' {
			if node, endIndex, ok := scanLinkOrNote(runes, i); ok {
				flush()
				elements = append(elements, node)
				i = endIndex
				continue
			}

			buf.WriteRune('(')
			continue
		}

		buf.WriteRune(r)
	}

	flush()

	return elements
}

func findClosingMarker(runes []rune, start int, marker rune) (int, bool) {
	if start+1 >= len(runes) {
		return 0, false
	}

	if unicode.IsSpace(runes[start+1]) {
		return 0, false
	}

	if runes[start+1] == marker {
		return 0, false
	}

	for i := start + 1; i < len(runes); i++ {
		if runes[i] == '\\' {
			i++
			continue
		}

		if runes[i] == marker {
			if unicode.IsSpace(runes[i-1]) {
				continue
			}

			return i, true
		}
	}

	return 0, false
}

func unescape(s string) string {
	var buf strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			i++
		}
		buf.WriteRune(runes[i])
	}
	return buf.String()
}

func scanLinkOrNote(runes []rune, start int) (ast.Inline, int, bool) {
	closeIndex := -1
	depth := 0

	for i := start + 1; i < len(runes); i++ {
		if runes[i] == '\\' {
			i++
			continue
		}
		if runes[i] == '(' {
			depth++
			continue
		}
		if runes[i] == ')' {
			if depth == 0 {
				closeIndex = i
				break
			}
			depth--
		}

	}

	if closeIndex == -1 {
		return nil, 0, false
	}

	innerStr := string(runes[start+1 : closeIndex])
	innerStr = strings.TrimSpace(innerStr)

	if strings.HasPrefix(innerStr, "*") {
		id := strings.TrimSpace(innerStr[1:])
		if id == "" {
			return nil, 0, false
		}
		return ast.FootnoteRef{Target: id}, closeIndex, true
	}

	sepIdx := strings.LastIndex(innerStr, " -> ")
	if sepIdx == -1 {
		return nil, 0, false
	}

	labelStr := innerStr[:sepIdx]
	targetStr := innerStr[sepIdx+4:] // len(" -> ") == 4

	target := unescape(strings.TrimSpace(targetStr))
	if target == "" {
		return nil, 0, false
	}

	labelNodes := parseInline(strings.TrimSpace(labelStr))

	return ast.Link{
		Target: target,
		Label:  labelNodes,
	}, closeIndex, true
}
