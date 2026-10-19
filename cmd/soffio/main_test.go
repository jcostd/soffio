// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestMain lets the test binary be soffio itself.
func TestMain(m *testing.M) {
	if os.Getenv("SOFFIO_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// soffio is package site with nothing added: a text in, HTML out
func TestSoffio(t *testing.T) {
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "SOFFIO_MAIN=1")
	cmd.Stdin = strings.NewReader("title: T\n\n== s | S\n*hi*")
	if out, err := cmd.Output(); err != nil || string(out) != "<section id=\"s\">\n<h2>S</h2>\n<p><strong>hi</strong></p>\n</section>\n" {
		t.Errorf("%v: %q", err, out)
	}
}
