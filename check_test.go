package soffio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckTarget(t *testing.T) {
	all := map[string]*Document{
		"it/home":        {ID: "it/home", Sections: []Section{{ID: "intro"}}},
		"it/about":       {ID: "it/about", Sections: []Section{{ID: "team"}}},
		"en/home":        {ID: "en/home", Sections: []Section{{ID: "intro"}}},
		"private/secret": {ID: "private/secret", Sections: []Section{{ID: "data"}}},
	}
	// the public view: private/secret is left out
	active := map[string]*Document{
		"it/home":  all["it/home"],
		"it/about": all["it/about"],
		"en/home":  all["en/home"],
	}
	static := t.TempDir()
	if err := os.MkdirAll(filepath.Join(static, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"a.pdf", ".b.pdf"} {
		if err := os.WriteFile(filepath.Join(static, "docs", f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		from, target, want string
	}{
		{"it/home", "/it/about", ""},
		{"it/home", "about", ""},
		{"it/home", "about#team", ""},
		{"it/home", "/it/home#intro", ""},
		{"it/home", "#intro", ""},
		{"it/home", "https://example.org/x", ""},
		{"it/home", "/static/docs/a.pdf", ""},
		{"it/home", "../static/docs/a.pdf#page=2", ""},
		{"en/home", "about", "link to missing page"},
		{"it/home", "privacy", "link to missing page"},
		{"it/home", "#nope", "link to missing section"},
		{"it/home", "about#", "link to missing section"},
		{"it/home", "/private/secret", "link to private page"},
		{"it/home", "/static/docs/b.pdf", "missing file"},
		{"it/home", "/static/docs", "missing file"},
		// there, but never copied
		{"it/home", "/static/docs/.b.pdf", "hidden file"},
		{"it/home", "/it/x/../about", ""},
		{"it/home", "/static/../static/docs/a.pdf", ""},
		{"it/home", "/static/../../etc/passwd", "link to missing page"},
	}
	for _, tt := range tests {
		if got := checkTarget(all, active, static, tt.from, tt.target); got != tt.want {
			t.Errorf("checkTarget(%q, %q) = %q, want %q", tt.from, tt.target, got, tt.want)
		}
	}
	if got := checkTarget(all, active, "", "it/home", "/static/docs/a.pdf"); got != "missing file" {
		t.Errorf("a file with no static dir: %q, want missing file", got)
	}
}

// parse parses src as file, failing the test on an error.
func parse(t *testing.T, file, id, src string) *Document {
	t.Helper()
	doc, err := Parse(file, strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	doc.ID = id
	return doc
}

func TestCheck(t *testing.T) {
	a := parse(t, "a.soffio", "a", `
== s | S
(b -> b) (broken -> nope) (secret -> secret)(*n1)(*n9)

- (*n2) in a list

:: img: /static/missing.png | gone

:: note: n1 | one, and (*n2)

:: note: n2 | two

:: note: n3 | never used`)
	b := parse(t, "b.soffio", "b", "\n== s | S\nx")
	secret := parse(t, "secret.soffio", "secret", "\n== s | S\nx")
	all := map[string]*Document{"a": a, "b": b, "secret": secret}
	active := map[string]*Document{"a": a, "b": b}

	err := Check(all, active, t.TempDir())
	want := `a.soffio:3: link to missing page "nope"
a.soffio:3: link to private page "secret"
a.soffio:3: note "n9" is not defined
a.soffio:7: missing file "/static/missing.png"
a.soffio:13: note "n3" is never referenced`
	if err == nil || err.Error() != want {
		t.Errorf("got:\n%v\nwant:\n%s", err, want)
	}
	if err := Check(all, all, t.TempDir()); err == nil || strings.Contains(err.Error(), "private") {
		t.Errorf("with every text active, no link is private: %v", err)
	}
}

func TestCheckNotesAndAddresses(t *testing.T) {
	doc := parse(t, "a.soffio", "a", `
== s | S
(*a) (bad -> nope%zz)

:: note: a | see (*b)

:: note: b | B

:: note: self | only (*self)

:: note: c | (*d)

:: note: d | only from c`)
	// b is reached through a; self, c and d never from the text
	want := `a.soffio:3: invalid address "nope%zz"
a.soffio:9: note "self" is never referenced
a.soffio:11: note "c" is never referenced
a.soffio:13: note "d" is never referenced`
	if err := CheckDoc(doc); err == nil || err.Error() != want {
		t.Errorf("got:\n%v\nwant:\n%s", err, want)
	}
}

func TestCheckDoc(t *testing.T) {
	doc := parse(t, "<stdin>", "", `
== s | S
(up -> #s) (gone -> #nope) (other -> other-page) (*n)(*m)

:: img: /static/x.png | not checked alone

:: note: n | n

:: note: u | unused`)
	want := `<stdin>:3: link to missing section "#nope"
<stdin>:3: note "m" is not defined
<stdin>:9: note "u" is never referenced`
	if err := CheckDoc(doc); err == nil || err.Error() != want {
		t.Errorf("got:\n%v\nwant:\n%s", err, want)
	}
}

func BenchmarkCheck(b *testing.B) {
	doc := &Document{
		ID: "bench",
		Sections: []Section{{ID: "s1", Blocks: []Block{
			TextBlock{Elements: []Inline{Link{Target: "bench#s1", Label: []Inline{PlainText{Content: "Self ref"}}}}},
		}}},
	}
	docs := map[string]*Document{"bench": doc}
	b.ReportAllocs()
	for b.Loop() {
		_ = Check(docs, docs, "static")
	}
}
