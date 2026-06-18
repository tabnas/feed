# feed (Go)

A Go port of [`@tabnas/feed`](https://github.com/tabnas/feed) — a
[`jsonic`](https://github.com/tabnas/jsonic) plugin (built on
[`xml`](https://github.com/tabnas/xml)) that parses syndication feeds
(**RSS 0.90, 0.91, 0.92, 1.0, 2.0** and **Atom 0.3, 1.0**) into typed
Go structs. By default every dialect is normalised to an Atom-shaped
result, so the same downstream code can consume feeds from any source.

The output JSON shape matches the TypeScript implementation exactly
(both pass the shared [`../test/specs/`](../test/specs/) fixtures).

## Install

```bash
go get github.com/tabnas/feed/go
```

## One tiny example

```go
package main

import (
    "fmt"

    tabnasjsonic "github.com/tabnas/jsonic/go"
    tabnasfeed "github.com/tabnas/feed/go"
)

func main() {
    j := tabnasjsonic.Make()
    if err := j.UseDefaults(tabnasfeed.Feed, tabnasfeed.Defaults); err != nil {
        panic(err)
    }
    got, err := j.Parse(`<rss version="2.0"><channel><title>My Blog</title></channel></rss>`)
    if err != nil {
        panic(err)
    }
    f := got.(tabnasfeed.AtomFeed)
    fmt.Println(f.Title.Value) // My Blog
    fmt.Println(f.Format)      // atom
}
```

The input was RSS 2.0 but `f` is an `AtomFeed` — `Title` is an
`*AtomText` (carrying its content type), and the whole struct follows
RFC 4287. Pass `map[string]any{"format": "native"}` to keep the source
dialect's struct, or `"raw"` for the underlying `map[string]any` XML
element tree.

## Documentation

Full docs follow the four [Diátaxis](https://diataxis.fr) quadrants:

- [Tutorial](doc/tutorial.md) — your first feed parse, step by step.
- [How-to guide](doc/guide.md) — recipes: native shape, raw tree,
  dialect detection, error handling.
- [Reference](doc/reference.md) — the API, the `format` option, the
  Atom/native structs, mapping tables, and the accepted grammar.
- [Concepts](doc/concepts.md) — why it defaults to Atom, what
  conversion loses, and the **differences from the TS version**.

The project [main README](../README.md) covers both languages side by
side; the TypeScript implementation is in [`../ts/`](../ts/).

## Testing

```bash
go test ./...
```

Runs the cross-language fixtures in [`../test/specs/`](../test/specs/)
and the vendored well-formed corpus in
[`../test/feedparser-wellformed/`](../test/feedparser-wellformed/).

## License

MIT. Copyright (c) Richard Rodger and contributors.
