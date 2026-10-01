package main

import (
	"html/template"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"soffio"
)

// TestMain lets the test binary be soffio itself, for the tests that
// run it end to end.
func TestMain(m *testing.M) {
	if os.Getenv("SOFFIO_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// soffioRun runs soffio with args in dir and returns its exit status
// and standard error.
func soffioRun(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SOFFIO_MAIN=1")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok {
		return e.ExitCode(), stderr.String()
	}
	if err != nil {
		t.Fatal(err)
	}
	return 0, stderr.String()
}

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

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
	return err == nil
}

func TestSoffio(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"src/index.soffio":  "title: Home\n\n== s | S\n(about -> about)",
		"src/about.soffio":  "title: About\n\n== s | S\n:: img: /static/a.png | A",
		"src/draft.soffio":  "title: Draft\nvisibility: private\n\n== s | S\nx",
		"static/a.png":      "png",
		"broken/x.soffio":   "title: X\n\n== s | S\n(gone -> nope)",
		"leak/x.soffio":     "title: X\n\n== s | S\n(draft -> draft)",
		"leak/draft.soffio": "title: D\nvisibility: private\n\n== s | S\nx",
		"e404/404.soffio":   "title: Not found\n\n== s | S\nx",
	})

	if code, stderr := soffioRun(t, dir, "-s", "static", "src"); code != 0 {
		t.Fatalf("build: exit %d: %s", code, stderr)
	}
	for _, name := range []string{"index.html", "about.html", "static/a.png", "rss.xml", "404.html"} {
		if !exists(dir, "public/"+name) {
			t.Errorf("public/%s missing", name)
		}
	}
	if exists(dir, "public/draft.html") {
		t.Error("a private text published without -a")
	}

	if code, stderr := soffioRun(t, dir, "-a", "-s", "static", "-o", "all", "src"); code != 0 || !exists(dir, "all/draft.html") {
		t.Errorf("-a: exit %d, draft.html there: %v: %s", code, exists(dir, "all/draft.html"), stderr)
	}
	if code, stderr := soffioRun(t, dir, "-n", "-s", "static", "-o", "none", "src"); code != 0 || exists(dir, "none") {
		t.Errorf("-n: exit %d, none/ there: %v: %s", code, exists(dir, "none"), stderr)
	}

	for _, tt := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"-n", "broken"}, 1, filepath.Join("broken", "x.soffio") + `:4: link to missing page "nope"`},
		{[]string{"-n", "leak"}, 1, `link to private page "draft"`},
		{[]string{"-n", "src"}, 1, `missing file "/static/a.png"`}, // no -s
		{[]string{"-n", "-t", "nope", "src"}, 1, "nope"},
		{[]string{"-n", "-baseurl", "https://x.org/", "src"}, 1, `drop the final '/'`},
		{[]string{"-n", "-langs", "it, en", "src"}, 1, `language " en": it contains a space`},
		{[]string{"src", "more"}, 2, "usage: soffio"},
		{[]string{"-n", "-s", "static", "e404"}, 1, "page 404 is the site file 404.html too"},
	} {
		code, stderr := soffioRun(t, dir, tt.args...)
		if code != tt.code || !strings.Contains(stderr, tt.want) {
			t.Errorf("soffio %v: exit %d, %q; want exit %d, %q", tt.args, code, stderr, tt.code, tt.want)
		}
	}
}

func TestSoffioPipe(t *testing.T) {
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "SOFFIO_MAIN=1")
	cmd.Stdin = strings.NewReader("title: T\n\n== s | S\n*hi*(*n)\n\n:: note: n | note")
	out, err := cmd.Output()
	if err != nil || !strings.Contains(string(out), "<strong>hi</strong>") {
		t.Errorf("pipe: %v: %s", err, out)
	}

	cmd = exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "SOFFIO_MAIN=1")
	cmd.Stdin = strings.NewReader("title: T\n\n== s | S\nx(*n)")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil || !strings.Contains(stderr.String(), `<stdin>:4: note "n" is not defined`) {
		t.Errorf("pipe with a missing note: %v: %s", err, stderr.String())
	}
}

func testSite(t *testing.T, layouts string, docs map[string]*soffio.Document) *site {
	t.Helper()
	return &site{
		baseURL: "https://example.org",
		langs:   []string{"it", "en"},
		outDir:  t.TempDir(),
		tmpl:    template.Must(template.New("").Parse(layouts)),
		docs:    docs,
		ids:     slices.Sorted(maps.Keys(docs)),
	}
}

func page(id string, meta map[string]string) *soffio.Document {
	return &soffio.Document{ID: id, Title: "T " + id, Meta: meta, Sections: []soffio.Section{
		{Level: 2, ID: "s", Title: "S", Blocks: []soffio.Block{
			soffio.TextBlock{Elements: []soffio.Inline{soffio.PlainText{Content: "Hello"}}},
		}},
	}}
}

