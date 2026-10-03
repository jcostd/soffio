# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Soffio is a minimalist, strict static site generator written in Go with zero external
dependencies. It parses a custom `.soffio` markup format, builds an in-memory corpus of
documents, validates referential integrity across the whole corpus (internal links,
footnotes, image assets, public/private leaks), then renders HTML via `html/template`
layouts. It also runs as a UNIX filter (stdin `.soffio` -> stdout HTML) with no directory
mode.

## Commands

The Makefile is the one place that says how soffio and preview are compiled: static
(`CGO_ENABLED=0`), for the baseline CPU of each family whatever the environment says
(`GOAMD64=v1`, `GOARM64=v8.0`, `GOARM=6`, `GO386=sse2`), `-trimpath`, `-s -w`, the
version from `VERSION`. release.sh calls it; don't compile release binaries any other
way. Speed comes from the compiler's defaults and, if `cmd/soffio/default.pgo` exists,
from profile-guided optimization; never `-gcflags=-B`.

```
make                                     # soffio and preview for here, in the repo root
make GOOS=windows GOARCH=amd64 OUT=dir   # for another system, into dir
make test                                # go vet, then every test
./release.sh                             # the release archives, see Release
```

Run one package's tests, or one test by name:

```
go test .
go test . -run TestCheckTarget
go test ./... -v
```

Requires Go 1.26+ (see `go.mod`).

Generator mode: `soffio [-a] [-n] [-baseurl url] [-langs l,...] [-o dir] [-s dir]... [-t dir] dir`
reads the `.soffio` texts under dir and writes a site to `-o` (default `public`): `-a`
takes the private texts too, `-n` checks everything and writes nothing, `-baseurl` is
required for the public site (with `-a` or `-n` it is the preview's localhost if not
given), `-s` a
static dir copied to `<o>/static` (repeatable: the drafts' files go in one only the
preview gets, as everything in `-s` is published), `-t` the templates instead of the built-in ones,
`-v` prints the version. Pipe mode, with no dir: `soffio [-n] < text.soffio > text.html`
(`-n` only checks; any other flag is refused, exit 2).
The README is the man page.

## Release

Every change worth knowing gets a line in `ChangeLog.txt`, newest first, Slackware
style. Bump `VERSION`, date the ChangeLog entry, commit "bump to vX.Y.Z",
`git tag -a vX.Y.Z`, `./release.sh`, and upload what it made in `dist/` to the
GitHub release: `soffio-<ver>-<os>-<arch>.tar.gz` (`.zip`
for windows), each holding the two programs as the Makefile builds them, README,
LICENSE and `templates/` (the built-in templates as files, to start a `-t` from),
owned by root, 755/644.
fucina-factory takes those archives as they are, so build them from the clean tag:
the binaries carry the git revision, and `vcs.modified=true` if the tree was dirty.

## Architecture

Pipeline: parse -> load -> check -> render -> site assembly. The language is one
package, `soffio`, at the module root; `cmd/soffio` builds the site with it.

- **`doc.go`**: the document tree. `Document` (ID, Title, Meta, `[]Section`),
  `Section` (level, ID, title, `[]Block`), blocks (`TextBlock`, `ImageBlock`,
  `NoteBlock`, `ListBlock`, each with the line it starts at) and inlines (`PlainText`,
  `Bold`, `Italic`, `Link`, `FootnoteRef`). Pure data.

- **`parse.go`**: no state machine. The header is `key: value` lines up to the first
  blank line. The body is cut into blocks at blank lines, and before every `==`
  section line and `::` command line (`::img:` is refused, not text); then a block's
  first line says what it is (command, `- ` list, or text). A leading BOM is dropped, invalid UTF-8 refused.
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
  links to directories, and returns documents by ID: the
  `id` header or the file name, under the file's directory (`it/about`). Every part
  of an ID passes `CheckID`; a text with no title is refused (an empty link in every
  list of pages); IDs differing only in case are duplicates; `static/` is no ID, it is where the
  static files go. `Document.File` is the path for messages.

- **`url.go`**: `resolve` is the one place a link target is interpreted: the ID or
  static file it points to, and its `#fragment`. `Check` and `href` both go through
  it, so what is checked is what is linked. `href` makes the address relative to
  the page (common prefix plus `../`), so the i18n layout needs no absolute URLs; a
  target under `static/` is a file, anything else a page and gets `.html`.

- **`check.go`**: `Check(all, active, staticDirs)` walks only the visibility-filtered
  active documents, in ID order: every link and image points at an active page, one of
  its sections, or a regular file in one of `staticDirs`, through a valid address; every
  note is defined and reached from the body, maybe through other notes, as `Render`
  reaches it: a note referred to only by itself, or by unreached notes, is "never
  referenced". A link to a document in all but not active is a **privacy leak**, an
  error of its own. This active-vs-all model is how `-a` works; keep it. Files have no
  visibility: everything in -s is published, and a draft's files live in a -s that only
  the preview gets, so a public text pointing at one is a missing file. Never guess
  privacy from links (headers, CSS and templates point at files too): fail closed. `CheckDoc`
  is what pipe mode can check alone: notes and `#section` links.

