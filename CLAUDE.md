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

```
make            # build ./cmd/soffio and ./cmd/preview binaries into repo root
make serve      # build, then run ./preview (serves ./public on :8080 and opens browser)
make clean      # remove binaries and the generated ./public directory
make test       # go test ./...
```

Run a single package's tests or a single test by name directly with `go test`:

```
go test ./parser/...
go test ./corpus/... -run TestValidateLinks
go test ./... -v
```

Requires Go 1.26+ (see `go.mod`).

Generator mode: `soffio [flags] <src_dir>` — reads `.soffio` files from `<src_dir>`, writes
a static site to `-o` (default `public`). Pipe mode: `soffio < input.soffio > output.html`
(triggered whenever no positional directory argument is given). See `soffio -help` for the
full flag list (`-baseurl`, `-langs`, `-o`, `-t`, `-s`, `-vis`, and `-html`/`-rss`/`-sitemap`/
`-robots`/`-errpage`/`-manifest` toggles).

## Architecture

Pipeline: `parser` -> `ast` -> `corpus` -> `renderer` -> `cmd/soffio` (site assembly).

- **`ast`**: Defines the document tree — `Document` (ID, Title, Meta, `[]Section`),
  `Section` (heading level/ID/title + `[]Block`), block types (`TextBlock`, `ImageBlock`,
  `NoteBlock`, `ListBlock`), and inline types (`PlainText`, `Bold`, `Italic`, `Link`,
  `FootnoteRef`). Pure data, no logic.

- **`parser`**: A hand-written line-oriented state machine (no regex/lexer generator).
  `block.go` runs a header/body state machine (`stateHeader`/`stateBody`): the header is
  RFC 822-style `key: value` lines terminated by a blank line; the body accumulates lines
  into a buffer and flushes into a `Block` on blank lines, new `==` sections, or new `:: `
  commands. `inline.go` recursively parses inline markup (`*bold*`, `_italic_`, `(Label ->
  target)` links, `(*note-id)` footnote refs) inside block content. When editing parsing
  logic, understand the buffer/flush lifecycle in `block.go`'s `stepBody`/`flush` before
  changing state transitions.

- **`corpus`**: Owns the whole-site view. `corpus.go` concurrently loads and parses every
  `.soffio` file under a source dir (bounded worker pool via a semaphore channel) into a
  `Collection` keyed by document ID (derived from frontmatter `id`, prefixed by the
  relative directory path for i18n namespacing, e.g. `it/about`). `validate.go` performs
  a second pass over only the *visibility-filtered* active document set, walking every
  inline element to confirm link targets and footnote refs resolve, and to catch
  **privacy leaks**: a link from a visible/public document into a document that exists in
  the full corpus but was filtered out (private) is a hard error (`PrivacyLeakError`), not
  just a missing-target error. This visibility-based validation model (active docs vs. all
  docs) is central to how the `-vis` flag works and must be preserved when touching
  link-checking.

- **`renderer`**: Stateless-per-call HTML emission from a single `ast.Document`, with no
  knowledge of the rest of the corpus (cross-doc validity is corpus's job, not renderer's).
  `render.go` walks sections/blocks/inlines and streams HTML; footnote refs are collected
  during the walk and the endnotes section is emitted at the end in first-referenced order.
  `urls.go`'s `resolveURL` converts a Soffio link target into a relative on-disk `.html`
  path using the *shared prefix* between source and target document IDs (POSIX-style `..`
  relative pathing) — this is what makes the i18n directory layout (`content/en/...`,
  `content/it/...`) work without absolute URLs baked into every layout.

- **`cmd/soffio`**: The CLI entrypoint and site-assembly layer. `main.go` wires flag
  parsing -> corpus load -> visibility filter -> link validation -> template load -> per-doc
  + feed/sitemap/robots/404/manifest generation. `write.go` renders each `ast.Document`
  through `renderer.Render` into an HTML fragment, then executes it through the site's
  `html/template` layout (chosen via the document's `layout` frontmatter key, falling back
  to `layout.html`), passing `Children` (docs whose ID is prefixed by the current doc's ID —
  this is how directory index/listing pages get their child pages) and `Alternates`
  (same-slug docs in other configured languages, used for i18n hreflang links). `template.go`
  embeds the default template set (`cmd/soffio/templates/*.{html,xml,txt,json}`) via
  `go:embed` and overlays any local templates found in `-t` dir of the same name; it also
  registers the `sortBy` template func for sorting `.Children` by an arbitrary frontmatter
  key. Generation of RSS/sitemap/robots/404/manifest is each individually skipped if the
  corresponding named template isn't defined (`tmpl.Lookup(...) != nil`), so adding a
  feature template automatically enables that output.

- **`cmd/preview`**: A trivial static file server (`http.FileServer`) over the output dir
  that auto-opens the default browser; platform-specific browser-open logic lives in
  `open_darwin.go`/`open_linux.go`/`open_windows.go` behind build tags. Every response
  carries a `Soffio-Preview: <abs dir>` header, so a second `preview` of the same dir
  finds the first one when the port is taken, opens the browser on it, and exits 0.
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
- Document IDs double as output paths (`<id>.html`) and as the mechanism for relative link
  resolution and parent/child relationships — treat ID computation (`corpus.Load`) and URL
  resolution (`renderer.resolveURL`) as tightly coupled; changes to one usually require the
  other to stay consistent.
