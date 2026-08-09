# Agents Guide — shared spec fixtures

`spec/*.tsv` holds the cross-runtime conformance fixtures. Both runtimes
auto-discover and run **every** file in this directory, so a change here
affects TypeScript and Go together — edit with that in mind.

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
  `ERROR:<substring>` for inputs that must fail.
- `detect` — the `{ dialect, version }` report for the input, computed from
  the raw element tree. The runner forces `{"format":"raw"}` for these files
  and ignores the `opts` column.

`expected` / `detect` / `opts` are **not** escape-decoded — they are raw
JSON, so JSON's own escape rules apply. To put a literal backslash in
`input`, write `\\`.

Results are compared after a JSON round-trip, so property order and the
class identity of the feed structs do not affect the comparison.

## Who runs what

- TypeScript: `ts/test/parity.test.ts` — reads `../../test/spec` at runtime
  from `dist-test/`, one `describe` per file.
- Go: `go/parity_test.go` — `TestSpec` globs `../test/spec/*.tsv`.

Both discover files by directory listing: adding a `.tsv` here runs it in
both runtimes without touching either runner.

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
- TypeScript is canonical. If the two runtimes disagree, the TS behaviour is
  the expected value — unless Go has exposed a genuine TS defect, in which
  case fix TS first and pin the corrected behaviour here.
- A new fixture must pass in BOTH runtimes: run `go test ./...` (from `go/`)
  and `npm test` (from `ts/`) before considering it done.