- **`html.go`**: `Render` returns one document as an HTML fragment, knowing nothing of
  the corpus. Note refs are numbered as met and the endnotes come last, in order of
  first reference, with no heading (none reads well in any language). Their anchors are
  `fn:x` and `fnref:x:n`: no section ID holds ':', so they never clash with one.

- **`cmd/soffio`**: flags (`checkFlags`: -baseurl is http(s)://host[/path] with
  nothing html/template would escape) and templates, configuration first, then load ->
  visibility filter -> check -> static copy -> pages and site files. `write.go`: each page goes through its layout
  (`layout` header, else `layout.html`; a missing one is an error) with `Children` (docs under `<id>/`, by ID:
  the IDs are sorted once and the children are the run from `id/` to `id0`, two
  binary searches), `Alternates` (the same path in each `-langs` language, only when
  the first part of the ID is one) and `Lang` (that first part, else the first of
  `-langs`). Every file is executed into a buffer and written in one call;
  html/template alone writes in small pieces; with `-n` it is executed and dropped.
  `copyDir` copies every -s into `<o>/static`, refusing a path in two of them or twice
  in letter case only (one address, two files), and refuses, by `os.SameFile`, to copy a
  file onto itself (it would come out empty) when a -s and `<o>/static` overlap,
  however the paths are spelled; with `-n` it
  walks and checks the same and writes nothing, so `-n` exits as the build would.
  `Check` refuses a link to a hidden file under -s, as `copyDir` never copies one.
  `template.go` parses either `-t` or the embedded `cmd/soffio/templates/*`, never a
  mix, and gives templates `sortBy` and `rfc822`. The embedded ones are the showcase:
  they need only a title (date and updated if there) and use `.Lang`, `.Alternates`,
  `.Children` with `sortBy`, `rfc822`, a shared `style` partial; keep them that small. `siteFiles` in `main.go` (rss.xml,
  sitemap.xml, robots.txt, 404.html, manifest.json) are each made if the template is
  there.

- **`cmd/preview`**: a static file server (`http.FileServer`) over the output dir
  that opens the default browser (`open` on macOS, `start` on Windows, else `xdg-open`,
  one switch on `runtime.GOOS`, so it builds on the BSDs too).
  Every response carries a `Soffio-Preview: <abs dir>` header, so a second `preview`
  of the same dir finds the first one when the port is taken, opens the browser on
  it, and exits 0.
- **Messages and exit status**: errors are `file:line: message`, all of them, in a
  fixed order, with no timestamps (`log.SetFlags(0)`). Every page and site file is
  attempted, but if any fails `soffio` exits 1; bad usage exits 2. A missing page is
  a broken site.

## Markup format (`.soffio`)

Frontmatter (`key: value` lines) + blank line + body. Known frontmatter keys: `id`,
`title`, `layout`, `visibility`; `date`, `updated`, `*_date` are dates;
`image`, `*_image` are images, checked as `:: img:` (parse: under `/static/`; `Check`: the
file is there, at the header line, which `Document.lines` keeps); any other key lands in `Meta` and is
available to templates/`sortBy`; `visibility` is `public` or `private`. Body syntax: `== id | Title` (section header, level
2–6 `=`), `*bold*`, `_italic_`, `(Label -> target)` links, `:: img: /static/path | caption`,
`- item` lists, `:: note: id | text` footnote defs, `(*note-id)` footnote refs. Link
targets without a leading `/` resolve relative to the current document's directory
(zero-config i18n); a `#section-id` suffix targets a heading within a document (own
document if the path part is omitted). Full syntax reference is in `README` (man-page
style) — treat it as the source of truth over any example content in `content/`.

## Notable conventions

- No third-party dependencies (stdlib only) — keep it that way; this is a stated design
  goal, not an oversight.
- Validation is strict by design, and there are no warnings: as the Go FAQ says of
  unused imports, what is worth complaining about is worth fixing. Broken links,
  missing or unused notes, missing files, unknown layouts and privacy leaks fail the
  build; bad input is refused with a reason, never guessed at.
- Document IDs are output paths (`<id>.html`), link targets and the parent/child
  relation, always with `/`, on every system: turn one into a file path only where
  the file is opened, with `filepath.FromSlash`.
- Output is reproducible: anything ranged over a map (IDs, children, notes, errors) is
  sorted first.
- Measure before tuning: a big corpus (fucina-content copied 300 times) and
  `runtime/pprof` found the unbuffered writes; guesses would not have.
