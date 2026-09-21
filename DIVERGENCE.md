# Divergences

Where a port of `@tabnas/feed` produces a different result from the
canonical TypeScript, and why.

TypeScript in [`ts/`](ts/) is canonical. A divergence recorded here is
not a licence to keep it: each entry names what is wrong, where the
repair belongs, and the executable row that fails the day it lands.

Every entry is also a row in [`test/divergent.tsv`](test/divergent.tsv),
which the Rust suite runs through
[`rs/tests/divergent_test.rs`](rs/tests/divergent_test.rs). That register
fails BOTH ways: when a port regresses, and when a port is repaired and
the row stops being true. Prose alone is not a record, and no claim here
is wider than what that file measures.

## None are recorded today

The three ports agree on every input this repository measures, so
[`test/divergent.tsv`](test/divergent.tsv) holds no rows and
[`rs/tests/parity_test.rs`](rs/tests/parity_test.rs) runs every shared
fixture row with no exemption. The register file stays, header and all,
because the suite still reads it and a new divergence is one row away.

The entry that stood here until 2026-09-21 recorded that the Rust
`tabnas-xml` crate rendered the literal text `namespace resolution
failed` in place of the message each namespace code names, which made two
rows of [`test/spec/xml-layer.tsv`](test/spec/xml-layer.tsv) unsatisfiable
in this port: they pin a message fragment, and the fragment was never
produced. The repair landed in `tabnas/xml` (`rs/src/lib.rs`, the
`@xml-bc` hook now marks the engine's no-token sentinel instead of
returning a bare `ActionError`), the register went red saying the
divergence was closed, and both rows went back to being ordinary fixture
rows. Measured through `tabnas_xml` and through this crate on 2026-09-21:
`unbound_prefix` renders `element or attribute uses an undeclared
namespace prefix` and `invalid_namespace_uri` renders `namespace name
cannot contain white space`, at row 1 column 1, in all three runtimes.

## What is NOT a divergence

Three differences show up in a reader's diff and are not behaviour
differences. They are listed so that nobody records them here later.

- **Numbers render with a decimal point.** The Rust engine's `Value`
  holds a number as an `f64`, so `serde_json` writes `12.0` where
  `JSON.stringify` writes `12`. The fixture comparison is structural and
  reads both as the same number, which is why every row carrying a
  `length`, `ttl`, `width` or `skipHours` passes unchanged. No value is
  ever turned into text by this crate, so the JavaScript
  `Number`-to-`String` algorithm does not arise here at all; see
  `rs/AGENTS.md`.
- **A rejection carries a code.** The canonical plugin throws a plain
  `Error` with no code; the Go port sets a `#BD` token. The Rust engine's
  error channel requires a code, so this crate uses
  `feed_unrecognized_root` for the one rejection it raises itself. The
  MESSAGE is identical in all three, and the message is what every
  fixture pins.
- **Options are a struct.** `FeedOptions` has a field per option rather
  than a loose bag, with `from_value` and `to_value` converting, so a
  serialized configuration still works. The names, the defaults and the
  reading rules are the canonical ones.
