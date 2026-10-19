# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Soffio is a minimalist, strict static site generator written in Go with zero external
dependencies, and a Unix tool: one job, done whole, the same way every time. It parses
a custom `.soffio` markup format, builds an in-memory corpus of documents, validates
referential integrity across the whole corpus (internal links, footnotes, files,
public/private leaks), then renders HTML via `html/template` layouts. It also runs as a
filter (stdin `.soffio` -> stdout HTML).

It knows its markup, its links and its files, and nothing of what a site is about: no
languages, no dates, no lists. What a site wants more is a program of its own that calls
`site.Main(funcs, check)` (the Fucina's is `fucina-engine`, in Fossil at
`~/fossils/fucina-engine.fossil`). Before adding anything here, ask whether every site
needs it and whether only soffio can do it; if not, it belongs to the site's program.

## Commands

The Makefile is the one place that says how soffio and preview are compiled: static
(`CGO_ENABLED=0`), for the baseline CPU of each family whatever the environment says
(`GOAMD64=v1`, `GOARM64=v8.0`, `GOARM=6`, `GO386=sse2`), `-trimpath`, `-s -w`; go
stamps the version from the tag (`go version -m soffio`). release.sh calls it; don't
compile release binaries any other way. Speed comes from the compiler's defaults and,
if `cmd/soffio/default.pgo` exists, from profile-guided optimization; never
`-gcflags=-B`.

```
make                                     # soffio and preview for here, in the repo root
make GOOS=windows GOARCH=amd64 OUT=dir   # for another system, into dir
make test                                # go vet, then every test, with its coverage
./release.sh                             # the release archives, see Release
```

Run one package's tests, or one test by name:

```
go test .
go test . -run TestCheckTarget
go test ./... -v
```

Every package is covered at 100% of its statements (`go test -cover ./...`), and
stays so: a new branch comes with the test that takes it. The commands are tested
end to end, as they run: each test binary's `TestMain` runs the command when
`SOFFIO_MAIN` or `PREVIEW_MAIN` says so (`SOFFIO_MAIN=ext` is a site's own command,
with a function and a check), and the tests run it as the command, whose coverage
`go test` counts as its own; what can't be written is a standard output open
read-only, what can't be read a file of mode 0. The empty `isBlock` and `isInline`
methods have no statements to cover.

Requires Go 1.26+ (see `go.mod`). The module is `github.com/jcostd/soffio/v5`: a new
major version changes the path, as Go wants.

Generator mode: `soffio [-a] [-n] [-baseurl url] [-o dir] [-s dir]... [-t dir] [-today day] dir`
reads the `.soffio` texts under dir and writes a site to `-o` (default `public`): `-a`
takes the private texts too, `-n` checks everything and writes nothing, `-baseurl` is
`.BaseURL` (empty if not given: no default that depends on another flag), `-s` a
static dir copied to `<o>/static` (repeatable: the drafts' files go in one only the
preview gets, as everything in `-s` is published), `-t` the templates instead of the
built-in ones, `-today` the templates' day. Pipe mode, with no dir:
`soffio [-n] < text.soffio > text.html` (`-n` only checks; any other flag is refused,
exit 2). `preview [-p port] [dir]` serves dir, `public` if none. No version flag.
The README is the man page.

## Release

Every change worth knowing gets a line in `ChangeLog.txt`, newest first, Slackware
style. Bump `VERSION`, date the ChangeLog entry, commit "bump to vX.Y.Z",
`git tag -a vX.Y.Z`, `./release.sh`, and upload what it made in `dist/` to the
GitHub release: `soffio-<ver>-<os>-<arch>.tar.gz` (`.zip`
for windows), each holding the two programs as the Makefile builds them, README,
LICENSE and `templates/` (the built-in templates as files, to start a `-t` from),
owned by root, 755/644; the darwin ones also `unquarantine.sh`, which a Mac user runs
with `sh` to lift the quarantine of a download from that folder (it checks soffio
runs with `soffio -n < /dev/null`). Build them from the clean tag: the binaries carry
the git revision, and `vcs.modified=true` if the tree was dirty. A site's own command
(fucina-engine) requires the tagged module, so tag and push before releasing it.

## Architecture

Pipeline: parse -> load -> check -> render -> site assembly. The language is one
package, `soffio`, at the module root; package `site` is the command, which
`cmd/soffio` runs with nothing added and a site's own program with its functions.

