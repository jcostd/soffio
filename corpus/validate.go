// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package corpus

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"soffio/ast"
)

// BrokenLinkError indicates a reference to a non-existent document.
type BrokenLinkError struct {
	Source string
	Line   int
	Target string
}

func (e *BrokenLinkError) Error() string {
	return fmt.Sprintf("%s:%d: [block start] broken link points to missing target '%s'", e.Source, e.Line, e.Target)
}

// BrokenNoteError indicates a reference to a non-existent footnote.
type BrokenNoteError struct {
	Source string
	Line   int
	Target string
}

func (e *BrokenNoteError) Error() string {
	return fmt.Sprintf("%s:%d: [block start] broken note ref points to missing note '%s'", e.Source, e.Line, e.Target)
}

// PrivacyLeakError indicates a public document referencing a private/excluded document.
type PrivacyLeakError struct {
	Source string
	Line   int
	Target string
}

func (e *PrivacyLeakError) Error() string {
	return fmt.Sprintf("%s:%d: [block start] PRIVACY LEAK: document references excluded/private target '%s'", e.Source, e.Line, e.Target)
}

type targetResult int

const (
	targetOK targetResult = iota
	targetNotFound
	targetPrivacyLeak
)

// ValidateLinks asserts reference integrity and guards against privacy leaks.
func (c *Collection) ValidateLinks(activeDocs map[string]*ast.Document, staticDir string) error {
	var errs []error

	// We iterate ONLY over activeDocs to ensure internal soundness.
	for id, doc := range activeDocs {
		notes := collectNotes(doc)
		for _, sec := range doc.Sections {
			for _, block := range sec.Blocks {
				switch b := block.(type) {
				case ast.TextBlock:
					walk(b.Elements, id, b.Line, notes, c.Docs, activeDocs, &errs)
				case ast.ImageBlock:
					u, err := url.Parse(b.Path)
					if err == nil && u.Scheme == "" && !strings.HasPrefix(b.Path, "//") {
						relPath := strings.TrimPrefix(b.Path, "/static/")
						relPath = strings.TrimPrefix(relPath, "/") // Fallback safety

						imgPath := filepath.Join(staticDir, filepath.FromSlash(relPath))
						if _, err := os.Stat(imgPath); os.IsNotExist(err) {
							log.Printf("soffio: warning: missing image '%s' referenced in document '%s'", b.Path, id)
						}
					}
					walk(b.Caption, id, b.Line, notes, c.Docs, activeDocs, &errs)
				case ast.NoteBlock:
					walk(b.Elements, id, b.Line, notes, c.Docs, activeDocs, &errs)
				case ast.ListBlock:
					for _, item := range b.Items {
						walk(item, id, b.Line, notes, c.Docs, activeDocs, &errs)
					}
				}
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func walk(inlines []ast.Inline, sourceID string, line int, notes map[string]struct{}, allDocs, activeDocs map[string]*ast.Document, errs *[]error) {
	for _, el := range inlines {
		switch v := el.(type) {
		case ast.Link:
			u, err := url.Parse(v.Target)
			if err == nil && u.Scheme == "" && !strings.HasPrefix(v.Target, "//") {
				res := checkTarget(allDocs, activeDocs, sourceID, v.Target)
				switch res {
				case targetNotFound:
					*errs = append(*errs, &BrokenLinkError{Source: sourceID, Line: line, Target: v.Target})
				case targetPrivacyLeak:
					*errs = append(*errs, &PrivacyLeakError{Source: sourceID, Line: line, Target: v.Target})
				}
			}
		case ast.Bold:
			walk(v.Elements, sourceID, line, notes, allDocs, activeDocs, errs)
		case ast.Italic:
			walk(v.Elements, sourceID, line, notes, allDocs, activeDocs, errs)
		case ast.FootnoteRef:
			if _, ok := notes[v.Target]; !ok {
				*errs = append(*errs, &BrokenNoteError{Source: sourceID, Line: line, Target: v.Target})
			}
		}
	}
}

// checkTarget resolves absolute and relative references within the corpus.
func checkTarget(allDocs, activeDocs map[string]*ast.Document, sourceID, target string) targetResult {
	docID, secID, hasHash := strings.Cut(target, "#")

	if docID == "" {
		docID = sourceID
	} else {
		if !strings.HasPrefix(docID, "/") {
			docID = path.Join(path.Dir(sourceID), docID)
		} else {
			docID = strings.TrimPrefix(docID, "/")
		}
	}

	doc, ok := activeDocs[docID]
	if !ok {
		_, exists := allDocs[docID]
		if exists {
			return targetPrivacyLeak
		}
		return targetNotFound
	}

	if !hasHash {
		return targetOK
	}

	for _, sec := range doc.Sections {
		if sec.ID == secID {
			return targetOK
		}
	}
	return targetNotFound
}

func collectNotes(doc *ast.Document) map[string]struct{} {
	notes := make(map[string]struct{})
	for _, sec := range doc.Sections {
		for _, block := range sec.Blocks {
			if n, ok := block.(ast.NoteBlock); ok {
				notes[n.ID] = struct{}{}
			}
		}
	}
	return notes
}
