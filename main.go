// Command soffio compiles a static website from soffio markup files.
package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"

	"soffio/corpus"
)

//go:embed content/*.soffio
var contentFS embed.FS

func main() {
	soffioFS, err := fs.Sub(contentFS, "content")
	if err != nil {
		log.Fatalf("failed to create sub-filesystem: %v", err)
	}

	c := corpus.New()
	if err := c.Load(soffioFS, "*.soffio"); err != nil {
		log.Fatalf("load failed:\n%v", err)
	}

	if errs := c.ValidateLinks(); len(errs) > 0 {
		for _, err := range errs {
			var linkErr *corpus.BrokenLinkError
			if errors.As(err, &linkErr) {
				log.Printf("broken link: %s -> %s\n", linkErr.Source, linkErr.Target)
			} else {
				log.Printf("error: %v\n", err)
			}
		}
	}

	fmt.Printf("loaded %d documents.\n", len(c.Docs))
}
