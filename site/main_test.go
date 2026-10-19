package site

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"image"
	imagepng "image/png"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jcostd/soffio/v5"
)

// TestMain lets the test binary be the command, for the tests that run
// it end to end: soffio, or with SOFFIO_MAIN=ext, a site's own, with a
// function and a check of its own.
func TestMain(m *testing.M) {
	switch os.Getenv("SOFFIO_MAIN") {
	case "1":
		Main(nil, nil)
		os.Exit(0)
	case "ext":
		Main(template.FuncMap{"shout": strings.ToUpper}, func(docs map[string]*soffio.Document) error {
			if docs["bad"] != nil {
				return errors.New("bad: a page this site refuses")
			}
			return nil
		})
		os.Exit(0)
	case "clash":
		Main(template.FuncMap{"doc": strings.ToUpper}, nil)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// soffioRun runs soffio with args in dir and returns its exit status
// and standard error.
func soffioRun(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	return run(t, "1", dir, args...)
}

// run runs the command of SOFFIO_MAIN=main with args in dir and returns
// its exit status and standard error. It is named as the test binary:
// site.
func run(t *testing.T, main, dir string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SOFFIO_MAIN="+main)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if e, ok := errors.AsType[*exec.ExitError](err); ok {
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
		"src/index.soffio":   "title: Home\n\n== s | S\n(about -> about)",
		"src/about.soffio":   "title: About\n\n== s | S\n:: img: /static/a.png | A",
		"src/draft.soffio":   "title: Draft\nvisibility: private\n\n== s | S\nx",
		"static/a.png":       "png",
		"broken/x.soffio":    "title: X\n\n== s | S\n(gone -> nope)",
		"leak/x.soffio":      "title: X\n\n== s | S\n(draft -> draft)",
		"leak/draft.soffio":  "title: D\nvisibility: private\n\n== s | S\nx",
		"e404/404.soffio":    "title: Not found\n\n== s | S\nx",
		"hlink/x.soffio":     "title: X\nby: -> nope\n\n== s | S\nx",
		"layout/x.soffio":    "title: X\nlayout: nope\n\n== s | S\nx",
		"badtmpl/x.html":     "{{ if }}",
		"badsite/x.html":     "x",
		"badsite/robots.txt": `{{ template "none" }}`,
		"named/layout.html":  "x",
		"named/404.html":     "{{ .ID }}",
		"named/sitemap.xml":  "{{ .ID }}",
	})

	if code, stderr := soffioRun(t, dir, "-baseurl", "https://example.org", "-s", "static", "src"); code != 0 {
		t.Fatalf("build: exit %d: %s", code, stderr)
	}
	for _, name := range []string{"index.html", "about.html", "static/a.png", "404.html", "robots.txt", "sitemap.xml"} {
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
	for file, want := range map[string]string{
		"public/index.html": `<link rel="canonical" href="https://example.org/index.html">`,
		"all/index.html":    `<li><a href="/about.html">About</a></li>`, // no -baseurl, from the root
		"public/about.html": `<h1>About</h1>`,
		"public/robots.txt": "Sitemap: https://example.org/sitemap.xml",
	} {
		if got, _ := os.ReadFile(filepath.Join(dir, file)); !strings.Contains(string(got), want) {
			t.Errorf("%s has no %s", file, want)
		}
	}
	// a site file's ID is its name, without its extension
	if code, stderr := soffioRun(t, dir, "-s", "static", "-t", "named", "-o", "named-out", "src"); code != 0 {
		t.Errorf("named: exit %d: %s", code, stderr)
	}
	for file, want := range map[string]string{"404.html": "404", "sitemap.xml": "sitemap"} {
		if got, _ := os.ReadFile(filepath.Join(dir, "named-out", file)); string(got) != want {
			t.Errorf("%s: %q, want %q", file, got, want)
		}
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
		{[]string{"-n", "-today", "17-10-2026", "src"}, 1, `-today "17-10-2026": want YYYY-MM-DD`},
		{[]string{"src", "more"}, 2, "usage: site [-a]"},
		{[]string{"-n", "-s", "static", "e404"}, 1, "page 404 is the site file 404.html too"},
		{[]string{"-n", "hlink"}, 1, filepath.Join("hlink", "x.soffio") + `:2: link to missing page "/nope"`},
		{[]string{"-n", "nope"}, 1, "nope: no such file or directory"},
		{[]string{"-n", "-t", "badtmpl", "src"}, 1, "site: template: x.html:1: missing value for if"},
		{[]string{"-n", "-t", "layout", "layout"}, 1, filepath.Join("layout", "x.soffio") + ": no layout nope.html in the templates"},
		{[]string{"-n", "-s", "static", "-t", "badsite", "src"}, 1, "site: robots.txt: "},
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

	// -n checks and prints nothing; a flag about the site is refused
	cmd = exec.Command(os.Args[0], "-n")
	cmd.Env = append(os.Environ(), "SOFFIO_MAIN=1")
	cmd.Stdin = strings.NewReader("title: T\n\n== s | S\nx")
	if out, err := cmd.Output(); err != nil || len(out) != 0 {
		t.Errorf("pipe -n: %v: %q", err, out)
	}
	if code, stderr := soffioRun(t, t.TempDir(), "-o", "x"); code != 2 || !strings.Contains(stderr, "site: -o needs a dir") {
		t.Errorf("pipe -o: exit %d, %q", code, stderr)
	}

	cmd = exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "SOFFIO_MAIN=1")
	cmd.Stdin = strings.NewReader("title: T\n\n== s | S\nx(*n)")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil || !strings.Contains(stderr.String(), `<stdin>:4: note "n" is not defined`) {
		t.Errorf("pipe with a missing note: %v: %s", err, stderr.String())
	}

	// what can't be written is said, not lost
	ro, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	cmd = exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "SOFFIO_MAIN=1")
	cmd.Stdin = strings.NewReader("title: T\n\n== s | S\nx")
	cmd.Stdout = ro
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil || !strings.Contains(stderr.String(), "site: write /dev/stdout: bad file descriptor") {
		t.Errorf("pipe to a file it can't write: %v: %s", err, stderr.String())
	}
}

