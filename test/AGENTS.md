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

`test/feedparser/` and `test/feedvalidator/` are the third-party conformance
corpora. They are FETCHED at pinned commits (`scripts/fetch-*.sh`) and are
gitignored — never commit them. They cover breadth; keep new behavioural
cases here in `spec/` instead.

`leniency.tsv` and `nonxml.tsv` are deliberately RED as of the
conformance-2026-08 baseline: they pin XML well-formedness that the engine
currently does not enforce. A red test is an honest test — do not delete or
loosen them to get green.

## Rules

- Prefer adding a fixture here over a one-off in-language assertion when a
  case is expressible as input → output. That is what keeps the two
  runtimes honest against each other.
- TypeScript is canonical. If the two runtimes disagree, the TS behaviour is
  the expected value — unless Go has exposed a genuine TS defect, in which
  case fix TS first and pin the corrected behaviour here.
- A new fixture must pass in BOTH runtimes: run `go test ./...` (from `go/`)
  and `npm test` (from `ts/`) before considering it done.
