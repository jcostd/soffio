package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"soffio/parser"
)

func printValidationReport(errs []error) {
	for _, err := range errs {
		if brokenErr, ok := errors.AsType[*BrokenLinkError](err); ok {
			log.Printf("[!] Warning: broken link %s in %s\n",
				brokenErr.TargetID, brokenErr.SourceDocID)
		} else {
			log.Println("[?] generic error:", err)
		}
	}
}

func main() {
	lib := Library{
		Documents: make(map[string]*parser.Document),
	}

	fsys := os.DirFS("content")

	if err := lib.Load(fsys, "*.soffio"); err != nil {
		log.Fatalf("Errors during library load:\n%v", err)
	}

	errs := lib.ValidateLinks()
	if len(errs) > 0 {
		printValidationReport(errs)
	}

	lib.PrintEntries()
	fmt.Printf("Libreria caricata! Trovati %d documenti.\n", len(lib.Documents))
}
