// Package corpus indexes and validates Soffio documents.
package corpus

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
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
	Line   int
	Target string
}

type BrokenNoteError struct {
	Source string
	Line   int
	Target string
}

func (e *BrokenLinkError) Error() string {
	return fmt.Sprintf("%s:%d: [block start] broken link points to missing target '%s'", e.Source, e.Line, e.Target)
}

func (e *BrokenNoteError) Error() string {
	return fmt.Sprintf("%s:%d: [block start] broken note ref points to missing note '%s'", e.Source, e.Line, e.Target)
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

// Load concurrently decodes files. The filesystem path scopes the document ID.
func (c *Collection) Load(fsys fs.FS, pattern string) error {
	var files []string

	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "static" {
			return fs.SkipDir
		}
		if !d.IsDir() {
			matched, mErr := path.Match(pattern, d.Name())
			if mErr == nil && matched {
				files = append(files, p)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk: %w", err)
	}

	results := make(chan parseResult, len(files))
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

			// Composite ID: prefix frontmatter ID with relative directory path.
			// Fallback to filename if ID is absent.
			if doc.ID == "" {
				doc.ID = strings.TrimSuffix(path.Base(filename), path.Ext(filename))
			}
			relDir := path.Dir(filename)
			if relDir != "." {
				doc.ID = path.Join(relDir, doc.ID)
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

// validTarget resolves absolute and relative references within the corpus.
func validTarget(docs map[string]*ast.Document, sourceID, target string) bool {
	docID, secID, hasHash := strings.Cut(target, "#")

	// Check absolute path first.
	doc, ok := docs[docID]
	if !ok {
		// Fallback to relative path resolution.
		relPath := path.Join(path.Dir(sourceID), docID)
		doc, ok = docs[relPath]
		if !ok {
			return false
		}
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

func walk(inlines []ast.Inline, sourceID string, line int, notes map[string]struct{}, docs map[string]*ast.Document, errs *[]error) {
	for _, el := range inlines {
		switch v := el.(type) {
		case ast.InternalLink:
			if !validTarget(docs, sourceID, v.Target) {
				*errs = append(*errs, &BrokenLinkError{Source: sourceID, Line: line, Target: v.Target})
			}
			walk(v.Label, sourceID, line, notes, docs, errs)
		case ast.ExternalLink:
			walk(v.Label, sourceID, line, notes, docs, errs)
		case ast.Bold:
			walk(v.Elements, sourceID, line, notes, docs, errs)
		case ast.Italic:
			walk(v.Elements, sourceID, line, notes, docs, errs)
		case ast.FootnoteRef:
			if _, ok := notes[v.Target]; !ok {
				*errs = append(*errs, &BrokenNoteError{Source: sourceID, Line: line, Target: v.Target})
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
					walk(b.Elements, id, b.Line, notes, c.Docs, &errs)
				case ast.ImageBlock:
					walk(b.Caption, id, b.Line, notes, c.Docs, &errs)
				case ast.NoteBlock:
					walk(b.Elements, id, b.Line, notes, c.Docs, &errs)
				case ast.ListBlock:
					for _, item := range b.Items {
						walk(item, id, b.Line, notes, c.Docs, &errs)
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
