// Copyright (C) 2026 Jacopo Costantini
// SPDX-License-Identifier: GPL-3.0-or-later

package soffio_test

import (
	"fmt"
	"strings"

	"github.com/jcostd/soffio"
)

const text = `title: Il toro
by: -> /it/persone/arp

== storia | Storia

Il _Toro_ nasce nel 1954.(*disegno) Vedi (l'elenco -> ../opere).

:: note: disegno | Un disegno a matita.
`

// A text alone: its header, as Meta and Lines, and its body.
func ExampleParse() {
	doc, err := soffio.Parse("toro.soffio", strings.NewReader(text))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(doc.Meta["title"], "at line", doc.Lines["title"])
	fmt.Println(doc.HeaderLinks, doc.Meta["by"])
	fmt.Println(doc.Sections[0].ID, doc.Sections[0].Level)
	// Output:
	// Il toro at line 1
	// [by] /it/persone/arp
	// storia 2
}

// A text as HTML: links relative to its page, the notes at the end.
func ExampleRender() {
	doc, _ := soffio.Parse("toro.soffio", strings.NewReader(text))
	doc.ID = "it/opere/toro"
	fmt.Print(soffio.Render(doc))
	// Output:
	// <section id="storia">
	// <h2>Storia</h2>
	// <p>Il <em>Toro</em> nasce nel 1954.<sup id="fnref:disegno:1"><a href="#fn:disegno" role="doc-noteref">1</a></sup> Vedi <a href="../opere.html">l&#39;elenco</a>.</p>
	// </section>
	// <section role="doc-endnotes">
	// 	<ol>
	// 		<li id="fn:disegno">Un disegno a matita. <a href="#fnref:disegno:1" role="doc-backlink">↩</a></li>
	// 	</ol>
	// </section>
}

// The pages a text links to, from its header and its body: read the
// other way, the pages that link to a page.
func ExampleLinks() {
	doc, _ := soffio.Parse("toro.soffio", strings.NewReader(text))
	doc.ID = "it/opere/toro"
	doc.Meta["by"] = "it/persone/arp" // as Load resolves it
	fmt.Println(soffio.Links(doc))
	// Output:
	// [it/opere it/persone/arp]
}
