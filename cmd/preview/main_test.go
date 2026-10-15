// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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
