package main

import (
	"os"
	"path/filepath"
	"testing"

	"soffio"
)

func TestSortBy(t *testing.T) {
	docs := []*soffio.Document{
		{ID: "a", Meta: map[string]string{"event_date": "1964-01-01"}},
		{ID: "b", Meta: map[string]string{"event_date": "2020-05-10"}},
		{ID: "c", Meta: map[string]string{"event_date": "1964-01-01"}},
		{ID: "d", Meta: map[string]string{}},
	}

	sorted := sortBy(docs, "event_date")

	// highest first, then by ID; no date last
	expected := []string{"b", "a", "c", "d"}
	for i, id := range expected {
		if sorted[i].ID != id {
			t.Errorf("index %d: want %s, got %s", i, id, sorted[i].ID)
		}
	}

	// docs itself is not sorted
	if docs[0].ID != "a" {
		t.Error("sortBy mutated original slice")
	}
}

func TestLoadTemplates(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "layout.html")

	if err := os.WriteFile(file, []byte(`{{define "layout.html"}}override{{end}}`), 0644); err != nil {
		t.Fatalf("write temp template: %v", err)
	}

	overrideTmpl, err := loadTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}

	if overrideTmpl.Lookup("layout.html") == nil {
		t.Error("failed to load local template override")
	}
}
