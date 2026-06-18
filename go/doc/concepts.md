# Concepts (Go)

Background on how the `feed` Go package is put together, and why. This
is understanding-oriented reading — for steps see the
[tutorial](tutorial.md) and [how-to guide](guide.md), and for exact
signatures, options, and the grammar see the [reference](reference.md).

It mirrors the canonical TypeScript implementation. The shared design
rationale is summarised here; the section at the end records where the
Go API and value types differ.

## A grammar plugin that contributes no grammar

`feed` is a plugin for the [`jsonic`](https://github.com/tabnas/jsonic)
parser (which wraps the `tabnas` engine), but it is an unusual one: it
adds **no rules and no tokens** of its own. All the syntax it accepts is
the XML grammar from [`github.com/tabnas/xml/go`](https://github.com/tabnas/xml)
— the `xml`, `element`, `content`, and `child` rules.

When you register it, `Feed`:

1. calls `j.UseDefaults(tabnasxml.Xml, tabnasxml.Defaults)` so the XML grammar is
   installed, and
2. registers a **before-close** (`AddBC`) action on the existing `xml`
   rule.

That action fires after the XML plugin has built the element tree. It
reads the root element, decides the dialect, and replaces the parse
result with the converted feed value. All the RSS/Atom knowledge lives
in plain helper functions over `map[string]any` element trees —
`Detect`, `parseAtom`, `parseRss2`, `rss2ToAtom`, and friends — not in
grammar rules.

## Three tiers, one option

The plugin is a pipeline with three stop-points, and the `format`
option chooses where to get off:

```
src ──► xml plugin ───► native parser ───► Atom converter
        (map[string]any)  (Rss2Feed/...)   (AtomFeed)
        format:"raw"      format:"native"  format:"atom"  (default)
```

- `"raw"` returns the `map[string]any` tree untouched — no detection,
  no conversion. Use it for namespace extensions the feed structs do
  not model, or to call `Detect` yourself.
- `"native"` runs the dialect-specific parser and stops: an
  `Rss2Feed`, `Rss1Feed`, or `AtomFeed` mirroring the source.
- `"atom"` (default) runs the native parser *and then* the Atom
  converter, so every dialect ends up in one struct.

## Why default to Atom?

Atom 1.0 (RFC 4287) is, in practice, a strict superset of what every
flavour of RSS expresses, with consistent typed elements: `AtomText`
carries its content type, `AtomLink` carries `Rel` / `Type` / `Length`,
and dates are well defined. RSS, by contrast, is a small family of
related but inconsistent formats. Picking one shape for downstream code
to target avoids per-dialect branching, and Atom can carry almost
everything the others express, so the large majority of feed-consuming
code can ignore the source dialect entirely.

## What conversion loses

Mapping RSS to Atom is not bijective. The default `"atom"` conversion
deliberately drops fields with no Atom counterpart:

- `TTL`, `Cloud`, `SkipHours`, `SkipDays` — RSS 2.x channel-level
  scheduling hints.
- `Guid.IsPermaLink` — the value becomes `Entries[i].ID`, but the
  boolean is dropped.
- `Image.Title`, `Image.Link`, `Image.Width`, `Image.Height` — Atom's
  `Logo` is just a URL.
- `TextInput` — an obsolete RSS UI element with no Atom counterpart.
- `Category.Domain` becomes `Category.Scheme`, losing the RSS naming.

If any of these matter, parse with `format: "native"` and read the
dialect-specific struct directly.

## Accepted vs. rejected

The plugin is liberal about *what* it parses and strict about the *root
element*:

- **Accepted roots**: `<feed>`, `<rss>`, `<RDF>`. Detection within
  those is forgiving — a missing/unknown `rss` `version` defaults to
  RSS 2.0; an `RDF` defaults to RSS 1.0 unless the channel carries the
  Netscape 0.90 namespace; a `feed` defaults to Atom 1.0 unless it
  carries the Atom 0.3 namespace.
- **Rejected**: any other root makes `Parse` return an error. The
  exception is `format: "raw"`, which returns the tree without calling
  detection.
- **XML errors come first**: malformed markup is rejected by the XML
  plugin during lexing/parsing, before the feed conversion runs.

A correctness detail: the XML grammar can fire the `xml` rule's close
phase more than once (for example when trailing whitespace makes the
rule recurse). The conversion action guards on `r.Child.Node` — the
same idiom the XML plugin's own `@xml-bc` uses — so it runs exactly
once, on the iteration that produced the element.

## JSON-shape parity with the TypeScript implementation

The Go structs carry JSON tags chosen so that `json.Marshal(result)`
produces the same shape the TypeScript parser produces with
`JSON.stringify`. This is what makes the shared fixtures in
[`../../test/specs/`](../../test/specs/) work for both languages: each
test JSON-marshal-unmarshals the parser output and deep-compares it to
the language-agnostic expected `*.atom.json` / `*.native.json`. The
vendored well-formed corpus in
[`../../test/feedparser-wellformed/`](../../test/feedparser-wellformed/)
(BSD 2-Clause, from
[kurtmckee/feedparser](https://github.com/kurtmckee/feedparser)) is run
by both languages too.

## Differences from the TS version

The two implementations are kept in lockstep by the shared fixtures, so
the *output JSON* is identical. The differences are in the language API
shape and value representation, not behaviour.

### Registration and parse entry

| Aspect          | TypeScript                                | Go                                              |
| --------------- | ----------------------------------------- | ----------------------------------------------- |
| Engine          | `new Tabnas().use(jsonic).use(Feed, opts)`| `j := tabnasjsonic.Make(); j.UseDefaults(tabnasfeed.Feed, tabnasfeed.Defaults, opts)` |
| Options value   | `{ format: 'native' }` (object)           | `map[string]any{"format": "native"}`            |
| Defaults        | implicit (`format` defaults to `'atom'`)  | explicit `tabnasfeed.Defaults` map you pass in        |
| Parse result    | `j.parse(src)` returns the value          | `j.Parse(src)` returns `(any, error)`           |
| Errors          | `parse` **throws** a `JsonicError`        | `Parse` **returns** a `*tabnasjsonic.JsonicError`     |

In Go you must handle the returned `error`; there is no exception. The
plugin signals a feed-level error by setting `ctx.ParseErr`, which
surfaces as the `Parse` error.

### Value types

| Concept           | TypeScript                          | Go                                  |
| ----------------- | ----------------------------------- | ----------------------------------- |
| Result            | union `AtomFeed \| Rss2Feed \| Rss1Feed \| XmlElement` | `any` (type-assert to the concrete struct) |
| Raw tree element  | `XmlElement` (object with methods)  | `map[string]any`                    |
| Optional sub-objects | optional object fields (`title?`) | pointer fields (`*AtomText`), `nil` when absent — always nil-check |
| Optional scalars  | omitted keys (`undefined`)          | zero values + `omitempty` JSON tags |
| `entry.source`    | `Partial<AtomFeed>`                 | a dedicated `AtomEntrySource` struct (no `Entries`) |
| `guid.isPermaLink`| `boolean` (optional)                | `*bool` (`nil` when the attribute is absent) |
| Detection helper  | `detect(root): { dialect, version }`| `Detect(root any) Detection` (a named struct) |

### Extra exported helper

Go additionally exports `Convert(root any, format string) (any, error)`
as a public function (the conversion the plugin runs internally). In TS
the equivalent `convert` is internal; only `Feed` and `detect` are
exported. Go also exports a `Version` constant mirroring the TS
package version.

### Known accepted differences

There are no known behavioural differences in the parsed output: both
languages pass the same `test/specs/` fixtures after a JSON round-trip.
The only differences are idiomatic-Go surface choices listed above
(pointers vs. optional fields, `(any, error)` vs. throw, `map[string]any`
vs. `XmlElement`).
