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

// the functions that take pages apart: sortUp, where and without with
// more values, except, group, bySize, parent, current
func TestSortUpGroup(t *testing.T) {
	docs := map[string]*soffio.Document{}
	for id, meta := range map[string]string{
		"a": "1990 testimony", "b": "1955 testimony", "c": " book", "d": "1990 book",
		"e": " friend", "f": " artist", "g": " artist", "h": " ", "w": "1960 work",
	} {
		when, kind, _ := strings.Cut(meta, " ")
		m := map[string]string{}
		if when != "" {
			m["when"] = when
		}
		if kind != "" && kind != "work" {
			m["kind"] = kind
		}
		if kind == "work" {
			m["layout"] = "work"
		}
		docs[id] = &soffio.Document{ID: id, Meta: m}
	}
	ids := func(docs []*soffio.Document) string {
		var s []string
		for _, d := range docs {
			s = append(s, d.ID)
		}
		return strings.Join(s, ",")
	}
	groups := func(gs [][]*soffio.Document) string {
		var s []string
		for _, g := range gs {
			s = append(s, ids(g))
		}
		return strings.Join(s, "|")
	}
	must := func(docs []*soffio.Document, err error) []*soffio.Document {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return docs
	}
	up := must(sortUp(docs, "when"))
	g, err := group(up, "kind")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ got, want string }{
		// oldest first, one year by ID, the undated last, by ID
		{ids(up), "b,w,a,d,c,e,f,g,h"},
		{ids(must(where(docs, "kind", "book", "friend"))), "c,d,e"},
		{ids(must(without(docs, "kind", "artist", ""))), "a,b,c,d,e"},
		{ids(must(except(docs, "a", "w", "nope"))), "b,c,d,e,f,g,h"},
		{ids(must(except(docs))), "a,b,c,d,e,f,g,h,w"},
		// by the first page of each, as the list runs; h and w have no kind
		{groups(g), "b,a|d,c|e|f,g"},
		{groups(bySize(g)), "e|b,a|d,c|f,g"},
		{ids(must(current(docs, "when", "1990-06-01"))), "a,d"},
		{ids(must(current(docs, "when", ""))), "a,b,d,w"},
	} {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
	if g, err := group("x", "kind"); g != nil || err == nil {
		t.Error("group took a string")
	}
	for id, want := range map[string]string{"it/persone/arp": "it/persone", "it": "", "": ""} {
		if got := parent(id); got != want {
			t.Errorf("parent(%q) = %q, want %q", id, got, want)
		}
	}
}

// a key a header doesn't hold is "" to the templates, so a function
// given it gets a string
func TestLoadTemplatesMissingKey(t *testing.T) {
	dir := writeFiles(t, map[string]string{"layout.html": `[{{.Meta.by}}]{{len (except .Docs .Meta.by)}}`})
	tmpl, err := loadTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	data := map[string]any{"Meta": map[string]string{}, "Docs": []*soffio.Document{{ID: "a"}}}
	if err := tmpl.ExecuteTemplate(&b, "layout.html", data); err != nil || b.String() != "[]1" {
		t.Errorf("got %q, %v", b.String(), err)
	}
}
