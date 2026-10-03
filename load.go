// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Load parses every .soffio file under dir, but hidden ones, into
// documents by ID: the id header, or else the file name, under the
// file's directory, as in it/about. Files are read in lexical order, so the errors come in the
// same order every time.
func Load(dir string) (map[string]*Document, error) {
	docs := map[string]*Document{}
	// IDs are output paths, and on case-insensitive filesystems (Windows,
	// macOS) Toro.html and toro.html are one file: IDs differing only in
	// case are duplicates too
	folded := map[string]*Document{}
	var errs []error

	if fi, err := os.Stat(dir); err != nil {
		return nil, err
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("%s: not a directory", dir)
	}
	fsys := os.DirFS(dir)
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		file := filepath.Join(dir, filepath.FromSlash(name))
		switch {
		case err != nil:
			return err
		case name != "." && strings.HasPrefix(d.Name(), "."):
			// hidden, as for ls: .git, and the ._ files macOS leaves
			// on other disks
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		case d.Type()&fs.ModeSymlink != 0:
			// WalkDir would not go in; it may loop
			if fi, err := fs.Stat(fsys, name); err == nil && fi.IsDir() {
				errs = append(errs, fmt.Errorf("%s: a link to a directory", file))
				return nil
			}
		}
		if d.IsDir() || path.Ext(name) != ".soffio" {
			return nil
		}
		doc, err := parseFile(fsys, name, file)
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		key := strings.ToLower(doc.ID)
		switch other := folded[key]; {
		case other == nil:
			folded[key] = doc
			docs[doc.ID] = doc
		case other.ID == doc.ID:
			errs = append(errs, fmt.Errorf("%s: duplicate id %q, also in %s", doc.File, doc.ID, other.File))
		default:
			errs = append(errs, fmt.Errorf("%s: id %q differs from %q in %s only in case", doc.File, doc.ID, other.ID, other.File))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return docs, errors.Join(errs...)
}

// parseFile parses name in fsys, known to the user as file.
func parseFile(fsys fs.FS, name, file string) (*Document, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	doc, err := Parse(file, f)
	if err != nil {
		return nil, err
	}
	// a page with no title is an empty link in every list of pages
	if doc.Title == "" {
		return nil, fmt.Errorf("%s: no title", file)
	}
	if doc.ID == "" {
		doc.ID = strings.TrimSuffix(path.Base(name), ".soffio")
	}
	doc.ID = path.Join(path.Dir(name), doc.ID)
	// the directories and the file name are part of the ID too
	for part := range strings.SplitSeq(doc.ID, "/") {
		if why := CheckID(part); why != "" {
			return nil, fmt.Errorf("%s: invalid id %q: %s", file, doc.ID, why)
		}
	}
	// static/ is where the static files go, in any case on Windows
	// and macOS
	if strings.HasPrefix(strings.ToLower(doc.ID), "static/") {
		return nil, fmt.Errorf("%s: id %q: static/ is for static files", file, doc.ID)
	}
	return doc, nil
}
