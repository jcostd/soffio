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
		// one file per path, so two IDs differ at least in case; that is
		// one file on Windows and macOS
		key := strings.ToLower(doc.ID)
		if other := folded[key]; other != nil {
			errs = append(errs, fmt.Errorf("%s: id %q differs from %q in %s only in case", doc.File, doc.ID, other.ID, other.File))
			return nil
		}
		folded[key] = doc
		docs[doc.ID] = doc
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
	doc.ID = strings.TrimSuffix(name, ".soffio")
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
	// a link in the header names a page, as one in the body does, from
	// the text's directory; the templates get the page's ID. Whether it
	// is there is for Check to say, as for every link
	var errs []error
	for _, key := range doc.links {
		id, frag, ok := resolve(doc.ID, doc.Meta[key])
		why := ""
		switch {
		case !ok:
			why = "is not a page of the site"
		case frag != "":
			why = "points at a section: a header link points at a page"
		case strings.HasPrefix(id, "static/"):
			why = "is a file: a header link points at a page"
		}
		if why != "" {
			errs = append(errs, fmt.Errorf("%s:%d: %s %q %s", file, doc.lines[key], key, doc.Meta[key], why))
			continue
		}
		doc.Meta[key] = id
	}
	if errs != nil {
		return nil, errors.Join(errs...)
	}
	return doc, nil
}
