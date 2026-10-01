// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"net/http"
	"net/http/httptest"
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
