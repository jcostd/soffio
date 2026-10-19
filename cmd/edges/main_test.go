// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"soffio"
)

// TestMain lets the test binary be edges itself, for the tests that run
// it end to end.
func TestMain(m *testing.M) {
	if os.Getenv("EDGES_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// edges runs edges with args, stdout to out, and returns its exit status,
// standard output and standard error.
func edges(t *testing.T, out *os.File, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "EDGES_MAIN=1")
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if out != nil {
		cmd.Stdout = out
	}
	err := cmd.Run()
	if e, ok := errors.AsType[*exec.ExitError](err); ok {
		return e.ExitCode(), stdout.String(), stderr.String()
	}
	if err != nil {
		t.Fatal(err)
	}
	return 0, stdout.String(), stderr.String()
}

func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// a page is its ID and layout; a link, its page, the header key or "-"
// for the body, and the page it leads to, once: a page the header names
// is not named again for the body
func TestWrite(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"it/a.soffio": "title: A\nlayout: work\nby: -> p\n\n== s | S\n\n(P -> p), (B -> b#s), (A -> a), (F -> /static/x.png), (W -> https://x.org)",
		"it/b.soffio": "title: B\n\n== s | S\n\nText.",
		"it/p.soffio": "title: P\nlayout: person\n\n== s | S\n\nText.",
	})
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

// the pages come by ID, the private ones only with -a; edges says what it
// can't read or write, and how it is used
func TestEdges(t *testing.T) {
	site := writeFiles(t, map[string]string{
		"b.soffio": "title: B\n\n== s | S\n(A -> a)",
		"a.soffio": "title: A\n\n== s | S\nx",
		"d.soffio": "title: D\nvisibility: private\n\n== s | S\nx",
	})
	bad := writeFiles(t, map[string]string{"x.soffio": "title: X\n\nx"})
	ro, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	for _, tt := range []struct {
		out          *os.File
		args         []string
		code         int
		stdout, want string
	}{
		{nil, []string{site}, 0, "a -\nb -\nb - a\n", ""},
		{nil, []string{"-a", site}, 0, "a -\nb -\nb - a\nd -\n", ""},
		{nil, []string{bad}, 1, "", "x.soffio:3: text before the first section"},
		{nil, nil, 2, "", "usage: edges [-a] dir"},
		{ro, []string{site}, 1, "", "edges: write /dev/stdout: bad file descriptor"},
	} {
		code, stdout, stderr := edges(t, tt.out, tt.args...)
		if code != tt.code || stdout != tt.stdout || !strings.Contains(stderr, tt.want) {
			t.Errorf("edges %v: exit %d, %q, %q; want exit %d, %q, %q", tt.args, code, stdout, stderr, tt.code, tt.stdout, tt.want)
		}
	}
}
