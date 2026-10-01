// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio

import (
	"net/url"
	"path"
	"strings"
)

// resolve splits the target of a link in the document from into the ID
// or static file it points to and its #fragment, if any. A target
// without a leading '/' is relative to from's directory; either is
// cleaned, so .. can't climb out of the site. ok is false
// for a link out of the site. Check and href both resolve through here,
// so what is checked is what is linked.
func resolve(from, target string) (id, frag string, ok bool) {
	if u, err := url.Parse(target); err != nil || u.Scheme != "" || strings.HasPrefix(target, "//") {
		return "", "", false
	}
	p, sec, hash := strings.Cut(target, "#")
	if hash {
		frag = "#" + sec
	}
	switch {
	case p == "":
		id = from
	case strings.HasPrefix(p, "/"):
		id = path.Clean(p)[1:]
	default:
		id = path.Join(path.Dir(from), p)
	}
	return id, frag, true
}

// href is the address of target seen from the page from. It is
// relative, so the site works under any base.
func href(from, target string) string {
	id, frag, ok := resolve(from, target)
	switch {
	case !ok:
		return target
	case strings.HasPrefix(target, "#"):
		return target
	}
	// a file under static/ is an asset; anything else is a page, even
	// an ID with a dot in it
	if !strings.HasPrefix(id, "static/") {
		id += ".html"
	}
	return relative(path.Dir(from), id) + frag
}

// relative is the path to to from the directory dir, both from the root.
func relative(dir, to string) string {
	if dir == "." {
		return to
	}
	a, b := strings.Split(dir, "/"), strings.Split(to, "/")
	i := 0
	for i < len(a) && i < len(b)-1 && a[i] == b[i] {
		i++
	}
	return strings.Repeat("../", len(a)-i) + strings.Join(b[i:], "/")
}
