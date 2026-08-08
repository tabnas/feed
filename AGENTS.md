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
| [`test/spec/`](test/spec/) | Shared `.tsv` conformance fixtures. Both runtimes auto-discover this dir; the header row's second column name selects what is compared (`expected` = the parse result, `detect` = the dialect report). See [`test/AGENTS.md`](test/AGENTS.md). |
| [`test/feedparser-wellformed/`](test/feedparser-wellformed/) | Vendored well-formed feed corpus from kurtmckee/feedparser (BSD 2-Clause), in `atom10/` `atom/` `rss/` `rdf/` subdirs. Both runtimes parse these and assert detection. See `THIRD_PARTY_NOTICES.md`. |
| `test/feedvalidator/`, `test/feedparser/` | The full third-party conformance corpora, **fetched at a pinned commit and gitignored — never committed**. `make fetch` (or `scripts/fetch-feedvalidator.sh` / `scripts/fetch-feedparser.sh`) populates them. |
| [`scripts/fetch-corpus.mjs`](scripts/fetch-corpus.mjs) | The fetcher, holding the pinned upstream SHAs. The two `.sh` wrappers are thin `exec`s over it so `npm pretest` works on Windows CI too. |
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
2. The shared fixtures in `test/spec/*.tsv` are the **parity contract**.
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
- **`strictNamespaces` defaults ON here, unlike `@tabnas/xml`.** `feed`
  installs Xml with `{ strictNamespaces: true }`, so an element or attribute
  using an undeclared prefix (`<dc:language>` with no `xmlns:dc`) is an
  error. `@tabnas/xml` defaults it off on purpose — bare XML 1.0
  well-formedness does not require namespace well-formedness — but feeds are
  namespace-defined formats: Atom *is* its namespace, RSS 1.0 is RDF, and
  every RSS 2.0 extension (`dc:`, `content:`, `sy:`, `georss:`) is a
  prefixed name, so an unbound prefix is a typo or a truncation, not an
  extension to pass through. The W3C Feed Validation Service agrees: it is
  worth exactly +6 must-reject documents in the feedvalidator corpus and +1
  in feedparser's `illformed/`, at a cost of zero well-formed documents.
  Callers who want the bare-XML behaviour pass `{ strictNamespaces: false }`.
  Keep the two runtimes' defaults in step — TS `withDefaults`, Go `Defaults`.
