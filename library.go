package main

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"soffio/parser"
)

var linkRX = regexp.MustCompile(`\[\[(.*?)->(.*?)\]\]`)

var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrDuplicateID      = errors.New("duplicated document ID")
)

type BrokenLinkError struct {
	SourceDocID string
	TargetID    string
}

func (e *BrokenLinkError) Error() string {
	return fmt.Sprintf("broken link: document '%s' points at '%s' but doesn't exist", e.SourceDocID, e.TargetID)
}

type Library struct {
	Documents map[string]*parser.Document
}

func (l *Library) Load(fsys fs.FS, pattern string) error {
	files, err := fs.Glob(fsys, pattern)
	if err != nil {
		return fmt.Errorf("glob pattern error: %w", err)
	}

	var loadErrs []error // Raccoglitore per tutti gli errori in fase di caricamento

	for _, filename := range files {
		f, err := fsys.Open(filename)
		if err != nil {
			loadErrs = append(loadErrs, fmt.Errorf("opening %s: %w", filename, err))
			continue
		}

		doc, err := parser.Parse(f)
		f.Close()

		if err != nil {
			loadErrs = append(loadErrs, fmt.Errorf("parsing %s: %w", filename, err))
			continue
		}

		if _, exists := l.Documents[doc.ID]; exists {
			// Usiamo %w per "wrappare" (avvolgere) il nostro sentinel error standard
			loadErrs = append(loadErrs, fmt.Errorf("%w: '%s' found in file %s", ErrDuplicateID, doc.ID, filename))
			continue
		}

		l.Documents[doc.ID] = &doc
	}

	return errors.Join(loadErrs...)
}

func (l *Library) PrintEntries() {
	for k := range l.Documents {
		fmt.Println(k)
	}
}

func (l *Library) ValidateLinks() []error {
	var report []error

	for k, doc := range l.Documents {
		for _, sec := range doc.Sections {
			for _, block := range sec.Blocks {
				if block.Type() != parser.BlockTypeText {
					continue
				}

				t := block.(parser.TextBlock)
				matches := linkRX.FindAllStringSubmatch(t.Content, -1)

			MatchLoop:
				for _, m := range matches {
					target := strings.TrimSpace(m[2])
					if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
						continue
					}

					if docID, sectionID, hasHash := strings.Cut(target, "#"); hasHash {
						if refDoc, ok := l.Documents[docID]; ok {
							for _, refSec := range refDoc.Sections {
								if refSec.ID == sectionID {
									continue MatchLoop
								}
							}
						}
					} else if l.HasDocument(target) {
						continue
					}

					report = append(report, &BrokenLinkError{
						SourceDocID: k,
						TargetID:    target,
					})
				}
			}
		}
	}

	return report
}

func (l *Library) GetDocument(id string) (*parser.Document, error) {
	doc, ok := l.Documents[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrDocumentNotFound, id)
	}

	return doc, nil
}

func (l *Library) HasDocument(id string) bool {
	_, ok := l.Documents[id]
	return ok
}
