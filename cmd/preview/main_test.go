// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestMain lets the test binary be preview itself, for the tests that
// run it end to end; to serve, it serves until its standard input
// closes.
func TestMain(m *testing.M) {
	switch os.Getenv("PREVIEW_MAIN") {
	case "serve":
		go func() {
			io.Copy(io.Discard, os.Stdin)
			os.Exit(0)
		}()
		fallthrough
	case "1":
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// preview runs preview with args and the commands in path, and returns
// its exit status and standard error.
func preview(t *testing.T, path string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "PREVIEW_MAIN=1", "PATH="+path)
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

// browser is a directory holding an xdg-open that opens nothing.
func browser(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "xdg-open"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// serve serves h on a port of its own, until the test ends, and returns
// the port.
func serve(t *testing.T, h http.Handler) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go http.Serve(l, h)
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

// preview serves a directory, or finds it served already, and opens the
// browser on it; it says why it can't
func TestPreview(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "index.html")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ours, theirs := serve(t, handler(dir)), serve(t, http.NotFoundHandler())
	for _, tt := range []struct {
		path string
		args []string
		code int
		want string
	}{
		{browser(t), []string{"a", "b"}, 2, "usage: preview"},
		{browser(t), []string{"-d", filepath.Join(dir, "nope")}, 1, "nope: no such file or directory"},
		{browser(t), []string{file}, 1, "index.html: not a directory"},
		{browser(t), []string{"-p", ours, dir}, 0, "preview: already serving " + dir},
		{t.TempDir(), []string{"-p", ours, "-d", dir}, 0, `preview: exec: "xdg-open": executable file not found in $PATH`},
		{browser(t), []string{"-p", theirs, dir}, 1, "address already in use"},
	} {
		code, stderr := preview(t, tt.path, tt.args...)
		if code != tt.code || !strings.Contains(stderr, tt.want) {
			t.Errorf("preview %v: exit %d, %q; want exit %d, %q", tt.args, code, stderr, tt.code, tt.want)
		}
	}
}

// a directory is named in full, to know it again: where there is none,
// preview says so
func TestPreviewNoWorkingDir(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone")
	if err := os.Mkdir(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(gone)
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	if code, stderr := preview(t, browser(t), "public"); code != 1 || !strings.Contains(stderr, "preview: getwd: ") {
		t.Errorf("exit %d, %q", code, stderr)
	}
}

// preview serves dir, labelled with it, until it is stopped
func TestPreviewServe(t *testing.T) {
	dir := t.TempDir()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	l.Close()

	cmd := exec.Command(os.Args[0], "-p", port, dir)
	cmd.Env = append(os.Environ(), "PREVIEW_MAIN=serve", "PATH="+browser(t))
	stop, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	url := "http://127.0.0.1:" + port
	for i := 0; !servesDir(url, dir); i++ {
		if i == 100 {
			t.Fatal("preview never served")
		}
		time.Sleep(20 * time.Millisecond)
	}
	stop.Close()
	if err := cmd.Wait(); err != nil || !strings.Contains(stderr.String(), "preview: serving "+dir+" at "+url) {
		t.Errorf("%v: %s", err, stderr.String())
	}
}

func TestServesDir(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()

	srv := httptest.NewServer(handler(dir))
	defer srv.Close()

	if !servesDir(srv.URL, dir) {
		t.Errorf("servesDir(%q) = false, want true for the served dir", dir)
	}
	if servesDir(srv.URL, other) {
		t.Errorf("servesDir(%q) = true, want false for another dir", other)
	}
}

func TestServesDirForeignServer(t *testing.T) {
	dir := t.TempDir()

	srv := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer srv.Close()

	if servesDir(srv.URL, dir) {
		t.Error("servesDir = true for a server that is not a preview")
	}
}

func TestServesDirNoServer(t *testing.T) {
	srv := httptest.NewServer(handler(t.TempDir()))
	url := srv.URL
	srv.Close()

	if servesDir(url, t.TempDir()) {
		t.Error("servesDir = true with nothing listening")
	}
}

func TestHeaders(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"index.html", "style.css", "font.woff2", "pic.WEBP", "notes.unknown"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewServer(handler(dir))
	defer srv.Close()

	for path, want := range map[string]string{
		"/":              "text/html; charset=utf-8",
		"/style.css":     "text/css; charset=utf-8",
		"/font.woff2":    "font/woff2",
		"/pic.WEBP":      "image/webp",
		"/notes.unknown": "",
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status %d", path, resp.StatusCode)
		}
		if got := resp.Header.Get("Content-Type"); want != "" && got != want {
			t.Errorf("%s: Content-Type %q, want %q", path, got, want)
		}
		for k, v := range map[string]string{
			"Cache-Control":          "no-store",
			"X-Content-Type-Options": "nosniff",
			"Referrer-Policy":        "no-referrer",
		} {
			if got := resp.Header.Get(k); got != v {
				t.Errorf("%s: %s %q, want %q", path, k, got, v)
			}
		}
	}

	// a missing file is a plain-text 404, not an empty page typed as HTML
	resp, err := http.Get(srv.URL + "/missing.html")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound || resp.Header.Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("missing: %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}
