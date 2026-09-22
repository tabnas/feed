# Agents Guide: rs/

The Rust port of the canonical TypeScript in [`../ts`](../ts). Read
[`../AGENTS.md`](../AGENTS.md) first: it holds the cross-runtime rules,
the option names and defaults, the dialect vocabulary and the shape of
the parsed feed. This file covers only what is specific to this crate.

## Layout

| Path | |
|---|---|
| `src/lib.rs` | everything: `FeedOptions`, `detect`, `convert`, the dialect parsers, the native-to-Atom mapping, `feed`, `plugin`, `make`, `make_with`, `parse` |
| `tests/parity_test.rs` | every `../test/spec/*.tsv` fixture, plus the recorded-divergence exemption list and the named-column census |
| `tests/conformance_test.rs` | both FETCHED corpora, `../test/feedvalidator/` and `../test/feedparser/`, including the `Expect:` evaluator |
| `tests/divergent_test.rs` | `../test/divergent.tsv`, through `tabnas_support::Register` |
| `tests/feed_test.rs` | in-language cases mirrored from `ts/test/feed.test.ts`, `ts/test/feedparser.test.ts` and `go/feed_test.go`, plus the JavaScript classes, the integer reader and the untrusted-input bounds |
| `tests/debug_model_test.rs` | the composition test, mirrored from `ts/test/debug-model.test.ts` |
| `tests/perf_test.rs` | reusing an instance beats rebuilding one, mirrored from `go/perf_test.go` |
| `tests/version_test.rs` | `Cargo.toml`, `VERSION`, `ts/package.json` and `go/feed.go` must agree |
| `tests/common/mod.rs` | the parsers, the value normaliser and the fixture runner the suites share |
| `README.md` | the crate front page, doctested, and in the gated prose set |

## Four crates by path

`Cargo.toml` takes `tabnas` (`../../parser/rs`), `tabnas-jsonic`
(`../../jsonic/rs`, which brings `tabnas-json`), `tabnas-xml`
(`../../xml/rs`) and, as dev-dependencies, `tabnas-support`
(`../../support/rs`, feature `serde_json`) and `tabnas-debug`
(`../../debug/rs`). None is published. Clone them as siblings before
running cargo, and expect `Cargo.lock` to move when one bumps its
version: `../ci/rust/run.sh` exempts exactly those entries when it diffs
the lock, and asserts everything else.

## There is no grammar here

The plugin contributes NO rules. It installs `tabnas_xml` with
`strict_namespaces` from its own options and then adds one before-close
callback to the existing `xml` rule, which is where the conversion runs.
`tests/debug_model_test.rs` holds that to the structured grammar model:
the rule set is the xml crate's `child`, `content`, `element`, `xml`, the
start rule is `xml`, and the plugin list names both plugins. Adding a
rule here would be a change of design, not a refactor.

## The before-close guard is what makes it run once

The `xml` rule's before-close fires more than once, because `r: xml`
recurses to consume trailing white space. Two guards keep the conversion
to exactly one run, and both are needed:

1. `rule.child_node.is_undefined()` returns early unless an element was
   parsed in THIS iteration, which mirrors the guard in the xml crate's
   own `@xml-bc`, so the conversion happens on the same iteration that
   hook copied the element to the document node.
2. The node is re-checked with `is_element`. Once converted it is a feed
   object with no `children`, so a second pass is a no-op rather than a
   second conversion.

The result is written through `*rule.node.borrow_mut()`, deliberately,
because the start rule's cell IS the document node. This is the one
place the usual warning about writing through a shared node cell does
not apply; see `../../xml/rs/AGENTS.md` for the general form of that
hazard.

## The JavaScript character classes, spelled out

Two of them, both reachable from document content, both wrong by
default in Rust:

- **`String.prototype.trim` is not `str::trim`.** ECMA-262 white space
  has U+FEFF and has not U+0085; the Unicode `White_Space` property that
  `str::trim` and `char::is_whitespace` use is the reverse. Every text
  value in a feed is trimmed, so a title wrapped in byte-order marks and
  a title wrapped in next-line controls both come out differently.
  `JS_SPACE` carries the class body and `js_trim` is the trimmer.
- **`\s` and `\S` in the RSS author pattern.** The `regex` crate reads
  them as the same Unicode property, so the pattern is built from
  `JS_SPACE` rather than from the shorthands. The pattern runs over a
  `managingEditor`, `webMaster` or item `author` value, which the
  document supplies.

