// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"
)

var (
	// ErrNotFound is returned when a requested document ID does not exist in the collection.
	ErrNotFound = errors.New("document not found")
	// ErrDuplicateID is returned when two documents claim the same logical ID.
	ErrDuplicateID = errors.New("duplicate document ID")
)

// Collection holds the parsed documents mapped by their logical ID.
type Collection struct {
	Docs map[string]*Document
}

// New initializes an empty document collection.
func New() *Collection {
	return &Collection{
		Docs: make(map[string]*Document),
	}
}

type parseResult struct {
	filename string
	doc      *Document
	err      error
}

// Load concurrently decodes files from the filesystem.
// It ignores directories matching skipDir to prevent parsing static assets.
func (c *Collection) Load(fsys fs.FS, pattern, skipDir string) error {
	var files []string

	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == skipDir {
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

	sem := make(chan struct{}, 100)
	results := make(chan parseResult, len(files))
	var wg sync.WaitGroup

	for _, name := range files {
		wg.Add(1)
		go func(filename string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			f, err := fsys.Open(filename)
			if err != nil {
				results <- parseResult{err: fmt.Errorf("open %s: %w", filename, err)}
				return
			}
			defer f.Close()

			doc, err := Parse(f)
			if err != nil {
				results <- parseResult{err: fmt.Errorf("parse %s: %w", filename, err)}
				return
			}

			// Composite ID: prefix frontmatter ID with relative directory path.
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

	// IDs are output paths, and on case-insensitive filesystems (Windows,
	// macOS) Toro.html and toro.html are one file: IDs differing only in
	// case are duplicates too
	folded := make(map[string]string, len(files))

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
		key := strings.ToLower(res.doc.ID)
		if other, exists := folded[key]; exists {
			errs = append(errs, fmt.Errorf("%w: %s in %s differs from %s only in case", ErrDuplicateID, res.doc.ID, res.filename, other))
			continue
		}

		folded[key] = res.doc.ID
		c.Docs[res.doc.ID] = res.doc
	}

	return errors.Join(errs...)
}

// Get retrieves a document by its logical ID.
func (c *Collection) Get(id string) (*Document, error) {
	if doc, ok := c.Docs[id]; ok {
		return doc, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
}