func testSite(t *testing.T, layouts string, docs map[string]*soffio.Document) *site {
	t.Helper()
	s := &site{
		baseURL: "https://example.org",
		outDir:  t.TempDir(),
		docs:    docs,
		ids:     slices.Sorted(maps.Keys(docs)),
	}
	s.tmpl = template.Must(template.New("").Funcs(s.funcs()).Parse(layouts))
	s.prepare()
	return s
}

func page(id string, meta map[string]string) *soffio.Document {
	m := map[string]string{"title": "T " + id}
	maps.Copy(m, meta)
	return &soffio.Document{ID: id, Meta: m, Sections: []soffio.Section{
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
	s := testSite(t, `{{define "layout.html"}}<title>{{.Meta.title}}</title>{{.Content}}{{end}}`+
		`{{define "post.html"}}post {{.Meta.title}}{{end}}`, docs)

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

	// a page that can't be written fails
	file := filepath.Join(s.outDir, "a.html")
	s.outDir = file
	if err := s.writeDoc(docs["a"]); err == nil {
		t.Error("writing under a file: no error")
	}

	s.dry = true
	s.outDir = filepath.Join(filepath.Dir(file), "dry")
	if err := s.writeDoc(docs["a"]); err != nil || exists(s.outDir, ".") {
		t.Errorf("dry run: %v, wrote %v", err, exists(s.outDir, "."))
	}
}

// a page gets its ID and every page, by ID
func TestWriteDocDocs(t *testing.T) {
	docs := map[string]*soffio.Document{"it/a": page("it/a", nil), "it/b": page("it/b", nil)}
	s := testSite(t, `{{define "layout.html"}}{{.ID}}|{{range .Docs}}{{.ID}};{{end}}{{end}}`, docs)
	if err := s.writeDoc(docs["it/b"]); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(s.outDir, "it/b.html")); string(got) != "it/b|it/a;it/b;" {
		t.Errorf("got %q", got)
	}
}

func TestCopyDir(t *testing.T) {
	src := writeFiles(t, map[string]string{"file.txt": "hello", "css/deep/style.css": "body {}", ".DS_Store": "", ".git/x": ""})
	dst := filepath.Join(t.TempDir(), "static")
	if err := copyDir([]string{src}, dst, false); err != nil {
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
	if err := copyDir([]string{src}, dst, false); err != nil {
		t.Errorf("copy again: %v", err)
	}

	// a dry run writes nothing
	dry := filepath.Join(t.TempDir(), "static")
	if err := copyDir([]string{src}, dry, true); err != nil || exists(dry, ".") {
		t.Errorf("dry run: %v, wrote %v", err, exists(dry, "."))
	}

	// the same tree, or dst inside src, there or not yet: a file would
	// be copied onto itself; a dry run says so too
	for _, d := range []string{src, filepath.Join(src, "css"), filepath.Join(src, "public", "static")} {
		for _, dry := range []bool{true, false} {
			if err := copyDir([]string{src}, d, dry); err == nil || !strings.Contains(err.Error(), "copied onto itself") {
				t.Errorf("copyDir(%s, %s, %v): %v", src, d, dry, err)
			}
		}
	}
	if got, _ := os.ReadFile(filepath.Join(src, "file.txt")); string(got) != "hello" {
		t.Errorf("file.txt is %q now", got)
	}

	if err := os.Symlink(filepath.Join(src, "css"), filepath.Join(src, "loop")); err != nil {
		t.Skip("no symlinks here:", err)
	}
	for _, dry := range []bool{true, false} {
		if err := copyDir([]string{src}, t.TempDir(), dry); err == nil || !strings.Contains(err.Error(), "a link to a directory") {
			t.Errorf("link to a directory, dry %v: %v", dry, err)
		}
	}
	// a broken link says where it is
	loop := filepath.Join(src, "loop")
	if err := os.Remove(loop); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(src, "nope"), loop); err != nil {
		t.Fatal(err)
	}
	if err := copyDir([]string{src}, t.TempDir(), true); err == nil || !strings.Contains(err.Error(), loop) {
		t.Errorf("broken link: %v", err)
	}
}

func TestCopyDirs(t *testing.T) {
	a := writeFiles(t, map[string]string{"css/s.css": "a", "img/x.png": "a"})
	b := writeFiles(t, map[string]string{"img/y.png": "b"})
	dst := filepath.Join(t.TempDir(), "static")
	if err := copyDir([]string{a, b}, dst, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"css/s.css", "img/x.png", "img/y.png"} {
		if !exists(dst, name) {
			t.Errorf("%s missing", name)
		}
	}

	// one address, two files: in two -s, or in letter case only
	c := writeFiles(t, map[string]string{"img/X.png": "c"})
	for _, srcs := range [][]string{{a, a}, {a, c}} {
		for _, dry := range []bool{true, false} {
			err := copyDir(srcs, t.TempDir(), dry)
			if err == nil || !strings.Contains(err.Error(), "are both /static/") {
				t.Errorf("copyDir(%v, dry %v): %v", srcs, dry, err)
			}
		}
	}
}

func TestCheckFlags(t *testing.T) {
	static := writeFiles(t, map[string]string{"f": ""})
	file := filepath.Join(static, "f")
	tests := []struct {
		baseURL, static string
		ok              bool
	}{
		{"https://example.org", static, true},
		{"https://example.org/sub", "", true},
		{"http://localhost:8080", "", true},
		{"", "", true},
		{"https://example.org/", "", false},
		{"/", "", false},
		{"example.org", "", false},
		{"ftp://example.org", "", false},
		{"https://example.org/?a=1", "", false},
		{"https://example.org/c++", "", false},
		{"https://example.org", filepath.Join(static, "nope"), false},
		{"https://example.org", file, false},
	}
	for _, tt := range tests {
		if err := checkFlags(tt.baseURL, "", []string{tt.static}, "", "public"); (err == nil) != tt.ok {
			t.Errorf("checkFlags(%q, %q) = %v", tt.baseURL, tt.static, err)
		}
	}
	for tmpl, want := range map[string]string{
		static:                        "",
		filepath.Join(static, "nope"): "no such file",
		file:                          "-t " + file + ": not a directory",
	} {
		if err := checkFlags("https://example.org", "", nil, tmpl, "public"); want == "" && err != nil || !strings.Contains(fmt.Sprint(err), want) {
			t.Errorf("checkFlags -t %s: %v, want %q", tmpl, err, want)
		}
	}
}

// The files only drafts use live in a -s of their own, which only the
// preview gets: the public site has none of them, and a public text
// pointing at one is a missing file, as a link to a draft is an error.
func TestSoffioPrivateStatic(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"src/a.soffio":     "title: A\n\n== s | S\n:: img: /static/a.png | a",
		"src/draft.soffio": "title: D\nvisibility: private\n\n== s | S\n:: img: /static/d.png | d",
		"leak/a.soffio":    "title: A\n\n== s | S\n:: img: /static/d.png | d",
		"static/a.png":     "A",
		"drafts/d.png":     "D",
	})
	if code, stderr := soffioRun(t, dir, "-baseurl", "https://example.org", "-s", "static", "src"); code != 0 || exists(dir, "public/static/d.png") {
		t.Errorf("public: exit %d, d.png there: %v: %s", code, exists(dir, "public/static/d.png"), stderr)
	}
	if code, stderr := soffioRun(t, dir, "-a", "-s", "static", "-s", "drafts", "-o", "www", "src"); code != 0 ||
		!exists(dir, "www/static/a.png") || !exists(dir, "www/static/d.png") {
		t.Errorf("preview: exit %d: %s", code, stderr)
	}
	if code, stderr := soffioRun(t, dir, "-n", "-s", "static", "leak"); code != 1 || !strings.Contains(stderr, `missing file "/static/d.png"`) {
		t.Errorf("public text, draft file: exit %d, %q", code, stderr)
	}
	if code, stderr := soffioRun(t, dir, "-n", "-s", "", "src"); code != 2 || !strings.Contains(stderr, "no directory") {
		t.Errorf("-s \"\": exit %d, %q", code, stderr)
	}
}

