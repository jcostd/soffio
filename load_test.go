// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile := func(name, content string) {
		dir := filepath.Dir(name)
		if dir != "." {
			os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		}
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	// an explicit id
	writeFile("doc1.soffio", "ID: explicit-id\nTitle: Doc 1\n\n== s1 | S1\nText")

	// no id: the file name, doc2
	writeFile("doc2.soffio", "Title: Doc 2\n\n== s1 | S1\nText")

	// no id, in a directory: sub/doc3
	writeFile("sub/doc3.soffio", "Title: Doc 3\n\n== s1 | S1\nText")

	// the id of doc1 again
	writeFile("dup.soffio", "ID: explicit-id\nTitle: Dup\n\n== s1 | S1\nText")

	// text outside any section
	writeFile("bad.soffio", "Title: Bad\n\nTesto senza sezione dichiarata")

	// under static/: skipped
	writeFile("static/ignored.soffio", "Title: Ignored\n\n== s1 | S1\nText")

	docs, err := Load(os.DirFS(tmpDir), "static")

	if err == nil {
		t.Fatal("expected Load to return errors for duplicates and bad syntax, got nil")
	}

	// files are read in lexical order: doc1 first, dup is the duplicate
	if !strings.Contains(err.Error(), "duplicate document ID: explicit-id in dup.soffio") {
		t.Errorf("expected duplicate ID error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "found block content outside any section") {
		t.Errorf("expected parser error, got: %v", err)
	}

	if docs["explicit-id"] == nil {
		t.Errorf("missing explicitly named doc 'explicit-id'")
	}
	if docs["doc2"] == nil {
		t.Errorf("missing fallback named doc 'doc2'")
	}
	if docs["sub/doc3"] == nil {
		t.Errorf("missing subfolder doc 'sub/doc3'")
	}
	if docs["static/ignored"] != nil {
		t.Errorf("document inside 'static' dir should have been skipped")
	}
}

func TestLoadCaseDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	for name, id := range map[string]string{"a.soffio": "Toro", "b.soffio": "toro"} {
		content := "id: " + id + "\ntitle: T\n\n== s1 | S1\nText"
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	_, err := Load(os.DirFS(tmpDir), "static")
	if err == nil || !strings.Contains(err.Error(), "only in case") {
		t.Errorf("expected a case-only duplicate ID error, got: %v", err)
	}
}