`ported_patterns_use_the_javascript_character_classes` is spelled here as
`trimming_uses_the_javascript_white_space_class`,
`the_author_pattern_uses_the_javascript_classes` and
`element_text_is_trimmed_the_javascript_way`, and each asserts both
directions: what the JavaScript class does, and that the Rust default
disagrees.

## `parseInt`, and the number formatter that is NOT needed here

`js_parse_int` is ECMA-262 `parseInt(text, 10)`: leading white space
(the same class again), an optional sign, then the longest run of ASCII
digits. `"12abc"` is 12, `"0x10"` is 0, and a string with no digit is
NaN. It is neither `str::parse` nor Go's `strconv.Atoi`, and the Go port
uses `strconv.Atoi`, so the two disagree on a trailing-garbage value;
TypeScript is canonical and this crate follows it.

The NaN matters: the canonical plugin puts it straight into the feed, and
`JSON.stringify` renders it as `null`. `Value::to_json` renders a NaN as
`null` too, so the two agree without special handling.

**The JavaScript `Number`-to-`String` algorithm does not arise in this
crate, and no copy of it belongs here.** That defect class has been found
in six crates of this fleet, so it was checked for rather than assumed
absent: the only direction this port converts is text to number. The one
place a value is rendered back into text is `serialize_element`, for
xhtml and html bodies, and the element tree from `tabnas-xml` holds only
strings, so no number ever reaches it. Should that change, copy
`js_number_to_string` from `../../csv/rs/src/lib.rs` rather than writing
a seventh version.

## Absent is not empty

The canonical plugin sets an absent field to `undefined` and the fixture
comparison drops it, so this crate simply never inserts the key. That
makes three distinctions load-bearing, and each has its own helper:

- `attribute` returns the value whenever the ATTRIBUTE EXISTS, empty
  included. It is what `scheme`, `label`, `uri`, `version`, `domain`,
  `url` and `rdf:about` are read with, because the canonical code
  assigns those rather than guarding them: `category scheme=""` carries
  an empty `scheme` into the result.
- `attribute_truthy` returns it only when it is non-empty, which is the
  `if (a.x)` form and is what `rel`, `type`, `length`, `src` and the
  rest use.
- `find_child(...).is_some()` is presence of the ELEMENT, which is what
  the RSS channel and item fields test, so `<title></title>` produces an
  empty title rather than none.

Getting one of these wrong shows up as a spurious key or a missing one
in a shared fixture, which is exactly what those fixtures are for.

## Recorded divergences, and why there is no exemption list

There is none. Every row of every shared fixture runs in this port and
passes, `../test/divergent.tsv` holds no rows, and `tests/parity_test.rs`
skips nothing.

Two rows of `../test/spec/xml-layer.tsv` used to be exempt: they pin the
rendered MESSAGE of a namespace rejection, and the Rust `tabnas-xml`
crate raised that rejection down a path that never reached the message
template, so the rendered text was the stand-in `namespace resolution
failed`. That was repaired in `tabnas/xml` (rs). The register then went
RED, which is the mechanism working: a divergence that has been repaired
fails as loudly as one that has regressed, and the row cannot outlive it.
Both halves came out together, the two register rows and the exemption
list in `tests/parity_test.rs`.

A row this port cannot satisfy goes in `../test/divergent.tsv` with a
column per runtime and an argument in `../DIVERGENCE.md`; nothing is
skipped in the parity suite to make a red row green. A row that fails for
any other reason is a defect in this port.

## Depth is not this crate's to bound, and it has one anyway

Nesting costs super-linear time in the layer below, measured on a release
build of this stack through the `raw` format, which this crate does not
touch at all: 100 levels in 2.6 ms, 400 in 8.9 ms, 800 in 20 ms, 1,600 in
67 ms and 3,200 in 415 ms. `Value::to_json` and dropping a `Value` also
recurse. Both belong to `tabnas-xml` and the engine; this crate adds no
depth of its own and sets no budget of its own, because doing so here and
not in `tabnas-xml` would make the two disagree about what documents they
accept.

That decision has a price, and the price is measured rather than
assumed. On the default 8 MiB main-thread stack, a release build of this
stack parses a document nested about 8,300 elements deep and returns it
under the `raw` format, and `Value::to_json` over that result then
overflows the stack and ABORTS the process. The default `atom` format
survives further, because the value it hands back is small, and aborts
inside `parse` itself at about 20,000 levels. An abort is not a `Result`,
so a caller reading documents from strangers has to cap input size and
nesting depth before the parse; `README.md` says so under "Untrusted
input", with the same numbers.

