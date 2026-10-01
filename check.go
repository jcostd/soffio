// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Check verifies the active documents: every link and image points at
// an active page, one of its sections or a file in staticDir, and every
// note is both defined and referenced. A link to a document of all
// that is not active is a privacy leak: the public site would point
// at a private page.
func Check(all, active map[string]*Document, staticDir string) error {
	var errs []error
	for _, id := range slices.Sorted(maps.Keys(active)) {
		errs = append(errs, check(active[id], func(target string) string {
			return checkTarget(all, active, staticDir, id, target)
		})...)
	}
	return errors.Join(errs...)
}

// CheckDoc verifies what doc can say alone: its notes and its links to
// its own sections. Pipe mode knows no other page and no files.
func CheckDoc(doc *Document) error {
	return errors.Join(check(doc, func(target string) string {
		if sec, ok := strings.CutPrefix(target, "#"); ok && !hasSection(doc, sec) {
			return "link to missing section"
		}
		return ""
	})...)
}

// check walks doc, asking target what is wrong with each link and
// image, and matches note refs with notes.
func check(doc *Document, target func(string) string) []error {
	var errs []error
	errorf := func(n int, format string, args ...any) {
		errs = append(errs, fmt.Errorf("%s:%d: %s", doc.File, n, fmt.Sprintf(format, args...)))
	}

	var notes []NoteBlock
	for _, sec := range doc.Sections {
		for _, b := range sec.Blocks {
			if n, ok := b.(NoteBlock); ok {
				notes = append(notes, n)
			}
		}
	}
	used := map[string]bool{}

	var line int
	var walk func([]Inline)
	walk = func(in []Inline) {
		for _, el := range in {
			switch v := el.(type) {
			case Bold:
				walk(v.Elements)
			case Italic:
				walk(v.Elements)
			case Link:
				if why := target(v.Target); why != "" {
					errorf(line, "%s %q", why, v.Target)
				}
				walk(v.Label)
			case FootnoteRef:
				used[v.Target] = true
				if !slices.ContainsFunc(notes, func(n NoteBlock) bool { return n.ID == v.Target }) {
					errorf(line, "note %q is not defined", v.Target)
				}
			}
		}
	}
	for _, sec := range doc.Sections {
		for _, b := range sec.Blocks {
			switch b := b.(type) {
			case TextBlock:
				line = b.Line
				walk(b.Elements)
			case ImageBlock:
				line = b.Line
				if why := target(b.Path); why != "" {
					errorf(line, "%s %q", why, b.Path)
				}
				walk(b.Caption)
			case NoteBlock:
				line = b.Line
				walk(b.Elements)
			case ListBlock:
				line = b.Line
				for _, item := range b.Items {
					walk(item)
				}
			}
		}
	}

	for _, n := range notes {
		if !used[n.ID] {
			errorf(n.Line, "note %q is never referenced", n.ID)
		}
	}
	return errs
}

// checkTarget says what is wrong with target in the document from, or "".
func checkTarget(all, active map[string]*Document, staticDir, from, target string) string {
	id, frag, ok := resolve(from, target)
	if !ok {
		return ""
	}
	if file, ok := strings.CutPrefix(id, "static/"); ok {
		fi, err := os.Stat(filepath.Join(staticDir, filepath.FromSlash(file)))
		if staticDir == "" || err != nil || !fi.Mode().IsRegular() {
			return "missing file"
		}
		return ""
	}
	doc := active[id]
	switch {
	case doc == nil && all[id] != nil:
		return "link to private page"
	case doc == nil:
		return "link to missing page"
	case frag != "" && !hasSection(doc, frag[1:]):
		return "link to missing section"
	}
	return ""
}

func hasSection(doc *Document, id string) bool {
	return slices.ContainsFunc(doc.Sections, func(s Section) bool { return s.ID == id })
}