func TestWriteDoc(t *testing.T) {
	docs := map[string]*soffio.Document{
		"a": page("a", nil),
		"b": page("b", map[string]string{"layout": "post"}),
		"c": page("c", map[string]string{"layout": "postt"}),
	}
	s := testSite(t, `{{define "layout.html"}}<title>{{.Title}}</title>{{.Content}}{{end}}`+
		`{{define "post.html"}}post {{.Title}}{{end}}`, docs)

	for id, want := range map[string]string{
		"a": "<title>T a</title><section id=\"s\">\n<h2>S</h2>\n<p>Hello</p>\n</section>\n",
		"b": "post T b",
	} {
		if err := s.writeDoc(docs[id]); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(filepath.Join(s.outDir, id+".html")); string(got) != want {
			t.Errorf("%s: got %q, want %q", id, got, want)
		}
	}
	if err := s.writeDoc(docs["c"]); err == nil || err.Error() != "no layout postt.html in the templates" {
		t.Errorf("unknown layout: %v", err)
	}

	s.dry = true
	s.outDir = filepath.Join(s.outDir, "dry")
	if err := s.writeDoc(docs["a"]); err != nil || exists(s.outDir, ".") {
		t.Errorf("dry run: %v, wrote %v", err, exists(s.outDir, "."))
	}
}

func TestWriteDocAlternatesChildren(t *testing.T) {
	docs := map[string]*soffio.Document{}
	for _, id := range []string{"en/x", "it/x", "it/x/c", "it/x/a", "it/x/b", "blog/x"} {
		docs[id] = page(id, nil)
	}
	s := testSite(t, `{{define "layout.html"}}{{range .Alternates}}{{.Lang}}={{.URL}};{{end}}|{{range .Children}}{{.ID}};{{end}}{{end}}`, docs)

	tests := []struct{ id, want string }{
		{"it/x", "it=https://example.org/it/x.html;en=https://example.org/en/x.html;|it/x/a;it/x/b;it/x/c;"},
		// blog is no language: no alternates in en/ or it/
		{"blog/x", "|"},
	}
	for _, tt := range tests {
		if err := s.writeDoc(docs[tt.id]); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(s.outDir, tt.id+".html"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tt.want {
			t.Errorf("%s: got %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestCopyDir(t *testing.T) {
	src := writeFiles(t, map[string]string{"file.txt": "hello", "css/deep/style.css": "body {}", ".DS_Store": "", ".git/x": ""})
	dst := filepath.Join(t.TempDir(), "static")
	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"file.txt": "hello", "css/deep/style.css": "body {}"} {
		if got, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(name))); err != nil || string(got) != want {
			t.Errorf("%s: %q, %v", name, got, err)
		}
	}
	if exists(dst, ".DS_Store") || exists(dst, ".git") {
		t.Error("hidden files copied")
	}
	// a second build copies over the first
	if err := copyDir(src, dst); err != nil {
		t.Errorf("copy again: %v", err)
	}

	// the same tree, or dst inside src: a file would be copied onto itself
	for _, d := range []string{src, filepath.Join(src, "css")} {
		if err := copyDir(src, d); err == nil || !strings.Contains(err.Error(), "copied onto itself") {
			t.Errorf("copyDir(%s, %s): %v", src, d, err)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(src, "file.txt")); string(got) != "hello" {
		t.Errorf("file.txt is %q now", got)
	}

	if err := os.Symlink(filepath.Join(src, "css"), filepath.Join(src, "loop")); err != nil {
		t.Skip("no symlinks here:", err)
	}
	if err := copyDir(src, t.TempDir()); err == nil || !strings.Contains(err.Error(), "a link to a directory") {
		t.Errorf("link to a directory: %v", err)
	}
}

func TestCheckFlags(t *testing.T) {
	static := writeFiles(t, map[string]string{"f": ""})
	file := filepath.Join(static, "f")
	tests := []struct {
		baseURL, langs, static string
		ok                     bool
	}{
		{"https://example.org", "it,en", static, true},
		{"https://example.org/sub", "en", "", true},
		{"http://localhost:8080", "en", "", true},
		{"", "en", "", true},
		{"https://example.org/", "en", "", false},
		{"/", "en", "", false},
		{"example.org", "en", "", false},
		{"ftp://example.org", "en", "", false},
		{"https://example.org/?a=1", "en", "", false},
		{"https://example.org/c++", "en", "", false},
		{"https://example.org", "it,", "", false},
		{"https://example.org", "it/x", "", false},
		{"https://example.org", "it,en,it", "", false},
		{"https://example.org", "en", filepath.Join(static, "nope"), false},
		{"https://example.org", "en", file, false},
	}
	for _, tt := range tests {
		if err := checkFlags(tt.baseURL, tt.langs, tt.static); (err == nil) != tt.ok {
			t.Errorf("checkFlags(%q, %q, %q) = %v", tt.baseURL, tt.langs, tt.static, err)
		}
	}
}

func TestSoffioStaticOntoItself(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"src/a.soffio":        "title: A\n\n== s | S\n:: img: /static/a.png | a",
		"public/static/a.png": "PNG",
	})
	code, stderr := soffioRun(t, dir, "-s", "public/static", "-o", "public", "src")
	got, _ := os.ReadFile(filepath.Join(dir, "public", "static", "a.png"))
	if code != 1 || !strings.Contains(stderr, "would be copied onto itself") || string(got) != "PNG" {
		t.Errorf("exit %d, %q, a.png %q", code, stderr, got)
	}
}
