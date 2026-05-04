// Package corpus indexes and validates Soffio documents.
package corpus

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"soffio/ast"
	"soffio/parser"
)

var (
	ErrNotFound    = errors.New("document not found")
	ErrDuplicateID = errors.New("duplicate document ID")
)

type BrokenLinkError struct {
	Source string
	Target string
}

func (e *BrokenLinkError) Error() string {
	return fmt.Sprintf("broken link: '%s' points to missing target '%s'", e.Source, e.Target)
}

type BrokenNoteError struct {
	Source string
	Target string
}

func (e *BrokenNoteError) Error() string {
	return fmt.Sprintf("broken note ref: '%s' points to missing note '%s'", e.Source, e.Target)
}

type Collection struct {
	Docs map[string]*ast.Document
}

func New() *Collection {
	return &Collection{
		Docs: make(map[string]*ast.Document),
	}
}

type parseResult struct {
	filename string
	doc      *ast.Document
	err      error
}

// Load concurrently decodes pattern-matched files into c.
func (c *Collection) Load(fsys fs.FS, pattern string) error {
	files, err := fs.Glob(fsys, pattern)
	if err != nil {
		return fmt.Errorf("glob: %w", err)
	}

	results := make(chan parseResult)
	var wg sync.WaitGroup

	for _, name := range files {
		wg.Add(1)
		go func(filename string) {
			defer wg.Done()

			f, err := fsys.Open(filename)
			if err != nil {
				results <- parseResult{err: fmt.Errorf("open %s: %w", filename, err)}
				return
			}
			defer f.Close()

			doc, err := parser.Parse(f)
			if err != nil {
				results <- parseResult{err: fmt.Errorf("parse %s: %w", filename, err)}
				return
			}

			results <- parseResult{filename: filename, doc: &doc}
		}(name)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var errs []error
	for res := range results {
		if res.err != nil {
			errs = append(errs, res.err)
			continue
		}

		if _, exists := c.Docs[res.doc.ID]; exists {
			errs = append(errs, fmt.Errorf("%w: %s in %s", ErrDuplicateID, res.doc.ID, res.filename))
			continue
		}

		c.Docs[res.doc.ID] = res.doc
	}

	return errors.Join(errs...)
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

func validTarget(docs map[string]*ast.Document, target string) bool {
	docID, secID, hasHash := strings.Cut(target, "#")
	doc, ok := docs[docID]
	if !ok {
		return false
	}
	if !hasHash {
		return true
	}
	for _, sec := range doc.Sections {
		if sec.ID == secID {
			return true
		}
	}
	return false
}

func walk(inlines []ast.Inline, src string, notes map[string]struct{}, docs map[string]*ast.Document, errs *[]error) {
	for _, el := range inlines {
		switch v := el.(type) {
		case ast.InternalLink:
			if !validTarget(docs, v.Target) {
				*errs = append(*errs, &BrokenLinkError{Source: src, Target: v.Target})
			}
			walk(v.Label, src, notes, docs, errs)
		case ast.ExternalLink:
			walk(v.Label, src, notes, docs, errs)
		case ast.Bold:
			walk(v.Elements, src, notes, docs, errs)
		case ast.Italic:
			walk(v.Elements, src, notes, docs, errs)
		case ast.FootnoteRef:
			if _, ok := notes[v.Target]; !ok {
				*errs = append(*errs, &BrokenNoteError{Source: src, Target: v.Target})
			}
		}
	}
}

// ValidateLinks asserts the integrity of all intra-corpus references.
func (c *Collection) ValidateLinks() []error {
	var errs []error
	for id, doc := range c.Docs {
		notes := collectNotes(doc)
		for _, sec := range doc.Sections {
			for _, block := range sec.Blocks {
				switch b := block.(type) {
				case ast.TextBlock:
					walk(b.Elements, id, notes, c.Docs, &errs)
				case ast.ImageBlock:
					walk(b.Caption, id, notes, c.Docs, &errs)
				case ast.NoteBlock:
					walk(b.Elements, id, notes, c.Docs, &errs)
				case ast.ListBlock:
					for _, item := range b.Items {
						walk(item, id, notes, c.Docs, &errs)
					}
				}
			}
		}
	}
	return errs
}

func (c *Collection) Get(id string) (*ast.Document, error) {
	if doc, ok := c.Docs[id]; ok {
		return doc, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
}
