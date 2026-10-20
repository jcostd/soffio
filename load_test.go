// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeFiles makes the files, name to content, under a new directory.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const body = "\n\n== s1 | S1\nText"

func TestLoad(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"doc1.soffio":     "title: Doc 1" + body,
		"doc2.soffio":     "title: Doc 2" + body,
		"sub/doc3.soffio": "title: Doc 3" + body,
		"notes.txt":       "not a text",
		// hidden: skipped
		".draft.soffio": "broken",
		"._doc2.soffio": "\x00\x05\x16\x07",
		".git/x.soffio": "broken",
	})
	docs, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for id, file := range map[string]string{
		"doc1":     "doc1.soffio",
		"doc2":     "doc2.soffio",
		"sub/doc3": "sub/doc3.soffio",
	} {
		if docs[id] == nil || docs[id].File != filepath.Join(dir, filepath.FromSlash(file)) {
			t.Errorf("docs[%q] = %+v, want it from %s", id, docs[id], file)
		}
	}
	if len(docs) != 3 {
		t.Errorf("%d docs, want 3", len(docs))
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		files map[string]string
		want  string
	}{
		// files are read in lexical order: T before t
		{map[string]string{"Toro.soffio": "title: A" + body, "toro.soffio": "title: B" + body},
			`toro.soffio: id "toro" differs from "Toro" in `},
		{map[string]string{"my file.soffio": "title: x" + body},
			`my file.soffio: invalid id "my file": it contains a space`},
		{map[string]string{"città/x.soffio": "title: x" + body},
			`invalid id "città/x": 'à' is not a plain ASCII character`},
		{map[string]string{"static/x.soffio": "title: x" + body},
			`id "static/x": static/ is for static files`},
		{map[string]string{"Static/x.soffio": "title: x" + body},
			`id "Static/x": static/ is for static files`},
		{map[string]string{"a.soffio": "layout: a" + body}, `a.soffio: no title`},
		{map[string]string{"a.soffio": "title:  " + body}, `a.soffio: no title`},
		{map[string]string{"bad.soffio": "title: x\n\ntext"},
			`bad.soffio:3: text before the first section`},
		{map[string]string{"a.soffio": "title: A\nby: -> https://example.org/x" + body},
			`a.soffio:2: by "https://example.org/x" is not a page of the site`},
		{map[string]string{"a.soffio": "title: A\nby: -> b#s" + body},
			`a.soffio:2: by "b#s" points at a section: a header link points at a page`},
		{map[string]string{"a.soffio": "title: A\nby: -> /static/x.pdf" + body},
			`a.soffio:2: by "/static/x.pdf" is a file: a header link points at a page`},
	}
	for _, tt := range tests {
		_, err := Load(writeFiles(t, tt.files))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Load(%v) = %v, want %q", tt.files, err, tt.want)
		}
	}
}

func TestLoadLinkToDir(t *testing.T) {
	dir := writeFiles(t, map[string]string{"real/a.soffio": "title: A" + body})
	if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link")); err != nil {
		t.Skip("no symlinks here:", err)
	}
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "link: a link to a directory") {
		t.Errorf("a link to a directory: %v", err)
	}
}

func TestLoadMissingDir(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("Load of a missing directory: no error")
	}
	dir := writeFiles(t, map[string]string{"a.soffio": "title: A" + body})
	file := filepath.Join(dir, "a.soffio")
	if _, err := Load(file); err == nil || err.Error() != file+": not a directory" {
		t.Errorf("Load of a file: %v", err)
	}
}

// a header link is resolved as a body link is, from the text's
// directory, and becomes the page's ID; Links are the pages it names
func TestLoadHeaderLinks(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"it/opere/centauro.soffio": "title: Centauro\nartist: -> ../persone/picasso\nplace: -> /it/luoghi/murano\nsame: -> stella" + body,
	})
	docs, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"artist": "it/persone/picasso", "place": "it/luoghi/murano", "same": "it/opere/stella"} {
		if got := docs["it/opere/centauro"].Meta[key]; got != want {
			t.Errorf("Meta[%q] = %q, want %q", key, got, want)
		}
	}
	if got, want := docs["it/opere/centauro"].Links, []string{"it/luoghi/murano", "it/opere/stella", "it/persone/picasso"}; !slices.Equal(got, want) {
		t.Errorf("Links = %q, want %q", got, want)
	}
}

// what can't be read fails the load, and says where
func TestLoadUnreadable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads everything")
	}
	for name, want := range map[string]string{"locked": "locked: permission denied", "locked.soffio": "locked.soffio: permission denied"} {
		dir := writeFiles(t, map[string]string{"locked/a.soffio": "title: A" + body, "locked.soffio": "title: L" + body})
		path := filepath.Join(dir, name)
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(path, 0o755)
		if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Load with %s unreadable: %v", name, err)
		}
	}
}
