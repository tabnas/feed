# Tutorial — your first feed parse (Go)

This walks you from an empty Go module to a parsed feed, then shows the
one idea that makes `feed` useful: whatever dialect you feed in, you
get the *same* Atom-shaped struct out. Follow it in order.

For a recipe-style index of individual tasks, see the
[how-to guide](guide.md). For exhaustive signatures, options, and the
grammar, see the [reference](reference.md). For how it works — and how
it differs from the TypeScript version — see [concepts](concepts.md).

## 1. Install

`feed` is a plugin for the
[`jsonic`](https://github.com/tabnas/jsonic) parser (which itself wraps
the `tabnas` engine). Initialise a module and pull it in:

```bash
go mod init example
go get github.com/tabnas/feed/go
```

## 2. Parse a feed

Register the plugin on a `jsonic` instance with `UseDefaults`, then
call `Parse`:

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
    fmt.Println(f.Title.Value)          // My Blog
    fmt.Println(f.Entries[0].ID)        // https://example.com/1
    fmt.Println(f.Entries[0].Links[0])  // {https://example.com/1 alternate    0}
}
```

## 3. Read the result — it is Atom-shaped

You handed in **RSS 2.0**, but `f` came back as an `AtomFeed`. That is
the whole point: the plugin normalises every dialect to one struct so
the rest of your code never branches on the source format.

Notice three things in the output:

- `f.Title` is an `*feed.AtomText` — it carries a content type
  (`Type: "text"`) alongside its `Value`, not a bare string.
- The RSS `<guid>` became `f.Entries[0].ID`.
- The RSS `<link>` became an `AtomLink` with `Rel: "alternate"`.

`Parse` returns `(any, error)`. Type-assert the result to the concrete
type for the format you chose — with the default `format`, that is
`feed.AtomFeed`.

## 4. Parse a different dialect — same struct out

Hand in an **Atom 1.0** document. The output struct is the same type,
so your reading code does not change:

```go
got, _ := j.Parse(`
  <feed xmlns="http://www.w3.org/2005/Atom">
    <title>Example Feed</title>
    <entry>
      <title>T</title>
      <content type="html">&lt;p&gt;hi&lt;/p&gt;</content>
    </entry>
  </feed>`)

f := got.(feed.AtomFeed)
fmt.Println(f.Title.Value)             // Example Feed
fmt.Println(f.Entries[0].Content.Type)  // html
fmt.Println(f.Entries[0].Content.Value) // <p>hi</p>
```

The HTML content was XML-escaped in the source (`&lt;p&gt;`); the
parser decodes it back to `<p>hi</p>` and tags it `Type: "html"` so you
know not to trust it as plain text.

## 5. Handle an error

If the root element is not a feed the plugin recognises, `Parse`
returns a non-nil error (a `*jsonic.JsonicError`). Check it like any
other Go error:

```go
got, err := j.Parse(`<not-a-feed/>`)
if err != nil {
    // err.Error() contains:
    //   feed: unrecognized root element "not-a-feed";
    //   expected one of feed, rss, RDF
    fmt.Println("parse failed:", err)
    return
}
_ = got
```

The recognised root elements are `<feed>` (Atom), `<rss>` (RSS
0.91/0.92/2.0), and `<RDF>` (RSS 0.90/1.0). Anything else is an error.

## Where to go next

- [How-to guide](guide.md) — focused recipes (native shape, raw tree,
  dialect detection, error handling).
- [Reference](reference.md) — the API, the one option, the full
  Atom/native struct definitions, and the accepted grammar.
- [Concepts](concepts.md) — why it defaults to Atom, what conversion
  loses, and the differences from the TypeScript version.
