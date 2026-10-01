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

	// Funzione helper per creare file finti nella cartella temporanea
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

	// 1. File valido con ID esplicito nel frontmatter
	writeFile("doc1.soffio", "ID: explicit-id\nTitle: Doc 1\n\n== s1 | S1\nText")

	// 2. File valido senza ID (dovrebbe usare il nome del file 'doc2')
	writeFile("doc2.soffio", "Title: Doc 2\n\n== s1 | S1\nText")

	// 3. File in una sottocartella senza ID (dovrebbe diventare 'sub/doc3')
	writeFile("sub/doc3.soffio", "Title: Doc 3\n\n== s1 | S1\nText")

	// 4. File duplicato (usa l'ID esplicito già preso da doc1.txt)
	writeFile("dup.soffio", "ID: explicit-id\nTitle: Dup\n\n== s1 | S1\nText")

	// 5. File con sintassi non valida per scatenare un errore del parser
	writeFile("bad.soffio", "Title: Bad\n\nTesto senza sezione dichiarata")

	// 6. File dentro una cartella 'static' (dovrebbe essere ignorato da WalkDir)
	writeFile("static/ignored.soffio", "Title: Ignored\n\n== s1 | S1\nText")

	docs, err := Load(os.DirFS(tmpDir), "static")

	if err == nil {
		t.Fatal("expected Load to return errors for duplicates and bad syntax, got nil")
	}

	if !strings.Contains(err.Error(), "duplicate document ID") {
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