Nesting is not the only amplifier, and the other one is sharper.
`tabnas-xml` expands DOCTYPE entity definitions with no ceiling on the
result: a 569-byte document whose entities nest nine deep reaches
`tabnas_xml::entity::EntityDecoder::expand` asking for 6.2 GB in one
allocation, and the allocation failure aborts the process. `XmlOptions`
has no expansion limit to set, and neither `FeedOptions` here nor the
canonical `FeedOptions` exposes xml's `entities` switch, so there is
nothing this crate can cap. The repair belongs in `tabnas/xml`;
`README.md` tells a caller to limit input size and process memory in the
meantime.

`hostile_input_returns_rather_than_aborting` and
`a_deep_tree_that_parses_can_be_walked_and_dropped` pin what is true
rather than the wider claim: up to `WALKABLE_DEPTH`, on a stack the test
sizes itself, whatever the parser accepts the caller can walk and drop.
Raising that constant past the measured bound would not be a stronger
test, it would be a test that aborts the runner.

## The corpus must never skip

`../test/feedparser-wellformed/` is VENDORED, so it can never
legitimately be absent. `corpus_files` panics on a missing or empty
directory rather than skipping, exactly as `requireWellformed` fails in
Go and `loadDir` fails in TypeScript. A suite that reports green having
run nothing is indistinguishable from coverage that was never there.

The two FETCHED corpora (`../test/feedvalidator/` and
`../test/feedparser/`) run here too, in `tests/conformance_test.rs`. They
are gitignored rather than committed, so that file fetches a missing one
itself by shelling out to `node ../scripts/fetch-corpus.mjs` and then
FAILS if it is still absent: `cargo test` has no pretest hook any more
than `go test` does, which is how a suite ends up never running. The
consequence for the gate is that a checkout which has not fetched yet
needs node and network once; `../ci/rust/run.sh` says so.

The numbers this port reports are the numbers the other two report, which
is the point of having a third harness rather than a second one:
18/18 must-reject, 1809/1809 must-accept and 1108/1108 dialect over
feedvalidator; 1734/1734 parsed, 1734/1734 dialect, 9/14 version, 6/19
ill-formed rejected and 375 of 1360 value assertions over feedparser. The
two enumerated sets and the two value floors are kept identical to the
TypeScript and Go twins, so a divergence goes red on one side instead of
becoming a third baseline.

The value row is a RATCHET, not a pass line. Raise
`FEEDPARSER_VALUE_CORRECT_FLOOR` in all three twins in the same commit
when a repair improves the number; never lower it, and never lower the
checked floor, which exists because dropping value checks would improve
the ratio.

Corpus files are read LOSSILY here (`String::from_utf8_lossy`). Several
feedvalidator cases are not valid UTF-8 on purpose, and the other
runtimes see them decoded: Node substitutes U+FFFD and Go's `string(b)`
yields U+FFFD per invalid byte as the parser walks it. A strict read
would make this port disagree with both over file IO rather than over
parsing.

## Running it

```bash
cd rs
CARGO_TERM_COLOR=never cargo test --all-targets && cargo test --doc
cargo clippy --all-targets --all-features -- -D warnings
cargo fmt --check
```

Or `make test-rs` from the repository root, and `ci/rust/run.sh` for what
CI would say.

## The README is doctested and gated

`src/lib.rs` includes `../README.md` under `#[cfg(doctest)]`, so every
`rust` fence in it runs as a doctest and a stale example fails the gate.
Each fence must therefore be a COMPLETE program: wrap it in
`fn main() -> Result<(), Box<dyn std::error::Error>> { ... Ok(()) }`
rather than using `?` at the top level, and never use hidden `# ` lines,
which render as garbage on GitHub. `cargo test --doc` must list one
`readme_examples (line N)` entry per fence.

It is also in the gated prose set (`ts/scripts/gated-docs.cjs`), so
`make prose` covers it: Vale at error level, the recorded alert counts in
`.vale.ini` and `docs/STYLE-GUIDE.md`, and the checks in
`ts/test/docs.test.js`. Adding a page moves those counts; re-measure with
`node ts/scripts/vale-counts.cjs --write` and re-wrap any comment whose
numbers changed length.
