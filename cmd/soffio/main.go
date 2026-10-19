// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

// Command soffio turns a directory of .soffio texts into a static site,
// or one text on standard input into HTML on standard output. It is
// package site, with nothing added.
package main

import "github.com/jcostd/soffio/v5/site"

func main() { site.Main(nil, nil) }