func TestSoffioTemplatesOntoThemselves(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"src/a.soffio":     "title: A\n\n== s | S\nx",
		"site/layout.html": "{{ .Content }}",
		"site/robots.txt":  "ROBOTS",
	})
	// site and ./site/ are one directory
	code, stderr := soffioRun(t, dir, "-t", "site", "-o", "./site/", "src")
	got, _ := os.ReadFile(filepath.Join(dir, "site", "robots.txt"))
	if code != 1 || !strings.Contains(stderr, "written over its templates") || string(got) != "ROBOTS" {
		t.Errorf("exit %d, %q, robots.txt %q", code, stderr, got)
	}
}

func TestSoffioStaticOntoItself(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"src/a.soffio":        "title: A\n\n== s | S\n:: img: /static/a.png | a",
		"public/static/a.png": "PNG",
	})
	// -n says what the build would: -o inside -s, not there yet, too
	for _, args := range [][]string{
		{"-baseurl", "https://example.org", "-s", "public/static", "-o", "public", "src"},
		{"-n", "-s", "public/static", "-o", "public", "src"},
		{"-n", "-s", "public/static", "-o", "public/static/site", "src"},
	} {
		code, stderr := soffioRun(t, dir, args...)
		got, _ := os.ReadFile(filepath.Join(dir, "public", "static", "a.png"))
		if code != 1 || !strings.Contains(stderr, "would be copied onto itself") || string(got) != "PNG" {
			t.Errorf("soffio %v: exit %d, %q, a.png %q", args, code, stderr, got)
		}
	}
	if exists(dir, "public/static/site") {
		t.Error("-n made public/static/site")
	}
}

