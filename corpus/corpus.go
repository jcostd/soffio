// Package corpus manages a collection of parsed soffio documents.
package corpus

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"soffio/ast"
	"soffio/parser"
)

var linkRX = regexp.MustCompile(`\[\[(.*?)->(.*?)\]\]`)

var (
	ErrNotFound    = errors.New("document not found")
	ErrDuplicateID = errors.New("duplicate document ID")
)

// BrokenLinkError records a link that points to a non-existent document or section.
type BrokenLinkError struct {
	Source string
	Target string
}

func (e *BrokenLinkError) Error() string {
	return fmt.Sprintf("broken link: '%s' points to missing target '%s'", e.Source, e.Target)
}

// Collection holds parsed documents indexed by their ID.
type Collection struct {
	Docs map[string]*ast.Document
}

// New returns a newly allocated, empty Collection.
func New() *Collection {
	return &Collection{
		Docs: make(map[string]*ast.Document),
	}
}

// Load parses all documents matching pattern in fsys and adds them to the collection.
func (c *Collection) Load(fsys fs.FS, pattern string) error {
	files, err := fs.Glob(fsys, pattern)
	if err != nil {
		return fmt.Errorf("glob: %w", err)
	}

	var errs []error
	for _, name := range files {
		f, err := fsys.Open(name)
		if err != nil {
			errs = append(errs, fmt.Errorf("open %s: %w", name, err))
			continue
		}

		doc, err := parser.Parse(f)
		f.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("parse %s: %w", name, err))
			continue
		}

		if _, exists := c.Docs[doc.ID]; exists {
			errs = append(errs, fmt.Errorf("%w: %s in %s", ErrDuplicateID, doc.ID, name))
			continue
		}

		c.Docs[doc.ID] = &doc
	}

	return errors.Join(errs...)
}

// ValidateLinks checks all documents for broken internal links.
func (c *Collection) ValidateLinks() []error {
	var errs []error

	for id, doc := range c.Docs {
		for _, sec := range doc.Sections {
			for _, block := range sec.Blocks {
				text, ok := block.(ast.TextBlock)
				if !ok {
					continue
				}

				for _, match := range linkRX.FindAllStringSubmatch(text.Content, -1) {
					target := strings.TrimSpace(match[2])
					if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
						continue
					}

					if !c.validTarget(target) {
						errs = append(errs, &BrokenLinkError{
							Source: id,
							Target: target,
						})
					}
				}
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

// Get returns the document with the given ID.
func (c *Collection) Get(id string) (*ast.Document, error) {
	if doc, ok := c.Docs[id]; ok {
		return doc, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
}
