# Concepts

Background on how `@tabnas/feed` is put together, and why. This is
understanding-oriented reading — for steps see the
[tutorial](tutorial.md) and [how-to guide](guide.md), and for exact
signatures, options, and the grammar see the [reference](reference.md).

## A grammar plugin that contributes no grammar

`@tabnas/feed` is a plugin for the
[`tabnas`](https://github.com/tabnas/parser) parsing engine, but it is
an unusual one: it adds **no rules and no tokens** of its own. All the
syntax it accepts is the XML grammar from
[`@tabnas/xml`](https://github.com/tabnas/xml) — the `xml`, `element`,
`content`, and `child` rules, and the `XOP` / `XCL` / `XSC` / `TX`
tokens.

What the plugin actually does, when you `.use(Feed)`, is:

1. call `tn.use(Xml)` so the XML grammar is installed, and
2. register a **before-close** (`bc`) callback on the existing `xml`
   rule.

That callback fires after `@tabnas/xml` has built the element tree.
It reads the root element, decides what dialect it is, and replaces the
parse result with the converted feed object. All the RSS/Atom knowledge
lives in plain helper functions over `XmlElement` values — `detect`,
`parseAtom`, `parseRss2`, `rss2ToAtom`, and friends — not in grammar
rules. This is why the [grammar diagram](grammar.svg) is the XML
grammar's diagram: there is nothing else to draw.

## Three tiers, one option

The plugin is a pipeline with three stop-points, and the `format`
option chooses where to get off:

```
src ──► @tabnas/xml ──► native parser ──► Atom converter
        (XmlElement)    (Rss2Feed/...)    (AtomFeed)
        format:'raw'    format:'native'   format:'atom'  (default)
```

- `'raw'` returns the `XmlElement` tree untouched — no detection, no
  conversion. Use it for namespace extensions the feed types do not
  model, or to call `detect` yourself.
- `'native'` runs the dialect-specific parser and stops: you get an
  `Rss2Feed`, `Rss1Feed`, or `AtomFeed` that mirrors the source
  structure faithfully.
- `'atom'` (default) runs the native parser *and then* the Atom
  converter, so every dialect ends up in one shape.

Each tier is a strict superset of work over the previous one, so there
is never a reason to reach for a lower tier unless you need the extra
fidelity it preserves.

## Why default to Atom?

Atom 1.0 (RFC 4287) is, in practice, a strict superset of what every
flavour of RSS expresses, with consistent typed elements: `AtomText`
carries its content type, `AtomLink` carries `rel` / `type` / `length`,
and dates are well defined. RSS, by contrast, is a small family of
related but inconsistent formats — RSS 0.91 has no `guid`, 0.92 added
`enclosure` and `category`, 1.0 is RDF, 2.0 added `cloud` and `ttl`.

Picking one shape for downstream code to target avoids per-dialect
branching, and Atom is the obvious candidate because it can carry
almost everything the others express. The payoff: the large majority
of feed-consuming code can ignore the source dialect entirely. The
minority that genuinely needs RSS-only metadata opts into
`format: 'native'`.

## What conversion loses

Mapping RSS to Atom is not bijective. The default `'atom'` conversion
deliberately drops fields that have no Atom counterpart:

- `ttl`, `cloud`, `skipHours`, `skipDays` — RSS 2.x channel-level
  scheduling hints; Atom has no equivalent.
- `guid/@isPermaLink` — the value becomes `entry.id`, but the boolean
  flag is dropped.
- `image/title`, `image/link`, `image/width`, `image/height` — Atom's
  `logo` is just a URL.
- `textInput` — an obsolete RSS UI element with no Atom counterpart.
- `category/@domain` becomes `category.scheme` (the intended mapping),
  which loses the original RSS naming.

If any of these matter, parse with `format: 'native'` and read the
dialect-specific structure directly. This is a one-way door by design:
the converter never invents data, so what it cannot map, it omits
rather than guesses.

## Accepted vs. rejected

The plugin is liberal about *what* it parses and strict about the *root
element*:

- **Accepted roots**: `<feed>`, `<rss>`, `<RDF>`. Within those,
  detection is forgiving — a missing or unknown `rss` `@version`
  defaults to RSS 2.0; an `RDF` defaults to RSS 1.0 unless the channel
  carries the Netscape 0.90 namespace; a `feed` defaults to Atom 1.0
  unless it carries the Atom 0.3 namespace.
- **Rejected**: any other root element throws
  `unrecognized root element ...`. The one exception is
  `format: 'raw'`, which returns the tree without ever calling
  detection — so you can inspect or reject unknown documents yourself.
- **XML errors come first**: malformed markup (an unterminated tag, a
  mismatched close) is rejected by `@tabnas/xml` during lexing/parsing,
  before the feed conversion ever runs. Feed-level errors only happen
  for *well-formed* XML with an unexpected root.

A subtle correctness detail: the XML grammar can fire the `xml` rule's
close phase more than once (for example when trailing whitespace makes
the `xml` rule recurse). The conversion callback guards on
`rule.child.node` — the same idiom `@xml-bc` uses — so it runs exactly
once, on the iteration that actually produced the element. Without that
guard, the already-converted result would be fed back through
`detect`/`convert` a second time.

## Cross-language parity through shared fixtures

The TypeScript and Go implementations are kept in lockstep through
[`test/specs/`](../../test/specs/): each `<name>.xml` ships with a
`<name>.detect.json`, a `<name>.atom.json`, and an optional
`<name>.native.json`. Both test suites enumerate the directory, parse
each input, and JSON-deep-compare the result against the expectation
after a marshal/unmarshal round-trip (which collapses property-ordering
and pointer-vs-value differences). Adding a fixture covers both
languages immediately.

A subset of the well-formed feed corpus from
[`kurtmckee/feedparser`](https://github.com/kurtmckee/feedparser) is
also vendored at
[`test/feedparser-wellformed/`](../../test/feedparser-wellformed/) under
BSD 2-Clause; both languages run the same no-error and targeted value
checks against it.

For the Go port's API shape and its (small) accepted differences, see
[../../go/doc/concepts.md](../../go/doc/concepts.md).
