// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package main

import "os/exec"

func openBrowser(url string) error {
	return exec.Command("open", url).Start()
}
