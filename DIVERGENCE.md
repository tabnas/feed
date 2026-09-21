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

## The namespace diagnostics lose their message in the Rust port

| input | TypeScript | Go | Rust |
|---|---|---|---|
| `<feed xmlns="http://www.w3.org/2005/Atom"><dc:language>en</dc:language></feed>` | rejected, `element or attribute uses an undeclared namespace prefix` | same | rejected, `namespace resolution failed` |
| `<feed xmlns="http://www.w3.org/2005/Atom" xmlns:a="http://x\ny"/>` | rejected, `namespace name cannot contain white space` | same | rejected, `namespace resolution failed` |

All three ports REJECT both documents, and all three report the same
error code (`unbound_prefix`, `invalid_namespace_uri`). What differs is
the rendered message, which is what
[`test/spec/xml-layer.tsv`](test/spec/xml-layer.tsv) pins: this
repository's fixtures spell an expectation as `ERROR:<message fragment>`
rather than as a code, because the feed layer has no codes of its own
(see [`test/AGENTS.md`](test/AGENTS.md)).

Provenance of the table. The Rust column was measured on 2026-09-21
against the sibling checkouts. The TypeScript and Go columns are the
values those two fixture rows pin, which both suites assert; the same
message text is also what the Rust `tabnas-xml` crate's own template
table carries for those two codes, so the three ports agree on what the
message should say and only the Rust raise path fails to produce it.

The defect is not in this crate and reproduces without it:

```rust
let options = tabnas_xml::XmlOptions {
    strict_namespaces: true,
    ..Default::default()
};
let error = tabnas_xml::make_with(&options)
    .parse("<a><dc:b/></a>")
    .unwrap_err();
// error.code   == "unbound_prefix"        (right)
// error.detail == "namespace resolution failed"  (the placeholder)
```

`register_refs` in `xml/rs/src/lib.rs` raises a namespace failure from
the `@xml-bc` hook. It marks the CURRENT TOKEN bad, which is what makes
the engine interpolate the message template for the code. At document
close the context has no current token, so the fallback
`ActionError::new(code, "namespace resolution failed")` runs instead and
that literal string becomes the detail. The template is never consulted.

Both register rows carry the same `why`, which is the one-line form of
all of that: `tabnas/xml (rs): the namespace failure path never
interpolates its message template`.

**Owner of the repair: `tabnas/xml`, the Rust crate.** Nothing in
`tabnas-feed` can reach that raise path, and working around it here would
mean matching a message the reader does not see, which is a wider claim
than the test measures.

When it lands, both rows go red in
[`test/divergent.tsv`](test/divergent.tsv), and the two entries come out
of `RECORDED_DIVERGENCES` in
[`rs/tests/parity_test.rs`](rs/tests/parity_test.rs) so the shared
fixture rows run normally again.

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
