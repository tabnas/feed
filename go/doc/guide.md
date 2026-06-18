# How-to guide (Go)

Short, task-focused recipes. Each is self-contained and assumes you
have the plugin installed (see the [tutorial](tutorial.md) for the
basics). For full struct lists, options, and the mapping tables, follow
the links into the [reference](reference.md).

Every recipe starts from a `jsonic` instance with the `Feed` plugin
registered:

```go
import (
    jsonic "github.com/tabnas/jsonic/go"
    feed "github.com/tabnas/feed/go"
)

j := jsonic.Make()
j.UseDefaults(feed.Feed, feed.Defaults)
```

## Use it as a plugin

Register `feed.Feed` with `feed.Defaults` via `UseDefaults`. The plugin
pulls in `github.com/tabnas/xml/go` for you, so you do not register the
XML plugin yourself. Then `Parse` any feed source:

```go
j := jsonic.Make()
if err := j.UseDefaults(feed.Feed, feed.Defaults); err != nil {
    panic(err)
}
got, err := j.Parse(`<rss version="2.0"><channel><title>x</title></channel></rss>`)
if err != nil {
    panic(err)
}
f := got.(feed.AtomFeed) // f.Format == "atom", f.Version == "1.0"
```

`UseDefaults` merges any extra option map over `feed.Defaults`, so
caller options override defaults. The instance is reusable.

## Keep the source dialect's structure

The default Atom shape is lossy (see [what conversion
loses](concepts.md#what-conversion-loses)). When you need RSS-specific
fields like `TTL`, `Cloud`, or `SkipDays`, register with
`format: "native"`:

```go
j := jsonic.Make()
j.UseDefaults(feed.Feed, feed.Defaults, map[string]any{"format": "native"})
got, _ := j.Parse(rssSource)
native := got.(feed.Rss2Feed)
// native.TTL, native.Cloud, native.SkipDays
```

The native concrete type depends on the input dialect:

| Input dialect         | Type assertion target |
| --------------------- | --------------------- |
| Atom 1.0 / 0.3        | `feed.AtomFeed`       |
| RSS 2.0 / 0.92 / 0.91 | `feed.Rss2Feed`       |
| RSS 1.0 / 0.90        | `feed.Rss1Feed`       |

When the dialect is not known up front, switch on the result type:

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

## Access the raw XML tree

When even the native shape is not enough — for example you need a
non-standard namespace extension like `<media:content>` — drop down to
the raw element tree from the XML plugin with `format: "raw"`:

```go
j := jsonic.Make()
j.UseDefaults(feed.Feed, feed.Defaults, map[string]any{"format": "raw"})
got, _ := j.Parse(`<rss version="2.0"><channel><title>x</title></channel></rss>`)
tree := got.(map[string]any)
// tree["localName"] == "rss"
// tree["children"].([]any) == [...]
// tree["attributes"] holds {"version": "2.0"}
```

An element is a `map[string]any` with keys `name`, `localName`,
`prefix`, `namespace`, `attributes`, and `children` (a `[]any` of
strings for text and nested element maps).

## Detect a feed's dialect without converting

`feed.Detect` reports the dialect and version of a raw XML root. Parse
with `format: "raw"`, then call it:

```go
j := jsonic.Make()
j.UseDefaults(feed.Feed, feed.Defaults, map[string]any{"format": "raw"})
got, _ := j.Parse(`<rss version="2.0"><channel><title>x</title></channel></rss>`)

det := feed.Detect(got)
// det == feed.Detection{Dialect: "rss", Version: "rss20"}
```

`Detect` never returns an error; an unrecognised root gives
`feed.Detection{Dialect: "unknown", Version: "unknown"}`.

## Convert a tree you already have

If you obtained a raw element tree some other way, call `feed.Convert`
directly — it is what the plugin runs internally:

```go
out, err := feed.Convert(rawTree, "atom") // or "native" / "raw"
if err != nil {
    // e.g. unrecognized root element
}
f := out.(feed.AtomFeed)
```

## Handle parse errors

`Parse` returns a non-nil `error` for both XML-level failures
(unterminated tag, mismatched close) and feed-level failures
(unrecognised root). The concrete type is `*jsonic.JsonicError`, which
carries a `Code` and a formatted message with source context:

```go
import "errors"

got, err := j.Parse(src)
if err != nil {
    var je *jsonic.JsonicError
    if errors.As(err, &je) {
        // je.Code, je.Error() — structured, with row/col and a snippet
        log.Printf("feed parse failed: %v", je)
    }
    return err
}
_ = got
```

A feed-level error's message contains
`feed: unrecognized root element ...; expected one of feed, rss, RDF`.

## Read fields the Atom conversion drops

The default conversion is lossy by design. The recommended path is to
register the plugin with `format: "native"` and do any Atom-style
mapping in your own code only where you need it — that way you never
lose a field you might want.
