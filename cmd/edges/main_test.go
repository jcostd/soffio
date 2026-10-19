package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"soffio"
)

// a page is its ID and layout; a link, its page, the header key or "-"
// for the body, and the page it leads to, once: a page the header names
// is not named again for the body
func TestWrite(t *testing.T) {
	dir := t.TempDir()
	for name, src := range map[string]string{
		"it/a.soffio": "title: A\nlayout: work\nby: -> p\n\n== s | S\n\n(P -> p), (B -> b#s), (A -> a), (F -> /static/x.png), (W -> https://x.org)",
		"it/b.soffio": "title: B\n\n== s | S\n\nText.",
		"it/p.soffio": "title: P\nlayout: person\n\n== s | S\n\nText.",
	} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	docs, err := soffio.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, id := range []string{"it/a", "it/b", "it/p"} {
		write(&b, docs[id])
	}
	want := "it/a work\nit/a by it/p\nit/a - it/b\nit/b -\nit/p person\n"
	if b.String() != want {
		t.Errorf("got %q, want %q", b.String(), want)
	}
}
