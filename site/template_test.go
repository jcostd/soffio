package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTemplates(t *testing.T) {
	builtin, err := loadTemplates("", (&site{}).funcs())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"layout.html", "404.html", "sitemap.xml", "robots.txt"} {
		if builtin.Lookup(name) == nil {
			t.Errorf("built-in %s missing", name)
		}
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "layout.html"), []byte(`mine`), 0o644); err != nil {
		t.Fatal(err)
	}
	local, err := loadTemplates(dir, (&site{}).funcs())
	if err != nil {
		t.Fatal(err)
	}
	if local.Lookup("layout.html") == nil {
		t.Error("layout.html of dir missing")
	}
	// the built-in ones are not mixed in
	if local.Lookup("robots.txt") != nil {
		t.Error("built-in robots.txt mixed into dir's templates")
	}

}

// a key a header doesn't hold is "" to the templates, so a function
// given it gets a string
func TestLoadTemplatesMissingKey(t *testing.T) {
	dir := writeFiles(t, map[string]string{"layout.html": `[{{.Meta.by}}]{{with doc .Meta.by}}?{{end}}`})
	tmpl, err := loadTemplates(dir, (&site{}).funcs())
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	data := map[string]any{"Meta": map[string]string{}}
	if err := tmpl.ExecuteTemplate(&b, "layout.html", data); err != nil || b.String() != "[]" {
		t.Errorf("got %q, %v", b.String(), err)
	}
}
