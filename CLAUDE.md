# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Soffio is a strict static site generator in Go, stdlib only, and a Unix tool: one job,
done whole, the same way every time. It parses `.soffio` texts, checks the whole site
(links, notes, files, public pages pointing at private ones), and renders each text
through an `html/template` layout. With no directory it is a filter: a text on
standard input, HTML on standard output. The README is the man page.

It knows its markup, its links and its files, and nothing of what a site is about: no
languages, dates or lists. A site that needs more is a program of its own calling
`soffio.Main(funcs, check)`; the Fucina's is fucina-engine (Fossil,
`~/fossils/fucina-engine.fossil`). Before adding anything, ask whether every site needs
it and whether only soffio can do it; if not, it belongs to the site's program.

## The contract

Soffio gives mechanisms, never policies, as Lua's core does to its host. A site
extends it in four ways and no other:

1. functions for its templates, `Main`'s `funcs`, never named as soffio's (`doc`,
   `href`, `size`, `today`);
2. a `check` of its texts, which reads them and changes none; its errors are
   `file:line: message` (from `Document.Lines`), said after soffio's, all at once;
3. its templates, the `.html` files of `-t`: a layout per page, and partials. Soffio
   makes HTML pages, one per text, and nothing else (a sitemap, robots.txt, a 404
   are a server's, or a text, `404.soffio`);
4. programs of its own on package `soffio` (`Load`, `Check`, `Links`, `Parse`,
   `Render`).

The markup and the header (three keys, `title layout visibility`, each a word; two
kinds of value, `-> target` and `/static/...`) are fixed since 5.0.0. Soffio changes
to fix what is wrong, or to give a site in a better form what it already computes
when the old form costs the site too much: 5.1.0 keeps `Links` in `Document.Links`,
read once by `Load`, and gives the templates `.Pages`. It never gains a policy (a
word, a choice of behaviour, what a site is about). A need none of the four meets is a
missing mechanism, discussed as one.

## Commands

```
make                                     # soffio for here
make GOOS=windows GOARCH=amd64 OUT=dir   # for another system, into dir
make test                                # go vet, then every test with its coverage
go test . -run TestCheckTarget           # one test
go test -fuzz FuzzParse -fuzztime 60s .  # after touching parse.go or inline.go
./release.sh                             # the release archives, see Release
```

The Makefile is the one way soffio is compiled: static (`CGO_ENABLED=0`), for the
baseline CPU of each family (`GOAMD64=v1`, `GOARM64=v8.0`, `GOARM=6`, `GO386=sse2`),
`-trimpath`, `-s -w`; go stamps the version from the tag (`go version -m soffio`).
Speed comes from the compiler's defaults and, if `cmd/soffio/default.pgo` exists,
PGO; never `-gcflags=-B`.

Every package is covered at 100% of its statements, and a new branch comes with its
test. The command is tested end to end: the test binary runs as soffio when
`SOFFIO_MAIN` says so (`SOFFIO_MAIN=ext` is a site's own command, with a function and
a check). The empty `isBlock` and `isInline` methods have no statements.

Go 1.26+. The module is `github.com/jcostd/soffio/v5`: what it exports is the
contract; breaking it would need a /v6 path, and is not done.

`soffio [-a] [-n] [-baseurl url] [-o dir] [-s dir]... [-t dir] [-today day] dir`: `-a`
takes the private texts too, `-n` checks and writes nothing, `-baseurl` is `.BaseURL`
(empty without it), `-s` a static dir copied to `<o>/static` (repeatable), `-t` the
templates instead of the built-in ones, `-today` the templates' day. Pipe mode,
`soffio [-n] < text.soffio > text.html`, refuses the other flags (exit 2). No server,
no version flag, no default that depends on another flag.

## Release

`ChangeLog.txt` has a line for every change worth knowing, newest first, Slackware
style. Date the entry, commit "release vX.Y.Z", `git tag -a vX.Y.Z` (the tag is the
version; there is no VERSION file), push the commit and the tag, `./release.sh`, and
upload `dist/` to the GitHub release: `soffio-<ver>-<os>-<arch>.tar.gz` (`.zip` for
windows), each with soffio, README, LICENSE and `templates/` (the built-in templates,
to start a `-t` from), owned by root, 755/644; the darwin ones also
`unquarantine.sh`, which lifts the quarantine of that folder and checks that soffio
runs. Build from the clean tag: the binaries carry the git revision. fucina-engine
requires the published module, so push the tag before releasing it. A version the Go
proxy has seen is its for good: a number is never used twice, and the path without
/v5 holds an old numbering that is not used.

## Architecture

One package, `soffio`, at the module root; `cmd/soffio` runs `Main(nil, nil)`.
Parse, load, check, render, write.

- **`doc.go`**: the tree, plain data. `Document` (File, ID, Meta, Sections, Lines,
  HeaderLinks, Links), `Section`, the blocks (`TextBlock`, `ImageBlock` with its size
  when known, `NoteBlock`, `ListBlock`, each with its first line) and the inlines
  (`PlainText`, `Bold`, `Italic`, `Link`, `FootnoteRef`).
- **`parse.go`**, **`inline.go`**: no state machine. The header is `key: value` lines
  up to the first blank line, keys of `[a-z0-9_]`, each once; a `-> target` value is a
  link (its key in `HeaderLinks`), a `/static/` value a file; any other value is a
  word, as written. The body is cut into blocks at blank lines and before every `==`
  and `::` line (`::img:` is refused); a block's first line says what it is. Headings
  skip no level. A BOM is dropped, invalid UTF-8 refused. Inline markup is scanned
  by byte, linearly: `*` and `_` open at the start of a word and close at its end,
  `(*id)` is a note only if `id` is a valid ID, a label holds no link or note.
  `fuzz_test.go` keeps it from panicking, nesting `<a>`, repeating an id or leaking
  markup.
- **`load.go`**: `Load` walks the directory in lexical order (so errors come in the
  same order), skips hidden files, refuses links to directories, and returns the
  documents by ID, the file's path without `.soffio`. Every part of an ID passes
  `checkID`; a text needs a title; IDs that differ only in case clash; `static/` is
  not an ID. Header links are resolved here, from the text's directory, into the
  page's ID, which `Meta[key]` then holds; a section, file or outside address is
  refused. Then `Document.Links` is set.
- **`url.go`**: `resolve` is the one place a link target is read: its ID or static
  file, and its `#fragment`. `Check` and `href` both use it, so what is checked is what
  is linked. `href` writes the address relative to the page.
- **`check.go`**: `Check(all, active, staticDirs)` walks the active documents in ID
  order: every link, image and header file points at an active page, one of its
  sections, or a regular file in a `-s`, through a valid address; every note is
  defined and reached from the body, maybe through other notes. A link to a document
  in all but not in active is a privacy leak. Files have no visibility: everything in
  `-s` is published, so drafts' files go in a `-s` only the preview gets; never guess
  privacy from links. `Links` reads the same walk: the pages a document links to,
  once each, sorted, never itself, a file or an outside address. `checkDoc` is what
  pipe mode can check alone.
- **`html.go`**: `Render` makes one document an HTML fragment, knowing nothing of the
  site. A `<section>` holds those of the levels below it. Notes are numbered as met
  and listed at the end with no heading; their anchors, `fn:x` and `fnref:x:n`, never
  clash with a section ID, which has no `:`.
- **`main.go`**: `Main(funcs, check)`, named as it was run (`os.Args[0]` up to its first
  dot): flags (`checkFlags`), templates with soffio's functions and the site's (a
  name of soffio's is refused), load, visibility, `Check` then the site's check,
  static copy, pages. `pipe` is the filter.
- **`write.go`**: a page goes through its layout (`layout` header, else
  `layout.html`; a missing one is an error) with `ID`, `Meta`, `Content`, `BaseURL`,
  `Docs` (the active pages by ID) and `Pages` (the same in ID order). Each page is
  executed into a buffer and written in one call; with `-n` it is executed and
  dropped. `copyDir` copies every `-s` into `<o>/static`, refusing a path in two of
  them, or twice in letter case, and a copy of a file onto itself (`os.SameFile`);
  with `-n` it checks the same and writes nothing. `loadTemplates` parses `-t` or the
  embedded `templates/*`, never a mix, with `missingkey=zero`. soffio's functions:
  `doc`, `href`, `size` (an image's `image.Config`, read once per path; `image.go`
  reads a WebP's from its first 30 bytes, the others by `image.DecodeConfig`) and
  `today` (a template never reads the clock). `prepare` gives every `ImageBlock` its
  size. The built-in `layout.html` needs only a title, has no word of its own and
  writes every address with `href`, so its site works from the disk; keep it small.
- **`example_test.go`**: the examples `go doc` shows, run by `go test`. The README's
  site program (EXTENDING) is checked by hand against the checkout when the API moves.

Messages are `file:line: message`, all of them, in a fixed order, no timestamps; about
the command, its name first. Every page is attempted, but any failure exits 1; bad
usage exits 2. No warnings: what is worth saying is worth fixing.

## Conventions

- Stdlib only: a stated goal, not an oversight.
- Strict by design: broken links, missing or unused notes, missing files, unknown
  layouts and privacy leaks fail the build; bad input is refused with a reason.
- IDs are output paths and link targets, always with `/`; they become file paths only
  where a file is opened, with `filepath.FromSlash`.
- Output is reproducible: what is ranged over a map is sorted first.
- Measure before tuning: a big corpus and `runtime/pprof` found the unbuffered writes,
  as guesses would not have.
- The README is the reference for the markup, over any example in `content/`.