- **`doc.go`**: the document tree. `Document` (File, ID, Title, Meta, `[]Section`),
  `Section` (level, ID, title, `[]Block`), blocks (`TextBlock`, `ImageBlock` with its
  width and height when known, `NoteBlock`, `ListBlock`, each with the line it starts
  at) and inlines (`PlainText`, `Bold`, `Italic`, `Link`, `FootnoteRef`). Pure data.

- **`parse.go`**: no state machine. The header is `key: value` lines up to the first
  blank line: keys of `[a-z0-9_]` (a template reads `.Meta.key`), each once. soffio
  reads three keys, `title`, `layout`, `visibility` (public or private), and two kinds
  of value, whatever the key: `-> target`, a link (not in `layout` or `visibility`),
  kept in `Document.links`, and a value under `/static/`, a file, which `Check`
  checks. Every other value is a word, as written: no key has a type it does not
  need (dates, images, ids are the site's). The body is cut into blocks at blank
  lines, and before every `==` section line and `::` command line (`::img:` is
  refused, not text); then a block's first line says what it is (command, `- ` list,
  or text). A leading BOM is dropped, invalid UTF-8 refused.
  `inline.go` scans inline markup by byte (every marker is ASCII): a `*` or `_` opens
  only at the start of a word and closes only at its end; `(*id)` is a note only when
  `id` is a valid ID; `(label -> target)` is a link, and a link or note inside a
  label is an error, said by `inline` itself. The scan is linear: parentheses
  are paired once (`parens`), a failed search for a closing marker is remembered (a
  closer is good whatever opener it is for), and a label is parsed one level deep
  only. The fuzz test in `fuzz_test.go` keeps it from panicking, nesting `<a>`,
  repeating an id or leaking markup: `go test -fuzz FuzzParse -fuzztime 60s .`
  after touching it.

- **`load.go`**: `Load` walks the source dir in lexical order, sequentially (parsing is
  microseconds; order makes errors reproducible), skipping hidden files and refusing
  links to directories, and returns documents by ID: the file's path without
  `.soffio` (`it/about`), the one name of a page. Every part of an ID passes
  `CheckID`; a text with no title is refused (an empty link in every list of pages);
  IDs differing only in case are duplicates; `static/` is no ID, it is where the
  static files go. `Document.File` is the path for messages. A header link is
  resolved here, from the text's directory as a body link is, into the page's ID,
  which `Meta[key]` then holds: a section, a file or an address out of the site is
  refused, as a header link names a page.

- **`url.go`**: `resolve` is the one place a link target is interpreted: the ID or
  static file it points to, and its `#fragment`. `Check` and `href` both go through
  it, so what is checked is what is linked. `href` makes the address relative to
  the page (common prefix plus `../`), so a site needs no absolute URLs; a target
  under `static/` is a file, anything else a page and gets `.html`.

- **`check.go`**: `Check(all, active, staticDirs)` walks only the visibility-filtered
  active documents, in ID order: every link, image and header file points at an
  active page, one of its sections, or a regular file in one of `staticDirs`, through
  a valid address; every note is defined and reached from the body, maybe through
  other notes, as `Render` reaches it: a note referred to only by itself, or by
  unreached notes, is "never referenced". A link to a document in all but not active
  is a **privacy leak**, an error of its own. This active-vs-all model is how `-a`
  works; keep it. Header links and files are checked at their header line. `Links`
  is the other half: the IDs of the pages a document links to, header and body
  (notes, captions, list items), once each, sorted, never itself, a file or an
  outside address; `LinkKeys` the header keys that hold a link, for a site's own
  rules. Files have no visibility: everything in -s is published, and a draft's
  files live in a -s that only the preview gets, so a public text pointing at one is
  a missing file. Never guess privacy from links (headers, CSS and templates point
  at files too): fail closed. `CheckDoc` is what pipe mode can check alone: notes and
  `#section` links.

- **`html.go`**: `Render` returns one document as an HTML fragment, knowing nothing of
  the corpus. Note refs are numbered as met and the endnotes come last, in order of
  first reference, with no heading (none reads well in any language). Their anchors are
  `fn:x` and `fnref:x:n`: no section ID holds ':', so they never clash with one.

