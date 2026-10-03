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
	for i, id := range []string{"b", "a", "c", "d"} {
		if sorted[i].ID != id {
			t.Errorf("index %d: want %s, got %s", i, id, sorted[i].ID)
		}
	}
	// docs itself is not sorted
	if docs[0].ID != "a" {
		t.Error("sortBy changed docs")
	}
}

func TestRFC822(t *testing.T) {
	if got, err := rfc822("2026-08-19"); err != nil || got != "Wed, 19 Aug 2026 00:00:00 +0000" {
		t.Errorf("rfc822 = %q, %v", got, err)
	}
	if _, err := rfc822("19/08/2026"); err == nil {
		t.Error("rfc822 took a date that is not YYYY-MM-DD")
	}
}

func TestLoadTemplates(t *testing.T) {
	builtin, err := loadTemplates("")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range append([]string{"layout.html"}, siteFiles...) {
		if builtin.Lookup(name) == nil {
			t.Errorf("built-in %s missing", name)
		}
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "layout.html"), []byte(`mine`), 0o644); err != nil {
		t.Fatal(err)
	}
	local, err := loadTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	if local.Lookup("layout.html") == nil {
		t.Error("layout.html of dir missing")
	}
	// the built-in ones are not mixed in
	if local.Lookup("rss.xml") != nil {
		t.Error("built-in rss.xml mixed into dir's templates")
	}

}
