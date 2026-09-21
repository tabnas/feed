# Agents Guide — shared spec fixtures

`spec/*.tsv` holds the cross-runtime conformance fixtures. All three
runtimes auto-discover and run **every** file in this directory, so a
change here affects TypeScript, Go and Rust together — edit with that in
mind.

These replaced the old `test/specs/` directory of `.xml` + `.json` file
pairs: one mechanism, in one format, instead of two.

## Format

Tab-separated, one case per line, with a header row naming the columns.
Blank lines are skipped, and so are comment lines — a line starting with
`#` that contains no tab. (A data row always has at least one tab, so a
`#`-leading source such as a C preprocessor directive still works.)

| Column | Meaning |
|---|---|
| `input` | Feed XML source. Escapes `\n` `\r` `\t` `\\` are decoded. |
| `expected` *or* `detect` | See below. |
| `opts` | Optional JSON object of plugin options (empty means defaults). |

The **second column's header name selects what the runner compares**:

- `expected` — the parse result, as a JSON value, or `ERROR` /
  `ERROR:<substring>` for inputs that must fail. Unlike most of the fleet
  the text after the colon is a fragment of the MESSAGE, not an error
  code: these rejections come from the feed layer's own validation, which
  says what is wrong in prose rather than through a code the engine
  assigns. A bare `ERROR` accepts any failure.
- `detect` — the `{ dialect, version }` report for the input, computed from
  the raw element tree. The runner forces `{"format":"raw"}` for these files
  and ignores the `opts` column.

`expected` / `detect` / `opts` are **not** escape-decoded — they are raw
JSON, so JSON's own escape rules apply. To put a literal backslash in
`input`, write `\\`.

Results are compared after a JSON round-trip, so property order and the
class identity of the feed structs do not affect the comparison.

## Who runs what

- TypeScript: `ts/test/parity.test.ts` — a `makeRunner(...)` per fixture.
- Go: `go/parity_test.go` — a `support.Runner{...}` per fixture.
- Rust: `rs/tests/parity_test.rs` — a `tabnas_support::Runner` per fixture.

One runner per FILE, not one over the directory, because the second
column's header (`expected` or `detect`) says what the file asserts. All
three hold only what is specific to feed: that, how to build the parser for a
row's options, and the message matching above. Everything else — finding `test/spec`,
reading the file, decoding escapes, the `ERROR:` contract, the comparison,
the `<file>:<line>` in a failure message — comes from
[`@tabnas/support`](https://github.com/tabnas/support) and its Go and Rust
halves, so the three loaders cannot drift from each other either.

All three discover files by directory listing: adding a `.tsv` here runs it in
every runtime without touching any runner. An empty fixture, and a spec
directory with no fixtures in it, both **fail** — a runner that reports
green having run nothing is indistinguishable from coverage that was never
there.

## The divergence register is NOT in this directory

[`../test/divergent.tsv`](../test/divergent.tsv) records the rows where a
port disagrees, with one column per runtime, and it sits one level up on
purpose: every parity runner enumerates `test/spec` and reads the SECOND
column header to decide what a file asserts, so a register dropped in
here would be refused by all three of them.

No row is recorded there today: the three ports agree on every input
this repository measures, so every row of every file in `spec/` runs and
passes in all three runtimes, and the Rust parity runner steps over
nothing. The register keeps its header and is still executed by
`rs/tests/divergent_test.rs`, which names the empty state rather than
skipping. The two rows of [`spec/xml-layer.tsv`](spec/xml-layer.tsv) that
were recorded there until 2026-09-21 came back the day `@tabnas/xml`
repaired the Rust message they pin. See
[`../DIVERGENCE.md`](../DIVERGENCE.md).

`test/feedparser-wellformed/` is a separate, larger third-party corpus used
for smoke coverage (`TestCorpus*` / `feedparser.test.ts`), not for pinning
exact output — keep new behavioural cases here in `spec/` instead.

`test/feedvalidator/` and `test/feedparser/` are the FULL third-party
conformance corpora. They are **fetched** at a pinned commit by
`scripts/fetch-corpus.mjs` (via `make fetch`, `ts/` `npm pretest`, or the Go
harness on demand) into gitignored directories — never commit them. They
cover breadth; keep new behavioural cases here in `spec/` instead.

Each corpus has a runner in both runtimes, and neither may `skip`: if the
corpus is missing they fail with fetch instructions.

| Corpus | TypeScript | Go |
|---|---|---|
| `feedvalidator/` | `ts/test/feedvalidator.test.ts` | `TestFeedValidatorConformance` |
| `feedparser/` | `ts/test/feedparser-conformance.test.ts` | `TestFeedParserConformance` |

The feedparser runner checks the VALUE each document parses to, against the
upstream `Expect:` annotation the corpus itself carries (`ts/test/
expect-eval.ts` and its Go mirror). "It did not throw" is not a conformance
result. Most of that suite is at 100% and asserted exactly, but the value
check is a **ratchet**: `VALUE_CORRECT_FLOOR` / `feedParserValueCorrectFloor`
record what currently holds, and the denominator is floored too so value
checks cannot be dropped to improve the ratio. Raise both when the parser
improves; never lower either to get green. The two small sets of known
disagreements (`ACCEPTED` / `feedParserAcceptedIllformed`, and
`VERSION_KNOWN_WRONG` / `feedParserVersionKnownWrong`) are asserted to be
EXACT — fixing one of those cases is red until its entry is deleted.

## Rules

- Prefer adding a fixture here over a one-off in-language assertion when a
  case is expressible as input → output. That is what keeps the two
  runtimes honest against each other.
- TypeScript is canonical. If the runtimes disagree, the TS behaviour is
  the expected value — unless another port has exposed a genuine TS defect,
  in which case fix TS first and pin the corrected behaviour here.
- A new fixture must pass in EVERY runtime: run `go test -count=1 ./...`
  (from `go/`), `npm test` (from `ts/`) and `cargo test --all-targets`
  (from `rs/`) before considering it done.
- A row that a port cannot satisfy goes in `../test/divergent.tsv` with a
  column per runtime, and is argued in `../DIVERGENCE.md`. It does not get
  weakened here, and it does not get skipped.
