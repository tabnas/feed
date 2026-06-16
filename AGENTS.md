# Agents Guide — feed

## What this project is

`@tabnas/feed` is a **feed grammar plugin**: it parses RSS (0.90, 0.91,
0.92, 1.0/RDF, 2.0) and Atom (0.3, 1.0) documents and, by default,
normalizes every dialect into a single **Atom-shaped** result object.
Two other output modes exist:

- `{ format: 'native' }` — the dialect-specific structure (`AtomFeed`,
  `Rss2Feed`, `Rss1Feed`), no cross-dialect normalization.
- `{ format: 'raw' }` — the underlying `XmlElement` tree from
  `@tabnas/xml`, untouched.

It is a grammar plugin for the
[`tabnas`](https://github.com/tabnas/parser) parsing engine, but unlike a
from-scratch grammar it is **built on top of [`@tabnas/xml`](https://github.com/tabnas/xml)**.
The feed plugin contributes **no rules of its own**: it `use()`s the Xml
plugin (which supplies the `xml` / `element` / `content` / `child`
grammar) and then hooks the existing `xml` rule's *before-close* callback
to run feed detection + conversion on the parsed element tree. All the
RSS/Atom knowledge lives in plain helper functions over `XmlElement`, not
in grammar rules. `detect(root)` returns `{ dialect, version }` and is
exported for callers working with `raw` output.

## Repository map

| Path | What it is |
|---|---|
| [`ts/`](ts/) | **Canonical** TypeScript implementation — the `@tabnas/feed` package. Everything lives in `src/feed.ts` (plugin + types + helpers). No CLI. |
| [`go/`](go/) | Go port — module `github.com/tabnas/feed/go`. Plugin + helpers in `go/feed.go`; top-level `const Version` mirrors `ts/package.json`. |
| [`test/specs/`](test/specs/) | Cross-language fixtures: per base name, `<name>.xml` plus `<name>.atom.json` / `<name>.native.json` / `<name>.detect.json` (the last two optional). Both runtimes enumerate this dir. See `test/specs/README.md`. |
| [`test/feedparser-wellformed/`](test/feedparser-wellformed/) | Vendored well-formed feed corpus from kurtmckee/feedparser (BSD 2-Clause), in `atom10/` `atom/` `rss/` `rdf/` subdirs. Both runtimes parse these and assert detection. See `THIRD_PARTY_NOTICES.md`. |
| [`ts/doc/grammar.svg`](ts/doc/grammar.svg) / `grammar.txt` | Railroad diagram of the (xml) grammar, regenerated with `@tabnas/railroad`. |

There is no `package.json` `bin` — this package has no CLI.

## The tabnas dependencies (sibling checkout)

Feed sits two layers up the tabnas stack: it depends on **jsonic** and
**xml**, which in turn depend on **parser**. All are resolved as sibling
checkouts (none of the `@tabnas/*` packages are published yet):

- TypeScript `ts/package.json` `peerDependencies`:
  - `@tabnas/jsonic`: `file:../../jsonic/ts`
  - `@tabnas/xml`: `file:../../xml/ts`
  - `@tabnas/parser`: `">=2"` (the engine; pulled in transitively, also
    listed as a `file:` devDependency for local builds)
  - The same three plus `@tabnas/debug` and `@tabnas/railroad` are
    mirrored as `file:` `devDependencies` so a local `npm install`
    resolves them. (Note: `jsonic` and `xml` use explicit `file:` peer
    specs here rather than the usual `">=2"` — keep that as-is.)
- Go `go/go.mod` requires `github.com/tabnas/jsonic/go` and
  `github.com/tabnas/xml/go` with `replace` directives pointing at
  `../../jsonic/go` and `../../xml/go`. It does **not** require
  `parser/go` directly — that comes transitively through jsonic and xml.

Clone the transitive closure as siblings of this repo and build their TS
first (`cd <dep>/ts && npm install && npm run build`), then work here. CI
clones and builds them all in order (see below).

Both test suites construct a parser as **jsonic + Feed**, not parser +
Feed: the feed plugin pulls in Xml, and Xml/feed expect jsonic's lexer.

- TS: `new Tabnas().use(jsonic).use(Feed)` (or `.use(Feed, { format })`).
- Go: `j := jsonic.Make(); j.UseDefaults(feed.Feed, feed.Defaults, opts)`.

## Authority and alignment rules

1. **TypeScript is canonical.** When TS and Go disagree on parse or
   normalization behavior, TS wins; change Go to match, and add or extend
   a shared fixture when the behavior is expressible as input → output.
2. The shared fixtures in `test/specs/*` are the **parity contract**.
   Both suites enumerate the directory, parse each `.xml` with the
   matching `format` option, and deep-equal the result against the
   expected JSON after a JSON marshal/unmarshal round-trip (which
   normalizes property ordering and types). Add a spec by dropping in the
   `.xml` plus the expected `.json` file(s); both languages pick it up
   automatically. Keep each spec minimal — one behavior per fixture.
3. `detect` (TS) / `Detect` (Go) are part of the contract: the
   `<name>.detect.json` fixtures pin `{ dialect, version }` and both
   runtimes must agree. The dialect set is `atom` / `rss` / `rdf` /
   `unknown`; the version set is the `FeedVersion` union (`atom10`,
   `atom03`, `rss20`, `rss092`, `rss091u`, `rss091n`, `rss10`, `rss090`,
   `unknown`).
4. The feed plugin layers on Xml — don't fold feed-specific logic into
   grammar rules. The only rule touched is the existing `xml` rule, via
   `tn.rule('xml', rs => rs.bc(...))`. All RSS/Atom mapping stays in
   `XmlElement` helpers so the grammar remains pure XML.

## Feed-specific gotchas

- **No new grammar rules.** The plugin adds zero rules; it hooks the
  `xml` rule's before-close. So `debug.model()` reports the **xml**
  grammar's rule set — `['child', 'content', 'element', 'xml']` — and
  `m.config.start === 'xml'`, not anything feed-named. The
  `debug-model.test.ts` assertions encode exactly this, plus that
  `m.plugins` lists both `Feed` and `Xml`.
- **The bc guard is load-bearing.** `xml`'s before-close fires more than
  once (the `xml` rule recurses to consume trailing whitespace). The hook
  only runs conversion when an element was actually parsed *this*
  iteration (`rule.child.node` is set) and the engine root holds an
  element — mirroring `@xml`'s own `@xml-bc` guard so conversion happens
  exactly once. Don't drop that check.
- **Conversion pipeline:** `convert()` → `detect()` → parse to the native
  shape (`parseAtom` / `parseRss2` / `parseRss1`) → for the default
  `atom` format, `rss2ToAtom` / `rss1ToAtom` map onto the Atom shape.
  `raw` returns the `XmlElement` before any of this; `native` stops after
  the native parse.
- **Unrecognized roots throw.** A root element that is not `feed`, `rss`,
  or `RDF` raises `feed: unrecognized root element ...` (covered in
  `feed.test.ts` and the Go suite). `raw` format never reaches that path.
- **Xml `Plugin` type bridge.** `Xml` is still typed against jsonic's
  legacy `Plugin` signature, so `feed.ts` casts it
  (`tn.use(Xml as unknown as Plugin)`). The two are runtime-compatible;
  the cast is intentional, not a smell to "fix".
- **CommonJS at runtime.** The package compiles to CommonJS (tsconfig
  `module=nodenext`, no `"type":"module"`), so `require` is available;
  `debug-model.test.ts` relies on that to resolve `@tabnas/debug`
  dynamically.

## Build & test

TypeScript (package in `ts/`):

```bash
cd ts && npm install && npm run build   # tsc --build src test
cd ts && npm test                       # node --test over dist-test/*.test.js
```

Go (module in `go/`):

```bash
cd go && go build ./...
cd go && go test -v ./...               # runs test/specs + feedparser-wellformed
```

Or via the top-level `Makefile` (ts canonical, go tracks it):

```bash
make build        # build-ts then build-go
make test         # test-ts then test-go
make reset        # ts npm reset + go clean/build/test
```

`make publish-go V=x.y.z` seds `const Version` in `go/feed.go`, commits,
tags `go/vX.Y.Z`, and (when `gh` is present) cuts a release.

## Tests

- `ts/test/specs.test.ts` / `go/feed_test.go` drive the shared
  `test/specs/` fixtures across the three formats (`atom`, `native`, plus
  `detect`).
- `ts/test/feedparser.test.ts` / the Go equivalent run the vendored
  `test/feedparser-wellformed/` corpus and assert dialect/version
  detection per subdir.
- `ts/test/feed.test.ts` covers TS-only behavior: error paths, `raw`
  mode, and plugin registration shape.
- `ts/test/doc-examples.test.ts` runs the fenced examples from the
  README files (`README.md`, `ts/README.md`, `go/README.md`).
- `ts/test/debug-model.test.ts` is the optional `@tabnas/debug`
  composition test: it resolves the debug plugin dynamically and **skips**
  unless `@tabnas/debug` is installed (a `file:` devDependency) or
  `TABNAS_DEBUG_PATH` points at a built checkout. It asserts the
  structured grammar model (rule set, `config.start`, plugin list, push
  edges) described under gotchas above.

## CI

`.github/workflows/build.yml` has two jobs:

- **build** (Linux/macOS/Windows, Node 24): sets
  `git config --global core.autocrlf false` (CRLF would corrupt fixtures), then
  clones the full tabnas closure as siblings — `parser debug json abnf
  railroad jsonic xml` — `npm i && npm run build` each `*/ts` in that
  order, and finally `npm test` in `feed/ts`.
- **build-go** (Linux/macOS, Go 1.24): clones the same siblings, then
  reconstructs the workspace by mirroring `admin/scripts/link.sh` —
  creating `vendor/` symlinks for any `../vendor/`-style replaces and a
  `go.work` over every module except the vendor-replaced ones — then
  `go build ./...` and `go test -v ./...` in `feed/go`.