func TestWriteDocToday(t *testing.T) {
	// the templates know the day only if told, so the same flags make the
	// same site
	docs := map[string]*soffio.Document{"a": page("a", nil)}
	s := testSite(t, `{{define "layout.html"}}[{{today}}]{{end}}`, docs)
	for _, today := range []string{"", "2026-10-17"} {
		s.today = today
		if err := s.writeDoc(docs["a"]); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(filepath.Join(s.outDir, "a.html")); string(got) != "["+today+"]" {
			t.Errorf("-today %q: got %q", today, got)
		}
	}
}

func TestWriteDocDoc(t *testing.T) {
	// doc finds a page by ID: a partial needs no more than the page it
	// is given
	docs := map[string]*soffio.Document{"it/a": page("it/a", nil), "en/c": page("en/c", nil)}
	s := testSite(t, `{{define "layout.html"}}{{with doc "en/c"}}{{.Meta.title}}{{end}}|{{with doc "nope"}}?{{end}}{{end}}`, docs)
	if err := s.writeDoc(docs["it/a"]); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(s.outDir, "it/a.html")); string(got) != "T en/c|" {
		t.Errorf("got %q", got)
	}
}

// a site's own command has the functions it adds and refuses what its
// check refuses; it can't take a function of soffio's
func TestMainExtended(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"src/a.soffio":     "title: A\n\n== s | S\nx",
		"bad/bad.soffio":   "title: B\n\n== s | S\nx",
		"tmpl/layout.html": "{{ shout .Meta.title }}",
	})
	if code, stderr := run(t, "ext", dir, "-t", "tmpl", "src"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "public", "a.html")); string(got) != "A" {
		t.Errorf("a.html: %q", got)
	}
	if code, stderr := run(t, "ext", dir, "-n", "-t", "tmpl", "bad"); code != 1 || !strings.Contains(stderr, "bad: a page this site refuses") {
		t.Errorf("check: exit %d, %q", code, stderr)
	}
	if code, stderr := run(t, "clash", dir, "-n", "src"); code != 1 || !strings.Contains(stderr, "site: the templates' function doc is soffio's") {
		t.Errorf("clash: exit %d, %q", code, stderr)
	}
}

