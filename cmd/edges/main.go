// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

// Command edges prints the pages of a site and the links between them,
// a line each, read as soffio reads them, for the tools that read
// lines: a site's own rules of what links to what are a few lines of
// awk on it, not a second reader of the texts.
package main

import (
	"bufio"
	"cmp"
	"flag"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"slices"

	"soffio"
)

func main() {
	log.SetFlags(0)
	all := flag.Bool("a", false, "all texts: the private ones too")
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), "usage: edges [-a] dir\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	docs, err := soffio.Load(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	w := bufio.NewWriter(os.Stdout)
	for _, id := range slices.Sorted(maps.Keys(docs)) {
		if *all || docs[id].Meta["visibility"] != "private" {
			write(w, docs[id])
		}
	}
	if err := w.Flush(); err != nil {
		log.Fatalf("edges: %v", err)
	}
}

// write prints the line of doc, its ID and its layout, "-" if none;
// then one for each page it links to: its ID, the header key that holds
// the link, or "-" for the body, and the ID of that page. A page linked
// from the header and the body is a link of the header.
func write(w io.Writer, doc *soffio.Document) {
	fmt.Fprintln(w, doc.ID, cmp.Or(doc.Meta["layout"], "-"))
	done := map[string]bool{}
	for _, key := range soffio.LinkKeys(doc) {
		fmt.Fprintln(w, doc.ID, key, doc.Meta[key])
		done[doc.Meta[key]] = true
	}
	for _, to := range soffio.Links(doc) {
		if !done[to] {
			fmt.Fprintln(w, doc.ID, "-", to)
		}
	}
}
