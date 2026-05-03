// Package corpus manages a collection of parsed soffio documents.
package corpus

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

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

type Collection struct {
	Docs map[string]*ast.Document
}

func New() *Collection {
	return &Collection{
		Docs: make(map[string]*ast.Document),
	}
}

func (c *Collection) Load(fsys fs.FS, pattern string) error {
	files, err := fs.Glob(fsys, pattern)
	if err != nil {
		return fmt.Errorf("glob: %w", err)
	}

	var errs []error
	for _, name := range files {
		if err := c.loadFile(fsys, name); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (c *Collection) loadFile(fsys fs.FS, name string) error {
	f, err := fsys.Open(name)
	if err != nil {
		return fmt.Errorf("open %s: %w", name, err)
	}
	defer f.Close()

	doc, err := parser.Parse(f)
	if err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}

	if _, exists := c.Docs[doc.ID]; exists {
		return fmt.Errorf("%w: %s in %s", ErrDuplicateID, doc.ID, name)
	}

	c.Docs[doc.ID] = &doc
	return nil
}

func (c *Collection) ValidateLinks() []error {
	var errs []error

	for id, doc := range c.Docs {
		for _, sec := range doc.Sections {
			for _, block := range sec.Blocks {

				var walk func(inlines []ast.Inline)
				walk = func(inlines []ast.Inline) {
					for _, el := range inlines {
						switch v := el.(type) {
						case ast.InternalLink:
							if !c.validTarget(v.Target) {
								errs = append(errs, &BrokenLinkError{
									Source: id,
									Target: v.Target,
								})
							}
							walk(v.Label)
						case ast.ExternalLink:
							walk(v.Label)
						case ast.Bold:
							walk(v.Elements)
						case ast.Italic:
							walk(v.Elements)
						}
					}
				}

				walk(block.Inlines())
			}
		}
	}

	return errs
}

func (c *Collection) validTarget(target string) bool {
	docID, secID, hasHash := strings.Cut(target, "#")
	doc, ok := c.Docs[docID]
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

func (c *Collection) Get(id string) (*ast.Document, error) {
	if doc, ok := c.Docs[id]; ok {
		return doc, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
}