- **`site`**: `Main(funcs, check)` is the command, named as it was run (`os.Args[0]`
  up to its first dot): flags (`checkFlags`: a -baseurl, if given, is
  http(s)://host[/path] with nothing html/template would escape), templates with
  soffio's functions and the site's (one of soffio's names is refused), load ->
  visibility filter -> `Check` -> the site's `check` -> static copy -> pages and site
  files. `write.go`: each page goes through its layout (`layout` header, else
  `layout.html`; a missing one is an error) with `ID`, `Title`, `Meta`, `Content`,
  `BaseURL`, `Backlinks` (the pages whose `Links` hold it: `prepare` turns every
  page's links round once, in ID order, so they come by ID) and `Docs` (every active
  page, as the site files get). Every file is executed into a buffer and written in
  one call; html/template alone writes in small pieces; with `-n` it is executed and
  dropped. `copyDir` copies every -s into `<o>/static`, refusing a path in two of them
  or twice in letter case only (one address, two files), and refuses, by
  `os.SameFile`, to copy a file onto itself (it would come out empty) when a -s and
  `<o>/static` overlap, however the paths are spelled; with `-n` it walks and checks
  the same and writes nothing, so `-n` exits as the build would. `Check` refuses a
  link to a hidden file under -s, as `copyDir` never copies one.
  `template.go` parses either `-t` or the embedded `site/templates/*`, never a mix,
  with `missingkey=zero` (a header key a page lacks is "", as in Go, so a function
  given `.Meta.by` gets a string), and gives templates soffio's three functions, what
  only it knows: `doc` (the page of an ID, or nil), `size` (an image's width and
  height, read once per path from its file in a `-s` by `image.DecodeConfig`: jpeg,
  png, gif; or nil) and `today` (`-today` or empty: a template never reads the clock,
  so the output stays reproducible). `site.funcs` makes them before the templates are
  parsed; they ask the site, whose pages come later. `prepare` also gives every
  `ImageBlock` of a text its size, which `Render` writes. The embedded templates are
  the showcase: they need only a title (lang, date and updated if there), list every
  page on the home and the backlinks under each, share a `style` partial; keep them
  that small. `siteFiles` (index.html, 404.html, robots.txt, sitemap.xml) are each
  made if the template is there.

- **`cmd/preview`**: a static file server (`http.FileServer`) over a dir, `public` if
  none, that opens the default browser (`open` on macOS, `start` on Windows, else
  `xdg-open`: `browsers`, a table by `runtime.GOOS`, so it builds on the BSDs too).
  Every response carries a `Soffio-Preview: <abs dir>` header, so a second `preview`
  of the same dir finds the first one when the port is taken, opens the browser on
  it, and exits 0. Also `Cache-Control: no-store` (a rebuilt site is never shown from
  the browser's cache), `X-Content-Type-Options: nosniff`, `Referrer-Policy:
  no-referrer` (the preview holds private texts), and a `Content-Type` from the fixed
  `types` table for a site's extensions: Go asks the system otherwise, and a Windows
  registry may call `.css` text/plain, or a system lack `.woff2`.
- **Messages and exit status**: errors are `file:line: message`, all of them, in a
  fixed order, with no timestamps (`log.SetFlags(0)`); about the command, its name
  first. Every page and site file is attempted, but if any fails the command exits 1;
  bad usage exits 2. A missing page is a broken site. No warnings, no defaults that
  depend on other flags, no second way to say one thing.

## Markup format (`.soffio`)

Header (`key: value` lines) + blank line + body. Keys soffio reads: `title`, `layout`,
`visibility`. Values it reads, in any other key: `-> target`, a link to a page (the
templates get its ID), and `/static/...`, a file. Everything else lands in `Meta` as
written, for the templates and a site's own program. Body syntax: `== id | Title`
(section header, level 2–6 `=`), `*bold*`, `_italic_`, `(Label -> target)` links,
`:: img: /static/path | caption`, `- item` lists, `:: note: id | text` footnote defs,
`(*note-id)` footnote refs. Link targets without a leading `/` resolve relative to the
current document's directory; a `#section-id` suffix targets a heading within a
document (own document if the path part is omitted). Full syntax reference is in
`README` (man-page style): treat it as the source of truth over any example content
in `content/`.

## Notable conventions

- No third-party dependencies (stdlib only): keep it that way; this is a stated design
  goal, not an oversight.
- Validation is strict by design, and there are no warnings: as the Go FAQ says of
  unused imports, what is worth complaining about is worth fixing. Broken links,
  missing or unused notes, missing files, unknown layouts and privacy leaks fail the
  build; bad input is refused with a reason, never guessed at.
- Document IDs are output paths (`<id>.html`) and link targets, always with `/`, on
  every system: turn one into a file path only where the file is opened, with
  `filepath.FromSlash`.
- Output is reproducible: anything ranged over a map (IDs, notes, errors) is sorted
  first.
- Measure before tuning: a big corpus (fucina-content copied 300 times) and
  `runtime/pprof` found the unbuffered writes; guesses would not have.
