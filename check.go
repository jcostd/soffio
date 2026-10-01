// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Check verifies the links, note refs and images of the active
// documents. A link to a document in all that is not active is a
// privacy leak: the public site would point at a private page.
func Check(all, active map[string]*Document, staticDir string) error {
	var errs []error
	for _, id := range slices.Sorted(maps.Keys(active)) {
		doc := active[id]
		notes := map[string]bool{}
		for _, sec := range doc.Sections {
			for _, b := range sec.Blocks {
				if n, ok := b.(NoteBlock); ok {
					notes[n.ID] = true
				}
			}
		}

		var line int
		var walk func([]Inline)
		walk = func(in []Inline) {
			for _, el := range in {
				switch v := el.(type) {
				case Bold:
					walk(v.Elements)
				case Italic:
					walk(v.Elements)
				case FootnoteRef:
					if !notes[v.Target] {
						errs = append(errs, fmt.Errorf("%s:%d: [block start] broken note ref points to missing note '%s'", id, line, v.Target))
					}
				case Link:
					if why := checkLink(all, active, id, v.Target); why != "" {
						errs = append(errs, fmt.Errorf("%s:%d: [block start] %s '%s'", id, line, why, v.Target))
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
					checkImage(staticDir, id, b.Path)
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
	}
	return errors.Join(errs...)
}

// checkLink says what is wrong with target, or "".
func checkLink(all, active map[string]*Document, from, target string) string {
	id, frag, ok := resolve(from, target)
	if !ok {
		return ""
	}
	doc, ok := active[id]
	switch {
	case !ok && all[id] != nil:
		return "PRIVACY LEAK: document references excluded/private target"
	case !ok:
		return "broken link points to missing target"
	case frag == "":
		return ""
	}
	for _, sec := range doc.Sections {
		if "#"+sec.ID == frag {
			return ""
		}
	}
	return "broken link points to missing target"
}

// checkImage warns about an image of the site missing from staticDir.
func checkImage(staticDir, id, src string) {
	if _, _, ok := resolve(id, src); !ok {
		return
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(src, "/static/"), "/")
	if _, err := os.Stat(filepath.Join(staticDir, filepath.FromSlash(rel))); errors.Is(err, os.ErrNotExist) {
		log.Printf("soffio: warning: missing image '%s' referenced in document '%s'", src, id)
	}
}
