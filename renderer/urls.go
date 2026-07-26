// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package renderer

import (
	"net/url"
	"path"
	"strings"
)

// resolveURL calculates the href for the browser using POSIX web rules.
func resolveURL(sourceID, target string) string {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "" || strings.HasPrefix(target, "//") {
		return target
	}

	targetPath, secID, hasHash := strings.Cut(target, "#")
	if targetPath == "" {
		if hasHash {
			return "#" + secID
		}
		return ""
	}

	ext := path.Ext(targetPath)
	if ext == "" {
		targetPath += ".html"
	}

	if !strings.HasPrefix(targetPath, "/") {
		targetPath = path.Join(path.Dir(sourceID), targetPath)
	} else {
		targetPath = strings.TrimPrefix(targetPath, "/")
	}

	sourceDir := path.Dir(sourceID)
	if sourceDir != "." && sourceDir != "" {
		baseParts := strings.Split(sourceDir, "/")
		targetParts := strings.Split(targetPath, "/")

		i := 0
		for i < len(baseParts) && i < len(targetParts) && baseParts[i] == targetParts[i] {
			i++
		}

		var out []string
		for j := i; j < len(baseParts); j++ {
			out = append(out, "..")
		}
		out = append(out, targetParts[i:]...)
		targetPath = strings.Join(out, "/")
	}

	if hasHash {
		targetPath += "#" + secID
	}

	return targetPath
}
