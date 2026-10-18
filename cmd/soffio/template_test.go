package main

import (
	"os"
	"path/filepath"
	"strings"
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
	sorted, err := sortBy(docs, "event_date")
	if err != nil {
		t.Fatal(err)
	}

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

func TestWhereReverse(t *testing.T) {
	docs := map[string]*soffio.Document{
		"c": {ID: "c", Meta: map[string]string{"layout": "opera", "artist": "it/persone/arp"}},
		"a": {ID: "a", Meta: map[string]string{"layout": "opera"}},
		"b": {ID: "b", Meta: map[string]string{"layout": "evento"}},
	}
	ids := func(docs []*soffio.Document, err error) string {
		if err != nil {
			return err.Error()
		}
		var s []string
		for _, d := range docs {
			s = append(s, d.ID)
		}
		return strings.Join(s, ",")
	}
	sorted, _ := sortBy(docs, "layout")
	// it/ is a namespace: it itself and itx/ are not under it
	langs := map[string]*soffio.Document{}
	for _, id := range []string{"it", "it/a", "it/b/c", "itx/a", "en/a"} {
		langs[id] = &soffio.Document{ID: id, Meta: map[string]string{}}
	}
	for _, tt := range []struct{ got, want string }{
		// a map, as .Docs, comes by ID
		{ids(where(docs, "layout", "opera")), "a,c"},
		{ids(where(docs, "artist", "it/persone/arp")), "c"},
		{ids(where(docs, "layout", "")), ""},
		{ids(where(nil, "layout", "opera")), ""},
		{ids(reverse(docs)), "c,b,a"},
		{ids(sorted, nil), "a,c,b"},
		{ids(reverse(sorted)), "b,c,a"},
		{ids(where("x", "layout", "opera")), "string is not a list of pages"},
		{ids(under(langs, "it")), "it/a,it/b/c"},
		{ids(under(langs, "it/b")), "it/b/c"},
		{ids(under(langs, "")), "en/a,it,it/a,it/b/c,itx/a"},
	} {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

func TestRFC822(t *testing.T) {
	if got, err := rfc822("2026-08-19"); err != nil || got != "Wed, 19 Aug 2026 00:00:00 +0000" {
		t.Errorf("rfc822 = %q, %v", got, err)
	}
	if got, err := rfc822("1954"); err != nil || got != "Fri, 01 Jan 1954 00:00:00 +0000" {
		t.Errorf("rfc822(1954) = %q, %v", got, err)
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
	for _, name := range []string{"layout.html", "404.html", "rss.xml", "sitemap.xml", "robots.txt"} {
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

func TestPast(t *testing.T) {
	for _, tt := range []struct {
		date, today string
		want        bool
	}{
		{"2027-03-29", "2027-03-29", false},
		{"2027-03-29", "2027-03-30", true},
		{"2027-03", "2027-03-31", false},
		{"2027-03", "2027-04-01", true},
		{"2027", "2027-12-31", false},
		{"2026", "2027-01-01", true},
		{"2027-03-29", "", false},
		{"", "2027-01-01", false},
	} {
		if got := past(tt.date, tt.today); got != tt.want {
			t.Errorf("past(%q, %q) = %v", tt.date, tt.today, got)
		}
	}
}
