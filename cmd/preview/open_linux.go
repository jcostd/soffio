// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux

package main

import "os/exec"

func openBrowser(url string) error {
	return exec.Command("xdg-open", url).Start()
}
