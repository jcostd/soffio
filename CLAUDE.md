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
go test . -run TestCheckLink
go test ./... -v
```

Requires Go 1.26+ (see `go.mod`).

Generator mode: `soffio [flags] <src_dir>` — reads `.soffio` files from `<src_dir>`, writes
a static site to `-o` (default `public`). Pipe mode: `soffio < input.soffio > output.html`
(triggered whenever no positional directory argument is given). See `soffio -help` for the
full flag list (`-baseurl`, `-langs`, `-o`, `-t`, `-s`, `-vis`, and `-html`/`-rss`/`-sitemap`/
`-robots`/`-errpage`/`-manifest` toggles).

## Release

Bump `VERSION`, commit "bump to vX.Y.Z", `git tag -a vX.Y.Z`, `./release.sh`, and upload
what it made in `dist/` to the GitHub release: `soffio-<ver>-<os>-<arch>.tar.gz` (`.zip`
for windows), each holding the two programs as the Makefile builds them, README and
LICENSE, owned by root, 755/644.
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
  section line and `:: ` command line; then a block's first line says what it is
  (command, `-` list, or text). `inline.go` scans inline markup by byte (every marker
  is ASCII): a `*` or `_` opens only at the start of a word and closes only at its
  end; `(*id)` is a note only when `id` is a valid ID; `(label -> target)` is a link.

- **`load.go`**: `Load` walks the source dir in lexical order, sequentially (parsing is
  microseconds; order makes errors reproducible), and returns documents by ID: the
  `id` header or the file name, under the file's directory (`it/about`). IDs
  differing only in case are duplicates.

- **`url.go`**: `resolve` is the one place a link target is interpreted: the ID or
  static file it points to, and its `#fragment`. `Check` and `href` both go through
  it, so what is checked is what is linked. `href` makes the address relative to
  the page (common prefix plus `../`), so the i18n layout needs no absolute URLs; a
  target under `static/` is a file, anything else a page and gets `.html`.

- **`check.go`**: `Check(all, active, staticDir)` walks only the visibility-filtered
  active documents, in ID order, and verifies links, note refs and images. A link to
  a document in all but not active is a **privacy leak**, a hard error, not just a
  missing target. This active-vs-all model is how `-vis` works; keep it.

- **`html.go`**: `Render` returns one document as an HTML fragment, knowing nothing of
  the corpus. Note refs are numbered as met and the endnotes come last, in order of
  first reference.

- **`cmd/soffio`**: flags, pipe mode, then load -> visibility filter -> check ->
  templates -> pages and site files. `write.go`: each page goes through its layout
  (`layout` header, else `layout.html`) with `Children` (docs under `<id>/`, by ID:
  the IDs are sorted once and the children are one run, found by binary search) and
  `Alternates` (the same path in each `-langs` language, only when the first part
  of the ID is one). Every file is executed into a buffer and written in one call;
  html/template alone writes in small pieces. `template.go` embeds
  `cmd/soffio/templates/*` and lets `-t` replace any of them by name; it also gives
  templates `sortBy`. rss.xml, sitemap.xml, robots.txt, 404.html and manifest.json
  are a table in `main.go`: each is made if its flag is on and its template exists.

- **`cmd/preview`**: a static file server (`http.FileServer`) over the output dir
  that opens the default browser (`open_darwin.go`/`open_linux.go`/`open_windows.go`).
  Every response carries a `Soffio-Preview: <abs dir>` header, so a second `preview`
  of the same dir finds the first one when the port is taken, opens the browser on
  it, and exits 0.
- **Exit status**: every page, feed and extra file is attempted, but if any of them fails
  to render, `soffio` exits 1. A missing page is a broken site.

## Markup format (`.soffio`)

Frontmatter (`key: value` lines) + blank line + body. Known frontmatter keys: `id`,
`title`, `layout`, `visibility`, `notes_title`; any other key lands in `Meta` and is
available to templates/`sortBy`. Body syntax: `== id | Title` (section header, level
2–6 `=`), `*bold*`, `_italic_`, `(Label -> target)` links, `:: img: /path | caption`,
`- item` lists, `:: note: id | text` footnote defs, `(*note-id)` footnote refs. Link
targets without a leading `/` resolve relative to the current document's directory
(zero-config i18n); a `#section-id` suffix targets a heading within a document (own
document if the path part is omitted). Full syntax reference is in `README` (man-page
style) — treat it as the source of truth over any example content in `content/`.

## Notable conventions

- No third-party dependencies (stdlib only) — keep it that way; this is a stated design
  goal, not an oversight.
- Validation is strict by design: broken links, missing footnotes, and privacy leaks fail
  the build rather than degrading gracefully. Don't soften these into warnings.
- Document IDs are output paths (`<id>.html`), link targets and the parent/child
  relation, always with `/`, on every system: turn one into a file path only where
  the file is opened, with `filepath.FromSlash`.
- Output is reproducible: anything ranged over a map (IDs, children, notes, errors) is
  sorted first.
- Measure before tuning: a big corpus (fucina-content copied 300 times) and
  `runtime/pprof` found the unbuffered writes; guesses would not have.