- **Xml `Plugin` type bridge.** `Xml` is still typed against jsonic's
  legacy `Plugin` signature, so `feed.ts` casts it
  (`tn.use(Xml as unknown as Plugin, { … })`). The two are runtime-compatible;
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
cd go && go test -count=1 -v ./...      # spec + feedparser-wellformed + feedvalidator
```

**Always pass `-count=1` to the Go suite.** `test/spec/*.tsv`,
`test/feedparser-wellformed/` and the fetched corpora all sit ABOVE the Go
module root, so Go does not record them as test inputs. Without `-count=1`,
editing a shared fixture replays a cached `ok ... (cached)` and the new rows
never run — a green tick that proves nothing. The `test-go` Makefile target
passes it.

**`GOWORK=off` is not the same run.** `go test` from `go/` picks up the
repo-set `go.work` and resolves `github.com/tabnas/xml/go` to the sibling
checkout; `GOWORK=off go test` resolves the last *published* module. Confirm
which you are in with `go list -m github.com/tabnas/xml/go` — a bare module
path means sibling, a path with a version means published. An unpublished
`xml` fix is invisible to the `GOWORK=off` run, so a green `GOWORK=off` suite
proves nothing about it (and today an unpublished `xml` fix is exactly what
the feedvalidator harness depends on — see "Conformance" below).

Or via the top-level `Makefile` (ts canonical, go tracks it):

```bash
make fetch        # third-party corpora at their pinned SHAs (idempotent)
make build        # build-ts then build-go
make test         # fetch, then test-ts then test-go
make reset        # ts npm reset + go clean/build/test
```

`make test` depends on `fetch`, `ts/package.json` has it as `pretest`, and
the Go harness re-runs the fetcher itself if the corpus is missing — three
independent paths, because a conformance suite that silently does not run is
worse than no suite at all.

`make publish-go V=x.y.z` seds `const Version` in `go/feed.go`, commits,
tags `go/vX.Y.Z`, and (when `gh` is present) cuts a release.

## Tests

- `ts/test/parity.test.ts` / `go/parity_test.go` drive the shared
  `test/spec/*.tsv` fixtures across the three formats (`atom`, `native`, plus
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
  edges) described under gotchas above. In a normal checkout the
  devDependency resolves, so it runs — a run reporting `skipped 0` is the
  expected state.
- `ts/test/perf.test.ts` / `go/perf_test.go` assert that reusing a parser
  instance is much faster than rebuilding one per parse.

- `ts/test/feedvalidator.test.ts` / `TestFeedValidatorConformance` in
  `go/conformance_test.go` run the **whole** `rubys/feedvalidator`
  `testcases/` tree and assert both halves — must-reject and must-accept,
  plus dialect detection. The two are line-for-line equivalents; a TS/Go
  divergence shows up as one going red. See "Conformance" below.

**No test may silently not-run.** The `feedparser-wellformed` corpus is
vendored, so it can never legitimately be absent: `loadDir` (TS) and
`requireWellformed`/`corpusFiles` (Go) throw or `t.Fatal` on a missing or
empty directory rather than skipping. Previously they returned an empty set,
which made every `assert.deepEqual(fails, [])` pass vacuously. The fetched
corpora get the same treatment: `requireCorpus` throws (TS, at import time)
or `t.Fatal`s after attempting a fetch (Go), and each harness additionally
asserts the corpus is not truncated (`1000 < FILES.length`). Neither ever
`skip`s. Likewise `doc-examples.test.ts` fails when a fenced block carries
`// =>` but yields no extracted assertion, instead of dropping it. `make
test` is expected to report `skipped 0` and zero Go `SKIP` lines.

## Conformance: what is actually verified

There is no single canonical RSS/Atom conformance suite (RSS 0.9x/2.0 has no
formal test suite at all). The two authoritative third-party corpora are
`rubys/feedvalidator` (the suite behind the W3C Feed Validation Service) and
`kurtmckee/feedparser`.

**The feedvalidator corpus is wired into `make test`** — the whole
`testcases/` tree, both halves asserted, in both runtimes
(`ts/test/feedvalidator.test.ts` and `go/conformance_test.go`, which classify
and assert identically). It is fetched, not vendored, so `make test` runs
`make fetch` first and both harnesses fail loudly rather than skip when the
corpus is absent.

| Corpus | Measure | Result |
|---|---|---|
| rubys/feedvalidator `testcases/` | not-well-formed docs rejected | **18/18** (was 16/18) |
| rubys/feedvalidator `testcases/` | well-formed RSS/Atom docs accepted | **1809/1809** (was 1796/1809) |
| rubys/feedvalidator `testcases/` | detected dialect matches the corpus directory | **1108/1108** (was 1107/1108) |

"Was" is the same harness run against the last **published** `@tabnas/xml`
(`v0.4.1`), which is what `GOWORK=off go test ./...` still resolves. All 16
distinct files behind those three "was" numbers were XML-layer, not
feed-layer, and every one is now fixed in the `xml` sibling:

- 7 rejected for a UTF-8 BOM before `<?xml` (`unexpected character(s): <`);
- 5 rejected `undeclared_entity` behind an unread external DTD subset, which
  XML 1.0 §4.1 WFC *Entity Declared* permits;
- 2 rejected `unbound_prefix` where the declaration and the use are sibling
  attributes on the same element (`xmlns:xsi` + `xsi:…`) — §5.2 scopes the
  declaration over them, so binding must not depend on attribute order. One
  of the two is the single detect miss;
- 2 under-rejections: the uppercase-`X` character reference `&#X26;` (XML 1.0
  [66] admits only lowercase `&#x`) and a namespace name containing a
  newline.

The mismatched-tag message no longer leaks its `$fsrc`/`$openname`
placeholders either — under `GOWORK=off` it still reads `closing tag
</$fsrc> does not match opening tag <$openname>`, which is the cleanest
one-line demonstration of which module you resolved.

Those behaviours are pinned row-by-row, in both runtimes, in
[`test/spec/xml-layer.tsv`](test/spec/xml-layer.tsv) — the cheap
proof-of-fix, without needing the corpus.

**Until `@tabnas/xml` publishes, the two CI jobs resolve DIFFERENT `xml`, and
only the Go one is green.** They are not interchangeable, so read the job name
before believing a red tick:

- **`go` job — passes.** `polyglot-ci` clones the sibling repos and runs
  `go work use` over them, so `xml/go` resolves to `xml`'s **main**, fixes
  included. The `require github.com/tabnas/xml/go v0.4.1` in `go/go.mod` is
  overridden by the workspace and never fetched. (`GOWORK=off go test ./...`
  locally is a different run — that one resolves the published module and
  fails, which is the point of the section above.)
- **`ts` job — fails, expected.** The workflow clones and builds the sibling
  TS packages but never links them into this repo's `node_modules`, and
  `ts/package.json` asks for `"@tabnas/xml": "*"` with `ts/package-lock.json`
  pinning the published `0.4.1`. So `npm i` fetches the **registry** build and
  the suite runs against unfixed `xml`. Measured there: `16/18` must-reject,
  `1796/1809` must-accept, `1107/1108` detect, and 7 of the 9
  `TestSpec/xml-layer.tsv` / `parity.test.ts` rows red — i.e. exactly the
  "was" column above.

This clears when `xml` publishes: bump the `require` in `go/go.mod` and the
lockfile, then delete this paragraph. Do **not** "fix" it here by reverting an
`xml-layer.tsv` row, skipping the conformance suites, or repointing a
devDependency at a local path — all three trade a real signal for a green
tick.

The 18-document must-reject set is the 14 that upstream annotates
`Expect: SAXError` plus 4 that are objectively not well-formed but carry an
`Expect:` naming a validator-level diagnostic instead. Those 4 are listed by
path, with the specific violation quoted, in `NOT_WELL_FORMED` /
`notWellFormed` in the two harnesses — reclassified, not excused, and the set
is asserted to be exactly those 4 so a fifth cannot be added silently.

The `kurtmckee/feedparser` corpus is fetched by the same `make fetch` but only
its 48-file vendored subset (`test/feedparser-wellformed/`) is asserted today.
Measured over the full tree with the default parser:

| Corpus | Measure | Result |
|---|---|---|
| kurtmckee/feedparser `wellformed/` | RSS/Atom-rooted docs parse to an Atom shape | 1734/1734 (100%) |
| kurtmckee/feedparser `illformed/` | docs annotated `Expect: bozo` are rejected | 5/19 whole dir; 4/14 of the annotated ones |

Those two rows are **measured, not asserted** — there is no harness behind
them yet. Wiring one (including the value-level `Expect:` evaluator, where the
real number is far worse than the parse-vs-error number) is the open Phase-2
work; a draft lives on the `conformance-2026-08` branch. Do not quote the
first row as a conformance result without saying it is unasserted.

### Still broken, and not ours to fix

`@tabnas/xml` accepts a document with **no document element at all** —
`<?xml version="1.0"?><!-- c -->` parses to `undefined` instead of raising.
XML 1.0 §2.1 is `document ::= prolog element Misc*`: exactly one element is
required. This is distinct from the trailing-content leniency that *was*
fixed (`…</rss><extra/>` and `…</rss>junk` are both rejected now). It costs
one file, `test/feedparser/illformed/rss_empty_document.xml`, and nothing in
feedvalidator. Repro:

```js
new Tabnas().use(jsonic).use(Xml).parse('<?xml version="1.0"?><!-- c -->')
// returns undefined; should throw
```

Fix belongs in `xml`, not here.

Out of the claim, and deliberately not asserted: 333 feedvalidator files whose
document element is not `feed`/`rss`/`RDF` (KML, OpenSearch, OPML, RSS 1.1
`Channel`, APP `service`, XRDS, bare `entry`), and feedparser's 8 `chardet/`
cases, which need byte-level encoding sniffing — `@tabnas/feed` takes a
`string`, so encoding detection is the caller's job.

## CI

The old per-repo `.github/workflows/build.yml` is gone. CI is now a thin
caller: `.github/workflows/ci.yml` (push/PR on `main`) delegates to the org
workflow `tabnas/.github/.github/workflows/polyglot-ci.yml@main`, passing the
sibling closure it must clone and build first:

```yaml
with:
  deps: "parser debug json abnf railroad jsonic xml"
```

`.github/workflows/release.yml` builds and publishes. Session credentials
cannot write `.github/workflows/*` — changes there are promoted by a
maintainer via `tabnas/admin` `rollout/apply-ci-folders.sh` (admin
`DECISIONS.md` ADR-8), so edit the org workflow, not this repo.
