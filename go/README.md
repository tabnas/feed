# feed (Go)

A Go port of [`@tabnas/feed`](https://github.com/tabnas/feed) — a
[Jsonic](https://github.com/tabnas/jsonic) plugin (built on
[`xml`](https://github.com/tabnas/xml)) that parses syndication
feeds (**RSS 0.90, 0.91, 0.92, 1.0, 2.0** and **Atom 0.3, 1.0**)
into typed Go structs. By default every dialect is normalised to an
Atom-shaped result, so the same downstream code can consume feeds
from any source.

For the full project — including the TypeScript implementation,
shared test fixtures, and language-agnostic explanation — see the
[main README](../README.md).

This README follows the four [Diátaxis](https://diataxis.fr) modes:

- [Tutorial](#tutorial) — work through a first feed parse
- [How-to guides](#how-to-guides) — short recipes for specific tasks
- [Reference](#reference) — types, options, mapping tables
- [Explanation](#explanation) — design rationale and trade-offs


---

## Tutorial

This walkthrough takes you from an empty Go module to a parsed feed.

Initialise a module and pull in the plugin:

```bash
go mod init example
go get github.com/tabnas/feed/go
```

Create `main.go`:

```go
package main

import (
    "fmt"

    jsonic "github.com/tabnas/jsonic/go"
    feed "github.com/tabnas/feed/go"
)

func main() {
    j := jsonic.Make()
    if err := j.UseDefaults(feed.Feed, feed.Defaults); err != nil {
        panic(err)
    }
    got, err := j.Parse(`
      <rss version="2.0">
        <channel>
          <title>My Blog</title>
          <link>https://example.com/</link>
          <description>Posts</description>
          <item>
            <title>Hello</title>
            <link>https://example.com/1</link>
            <guid>https://example.com/1</guid>
          </item>
        </channel>
      </rss>`)
    if err != nil {
        panic(err)
    }
    f := got.(feed.AtomFeed)
    fmt.Println(f.Title.Value)         // My Blog
    fmt.Println(f.Entries[0].ID)       // https://example.com/1
    fmt.Println(f.Entries[0].Links[0]) // {https://example.com/1 alternate ...}
}
```

The input was RSS 2.0 but `f` is an `AtomFeed`: `Title` is an
`*AtomText` (carrying its content type), the `<guid>` became
`Entries[0].ID`, and the `<link>` became an `AtomLink` with
`Rel: "alternate"`. The plugin handles every supported dialect this
way, so the rest of your code never has to branch on the source
format.

`Parse` returns `(any, error)`. With the default `format`, the
concrete type is `feed.AtomFeed`; with `format: "native"` it is
`feed.Rss2Feed`, `feed.Rss1Feed`, or `feed.AtomFeed` depending on
the input dialect; with `format: "raw"` it is `map[string]any` (the
raw element tree from `@tabnas/xml`).


---

## How-to guides

### How to keep the source dialect's structure

When you need RSS-specific fields like `TTL`, `Cloud`, or
`SkipDays` that the Atom shape does not carry, ask for the native
form:

```go
j := jsonic.Make()
j.UseDefaults(feed.Feed, feed.Defaults, map[string]any{"format": "native"})
got, _ := j.Parse(rssSource)
native := got.(feed.Rss2Feed)
// native.TTL, native.Cloud, native.SkipDays
```

The native return type depends on the input dialect:

| Input dialect         | Type assertion target |
|-----------------------|-----------------------|
| Atom 1.0 / Atom 0.3   | `feed.AtomFeed`       |
| RSS 2.0 / 0.92 / 0.91 | `feed.Rss2Feed`       |
| RSS 1.0 / 0.90        | `feed.Rss1Feed`       |

Switch on the `Format` field if the dialect is not known up front:

```go
switch v := got.(type) {
case feed.AtomFeed:
    // v.Format == "atom"
case feed.Rss2Feed:
    // v.Format == "rss"
case feed.Rss1Feed:
    // v.Format == "rdf"
}
```

### How to access the raw XML tree

When even the native shape is not enough — for example you need a
non-standard namespace extension like `<media:content>` — drop down
to the raw element tree from `@tabnas/xml`:

```go
j := jsonic.Make()
j.UseDefaults(feed.Feed, feed.Defaults, map[string]any{"format": "raw"})
got, _ := j.Parse(rssSource)
tree := got.(map[string]any)
// tree["localName"] == "rss"
// tree["children"].([]any) == [...]
// tree["attributes"].(map[string]string) == {"version": "2.0"}
```

### How to detect a feed's dialect without converting

Use `format: "raw"` to get the underlying XML tree, then call
`feed.Detect`:

```go
j := jsonic.Make()
j.UseDefaults(feed.Feed, feed.Defaults, map[string]any{"format": "raw"})
got, _ := j.Parse(rssSource)
det := feed.Detect(got)
// e.g. feed.Detection{Dialect: "rss", Version: "rss20"}
```

### How to handle parse errors

`Parse` returns a `*jsonic.JsonicError` for both XML-level errors
(unterminated tag, mismatched close, etc.) and feed-level errors
(unrecognised root element):

```go
got, err := j.Parse(src)
if err != nil {
    var je *jsonic.JsonicError
    if errors.As(err, &je) {
        // structured error with row/col, error code, source snippet
        log.Printf("feed parse failed: %v", je)
    }
    return err
}
```

### How to read fields that the Atom conversion drops

The default conversion is lossy by design (see
[Explanation](#what-conversion-loses) below). The recommended path
is to register the plugin with `format: "native"` and convert in
your own code only when you need the Atom shape.


---

## Reference

### Plugin registration

```go
func Feed(j *jsonic.Jsonic, opts map[string]any) error
var Defaults = map[string]any{ "format": "atom" }
```

Register via `UseDefaults`:

```go
j := jsonic.Make()
err := j.UseDefaults(feed.Feed, feed.Defaults, opts)
result, err := j.Parse(src)
```

`UseDefaults` merges `opts` over `Defaults`, so caller options
override defaults. Pass `nil` (or omit) for defaults-only.

### Options

| Key      | Type     | Default  | Effect                                          |
|----------|----------|----------|-------------------------------------------------|
| `format` | `string` | `"atom"` | `"atom"`, `"native"`, or `"raw"` (see below)    |

### Result types by `format`

| `format`   | Concrete type returned by `Parse`                      |
|------------|--------------------------------------------------------|
| `"atom"`   | `feed.AtomFeed`                                        |
| `"native"` | `feed.AtomFeed` / `feed.Rss2Feed` / `feed.Rss1Feed`    |
| `"raw"`    | `map[string]any` (the `@tabnas/xml` element tree)      |

### Atom shape (RFC 4287)

```go
type AtomFeed struct {
    Format       string         `json:"format"`
    Version      string         `json:"version"`
    ID           string         `json:"id,omitempty"`
    Title        *AtomText      `json:"title,omitempty"`
    Updated      string         `json:"updated,omitempty"`
    Authors      []AtomPerson   `json:"authors,omitempty"`
    Contributors []AtomPerson   `json:"contributors,omitempty"`
    Categories   []AtomCategory `json:"categories,omitempty"`
    Generator    *AtomGenerator `json:"generator,omitempty"`
    Icon         string         `json:"icon,omitempty"`
    Logo         string         `json:"logo,omitempty"`
    Rights       *AtomText      `json:"rights,omitempty"`
    Subtitle     *AtomText      `json:"subtitle,omitempty"`
    Links        []AtomLink     `json:"links,omitempty"`
    Entries      []AtomEntry    `json:"entries"`
}

type AtomEntry struct {
    ID           string           `json:"id,omitempty"`
    Title        *AtomText        `json:"title,omitempty"`
    Updated      string           `json:"updated,omitempty"`
    Published    string           `json:"published,omitempty"`
    Authors      []AtomPerson     `json:"authors,omitempty"`
    Contributors []AtomPerson     `json:"contributors,omitempty"`
    Categories   []AtomCategory   `json:"categories,omitempty"`
    Content      *AtomContent     `json:"content,omitempty"`
    Links        []AtomLink       `json:"links,omitempty"`
    Rights       *AtomText        `json:"rights,omitempty"`
    Summary      *AtomText        `json:"summary,omitempty"`
    Source       *AtomEntrySource `json:"source,omitempty"`
}

type AtomText      struct { Type, Value string }
type AtomPerson    struct { Name, URI, Email string }
type AtomLink      struct { Href, Rel, Type, Hreflang, Title string; Length int }
type AtomCategory  struct { Term, Scheme, Label string }
type AtomGenerator struct { URI, Version, Value string }
type AtomContent   struct { Type, Src, Value string }
```

`AtomEntrySource` is a slim variant of `AtomFeed` (no `Entries`)
used for the Atom-source-element that an RSS `<item>/<source>` maps
to.

### Native types

```go
type Rss2Feed struct { /* RSS 0.91 / 0.92 / 2.0 channel + items */ }
type Rss2Item struct { /* one <item> in RSS 2.x */ }
type Rss1Feed struct { /* RSS 0.90 / 1.0 RDF channel + items */ }
type Rss1Item struct { /* one <item> in RSS 1.0 */ }
```

See [`feed.go`](feed.go) for the full field list.

### Detection helper

```go
func Detect(root any) Detection
type Detection struct {
    Dialect string `json:"dialect"`
    Version string `json:"version"`
}
```

`Dialect` is one of `"atom"`, `"rss"`, `"rdf"`, `"unknown"`.
`Version` is one of `"atom10"`, `"atom03"`, `"rss20"`, `"rss092"`,
`"rss091u"`, `"rss091n"`, `"rss10"`, `"rss090"`, `"unknown"`.

### Mapping tables

For the full RSS-to-Atom and RDF-to-Atom mapping tables, see the
[Reference section of the main README](../README.md#reference).


---

## Explanation

### Why default to Atom?

Atom 1.0 (RFC 4287) is a strict superset of what every flavour of
RSS expresses, with consistent typed elements (`AtomText` carries
its content type, `AtomLink` carries `rel`/`type`/`length`, dates
are well-defined). RSS, by contrast, is a small family of related
formats with overlapping but inconsistent shapes. Picking one shape
for downstream code to target avoids per-dialect branching, and
Atom is the obvious candidate because it can carry everything the
others express.

### What conversion loses

Mapping RSS to Atom is not bijective. The default conversion drops:

- `Ttl`, `Cloud`, `SkipHours`, `SkipDays` (RSS 2.x channel-level
  scheduling hints — Atom has no equivalent)
- `Guid.IsPermaLink` (the value becomes `Entries[i].ID` but the
  boolean is dropped)
- `Image.Title`, `Image.Link`, `Image.Width`, `Image.Height`
  (Atom's `Logo` is just a URL)
- `TextInput` (an obsolete RSS UI element with no Atom counterpart)

If any of these matter, parse with `format: "native"` and read the
dialect-specific structure directly.

### Composition with the `xml` plugin

The plugin layers on top of
[`github.com/tabnas/xml/go`](https://github.com/tabnas/xml) in
three tiers:

```
src ──► xml plugin ───► native parser ───► Atom converter
        (map[string]any)  (Rss2Feed/...)   (AtomFeed)
        format:"raw"      format:"native"  format:"atom"  (default)
```

Each tier is exposed by a `format` option, so you can stop at
whichever level your application needs. Internally, `Feed` calls
`j.UseDefaults(xml.Xml, xml.Defaults)` itself and registers a
`bc` (before-close) action on the `xml` rule that runs after the
xml plugin's own `@xml-bc`. The hook gates on `r.Child.Node` — the
same idiom `@xml-bc` uses — so it runs exactly once even when the
grammar's trailing-whitespace recursion fires `bc` again.

### JSON-shape parity with the TypeScript implementation

The Go structs carry JSON tags chosen so that
`json.Marshal(result)` produces the same shape the TypeScript
parser produces with `JSON.stringify`. This is what makes the
shared fixtures in
[`../test/specs/`](../test/specs/) work for both languages: each
test JSON-marshal-unmarshals the parser output and deep-compares it
to the language-agnostic expected `*.atom.json` / `*.native.json`.


---

## Testing

```bash
go test ./...
```

The Go test suite runs the cross-language fixtures in
[`../test/specs/`](../test/specs/) and the vendored well-formed
corpus in
[`../test/feedparser-wellformed/`](../test/feedparser-wellformed/).


---

## License

MIT. Copyright (c) 2021-2025 Richard Rodger and contributors.
