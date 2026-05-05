//go:build windows

package main

import "os/exec"

func openBrowser(url string) {
	exec.Command("cmd", "/c", "start", url).Start()
}