// an image gets its size from its file, in the text and through size:
// none for a file Go can't read or an address out of the site
func TestWriteDocSizes(t *testing.T) {
	var png bytes.Buffer
	if err := imagepng.Encode(&png, image.NewGray(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	static := writeFiles(t, map[string]string{"a.png": png.String(), "b.svg": "<svg/>"})
	img := func(path string) soffio.Section {
		return soffio.Section{Level: 2, ID: path[len(path)-5 : len(path)-4], Title: "S", Blocks: []soffio.Block{soffio.ImageBlock{Path: path}}}
	}
	docs := map[string]*soffio.Document{"x": {ID: "x", Meta: map[string]string{"title": "X", "image": "/static/a.png"},
		Sections: []soffio.Section{img("/static/a.png"), img("/static/b.svg"), img("https://example.org/c.png")}}}
	s := testSite(t, `{{define "layout.html"}}{{with size .Meta.image}}{{.Width}}x{{.Height}}{{end}}|{{with size "/static/b.svg"}}?{{end}}|{{.Content}}{{end}}`, docs)
	s.static = []string{static}
	s.prepare()
	if err := s.writeDoc(docs["x"]); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(s.outDir, "x.html"))
	for _, want := range []string{"3x2||", `src="static/a.png" alt="" width="3" height="2" loading="lazy"`,
		`src="static/b.svg" alt="" loading="lazy"`, `src="https://example.org/c.png" alt="" loading="lazy"`} {
		if !strings.Contains(string(got), want) {
			t.Errorf("no %q in %q", want, got)
		}
	}
}

// what can't be copied fails the copy: a destination under a file, a
// source that is not there, a file that can't be read
func TestCopyDirErrors(t *testing.T) {
	src := writeFiles(t, map[string]string{"a.txt": "a", "file": ""})
	if err := copyDir([]string{src}, filepath.Join(src, "file", "static"), false); err == nil {
		t.Error("a destination under a file: no error")
	}
	dst := filepath.Join(t.TempDir(), "static")
	if err := copyDir([]string{filepath.Join(src, "nope")}, dst, false); err == nil {
		t.Error("a missing source: no error")
	}
	if os.Getuid() == 0 {
		t.Skip("root reads everything")
	}
	if err := os.Chmod(filepath.Join(src, "a.txt"), 0); err != nil {
		t.Fatal(err)
	}
	if err := copyDir([]string{src}, dst, false); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("an unreadable file: %v", err)
	}
}
