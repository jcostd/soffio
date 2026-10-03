// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Check verifies the active documents: every link and image points at
// an active page, one of its sections or a file in one of staticDirs,
// and every
// note is both defined and referenced. A link to a document of all
// that is not active is a privacy leak: the public site would point
// at a private page.
func Check(all, active map[string]*Document, staticDirs []string) error {
	var errs []error
	for _, id := range slices.Sorted(maps.Keys(active)) {
		errs = append(errs, check(active[id], func(target string) string {
			return checkTarget(all, active, staticDirs, id, target)
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
// image, and matches note refs with notes. A note counts as referenced
// only if the body leads to it, maybe through other notes: that is
// what Render puts at the end; a note only it refers to would be lost.
func check(doc *Document, target func(string) string) []error {
	var errs []error
	errorf := func(n int, format string, args ...any) {
		errs = append(errs, fmt.Errorf("%s:%d: %s", doc.File, n, fmt.Sprintf(format, args...)))
	}
	address := func(n int, t string) {
		why := target(t)
		if _, err := url.Parse(t); err != nil {
			why = "invalid address"
		}
		if why != "" {
			errorf(n, "%s %q", why, t)
		}
	}

	notes := map[string]NoteBlock{}
	var order []NoteBlock
	for _, sec := range doc.Sections {
		for _, b := range sec.Blocks {
			if n, ok := b.(NoteBlock); ok {
				notes[n.ID] = n
				order = append(order, n)
			}
		}
	}
	// the images of the header, in their order
	keys := slices.SortedFunc(maps.Keys(doc.Meta), func(a, b string) int {
		return cmp.Or(cmp.Compare(doc.lines[a], doc.lines[b]), cmp.Compare(a, b))
	})
	for _, key := range keys {
		if isImageKey(key) && doc.Meta[key] != "" {
			address(doc.lines[key], doc.Meta[key])
		}
	}

	var queue []NoteBlock // referenced, in order of first reference
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
				address(line, v.Target)
				walk(v.Label)
			case FootnoteRef:
				n, ok := notes[v.Target]
				switch {
				case !ok:
					errorf(line, "note %q is not defined", v.Target)
				case !used[v.Target]:
					used[v.Target] = true
					queue = append(queue, n)
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
				address(line, b.Path)
				walk(b.Caption)
			case ListBlock:
				line = b.Line
				for _, item := range b.Items {
					walk(item)
				}
			}
		}
	}
	for i := 0; i < len(queue); i++ {
		line = queue[i].Line
		walk(queue[i].Elements)
	}

	for _, n := range order {
		if !used[n.ID] {
			errorf(n.Line, "note %q is never referenced", n.ID)
		}
	}
	return errs
}

// checkTarget says what is wrong with target in the document from, or "".
func checkTarget(all, active map[string]*Document, staticDirs []string, from, target string) string {
	id, frag, ok := resolve(from, target)
	if !ok {
		return ""
	}
	if file, ok := strings.CutPrefix(id, "static/"); ok {
		if strings.Contains("/"+file, "/.") {
			return "hidden file"
		}
		for _, dir := range staticDirs {
			if fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(file))); err == nil && fi.Mode().IsRegular() {
				return ""
			}
		}
		return "missing file"
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
