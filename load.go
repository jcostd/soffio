// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// Load parses every .soffio file in fsys, skipping directories named
// skipDir, into documents by ID: the id header, or else the file name,
// under the file's directory, as in it/about. Files are read in
// lexical order, so the errors come in the same order every time.
func Load(fsys fs.FS, skipDir string) (map[string]*Document, error) {
	docs := map[string]*Document{}
	// IDs are output paths, and on case-insensitive filesystems (Windows,
	// macOS) Toro.html and toro.html are one file: IDs differing only in
	// case are duplicates too
	folded := map[string]string{}
	var errs []error

	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == skipDir:
			return fs.SkipDir
		case d.IsDir() || path.Ext(name) != ".soffio":
			return nil
		}

		doc, err := parseFile(fsys, name)
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		if _, ok := docs[doc.ID]; ok {
			errs = append(errs, fmt.Errorf("duplicate document ID: %s in %s", doc.ID, name))
			return nil
		}
		key := strings.ToLower(doc.ID)
		if other, ok := folded[key]; ok {
			errs = append(errs, fmt.Errorf("duplicate document ID: %s in %s differs from %s only in case", doc.ID, name, other))
			return nil
		}
		folded[key] = doc.ID
		docs[doc.ID] = doc
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}
	return docs, errors.Join(errs...)
}

func parseFile(fsys fs.FS, name string) (*Document, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	defer f.Close()

	doc, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	if doc.ID == "" {
		doc.ID = strings.TrimSuffix(path.Base(name), ".soffio")
	}
	doc.ID = path.Join(path.Dir(name), doc.ID)
	return &doc, nil
}
