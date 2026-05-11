# @jsonic/feed

A [Jsonic](https://jsonic.senecajs.org) plugin — built on
[`@jsonic/xml`](https://github.com/jsonicjs/xml) — that parses
syndication feeds (**RSS 0.90, 0.91, 0.92, 1.0, 2.0** and **Atom 0.3,
1.0**) into a typed structure. By default every dialect is normalised
to an Atom-shaped result, so the same downstream code can consume
feeds from any source.

The same parser is available in two languages:

| Language   | Package                                                        | Source                       | Docs                              |
| ---------- | -------------------------------------------------------------- | ---------------------------- | --------------------------------- |
| TypeScript | [`@jsonic/feed`](https://npmjs.com/package/@jsonic/feed)       | [`src/feed.ts`](src/feed.ts) | this file                         |
| Go         | [`github.com/jsonicjs/feed/go`](https://github.com/jsonicjs/feed/tree/main/go) | [`go/feed.go`](go/feed.go) | [`go/README.md`](go/README.md) |

[![npm version](https://img.shields.io/npm/v/@jsonic/feed.svg)](https://npmjs.com/package/@jsonic/feed)
[![build](https://github.com/jsonicjs/feed/actions/workflows/build.yml/badge.svg)](https://github.com/jsonicjs/feed/actions/workflows/build.yml)


| ![Voxgig](https://www.voxgig.com/res/img/vgt01r.png) | This open source module is sponsored and supported by [Voxgig](https://www.voxgig.com). |
| ---------------------------------------------------- | --------------------------------------------------------------------------------------- |


> **Go users:** the [`go/README.md`](go/README.md) is a Go-only view
> of these same docs. The rest of this file covers both languages
> side by side.

This documentation follows the four [Diátaxis](https://diataxis.fr)
modes:

- [Tutorial](#tutorial) — work through a first feed parse
- [How-to guides](#how-to-guides) — short recipes for specific tasks
- [Reference](#reference) — types, options, mapping tables
- [Explanation](#explanation) — design rationale and trade-offs


---

## Tutorial

This walkthrough takes you from an empty project to a parsed feed in
under a minute. By the end you will have a working `Feed`-equipped
parser, recognise the shape of its output, and know where to look in
the rest of the docs.

### TypeScript

Install the plugin and its peer dependencies:

```bash
npm install @jsonic/feed jsonic @jsonic/xml
```

Create `index.ts`:

```typescript
import { Jsonic } from 'jsonic'
import { Feed } from '@jsonic/feed'

const j = Jsonic.make().use(Feed)

const result = j(`
  <rss version="2.0">
    <channel>
      <title>My Blog</title>
      <link>https://example.com/</link>
      <description>Posts</description>
      <item>
        <title>Hello</title>
        <link>https://example.com/1</link>
        <guid>https://example.com/1</guid>
        <pubDate>Wed, 13 Dec 2003 18:30:02 GMT</pubDate>
      </item>
    </channel>
  </rss>
`)

console.log(result.title.value)            // 'My Blog'
console.log(result.entries[0].id)          // 'https://example.com/1'
console.log(result.entries[0].links[0])    // { href: '...', rel: 'alternate' }
```

The input was RSS 2.0 but `result` is in **Atom shape**:
`title` is an `AtomText` (`{ type, value }`), `entries[0].id` came
from RSS's `<guid>`, and the `<link>` became an Atom link with
`rel: 'alternate'`. The plugin handles every supported dialect this
way, so the rest of your code never has to branch on the source
format.

### Go

Initialise a module and pull in the plugin:

```bash
go mod init example
go get github.com/jsonicjs/feed/go
```

Create `main.go`:

```go
package main

import (
    "fmt"
    jsonic "github.com/jsonicjs/jsonic/go"
    feed "github.com/jsonicjs/feed/go"
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
          <item><title>Hello</title><guid>1</guid></item>
        </channel>
      </rss>`)
    if err != nil {
        panic(err)
    }
    f := got.(feed.AtomFeed)
    fmt.Println(f.Title.Value, "/", f.Entries[0].ID)
    // My Blog / 1
}
```

`got` is `any`; type-assert it to `feed.AtomFeed` (the default), or
to `feed.Rss2Feed` / `feed.Rss1Feed` when you opt into the native
shape (see the next section).


---

## How-to guides

### How to keep the source dialect's structure

When you need RSS-specific fields like `ttl`, `cloud`, or `skipDays`
that the Atom shape does not carry, ask for the native form:

```typescript
import { Feed, type Rss2Feed } from '@jsonic/feed'
const j = Jsonic.make().use(Feed, { format: 'native' })
const native = j(rssSource) as Rss2Feed
// native.ttl, native.cloud, native.skipDays
```

```go
j := jsonic.Make()
j.UseDefaults(feed.Feed, feed.Defaults, map[string]any{"format": "native"})
got, _ := j.Parse(rssSource)
native := got.(feed.Rss2Feed)
// native.TTL, native.Cloud, native.SkipDays
```

The native return type is a discriminated union on `format`:

| Input dialect       | Native return type | `format`  | `version`         |
|---------------------|--------------------|-----------|-------------------|
| Atom 1.0 / Atom 0.3 | `AtomFeed`         | `'atom'`  | `'1.0'` / `'0.3'` |
| RSS 2.0 / 0.92 / 0.91 | `Rss2Feed`       | `'rss'`   | `'2.0'` / `'0.92'` / `'0.91'` |
| RSS 1.0 / 0.90      | `Rss1Feed`         | `'rdf'`   | `'1.0'` / `'0.90'` |

### How to access the raw XML tree

When even the native shape is not enough — for example you need a
non-standard namespace extension like `<media:content>` — drop down
to the raw element tree from `@jsonic/xml`:

```typescript
const j = Jsonic.make().use(Feed, { format: 'raw' })
const tree = j(rssSource)
// tree.localName === 'rss', tree.children === [...]
```

```go
j := jsonic.Make()
j.UseDefaults(feed.Feed, feed.Defaults, map[string]any{"format": "raw"})
got, _ := j.Parse(rssSource)
tree := got.(map[string]any)
// tree["localName"] == "rss"; tree["children"].([]any)
```

### How to detect a feed's dialect without converting

Use `format: 'raw'` to get the underlying XML tree, then call
`detect`:

```typescript
import { Feed, detect } from '@jsonic/feed'
const j = Jsonic.make().use(Feed, { format: 'raw' })
const { dialect, version } = detect(j(rssSource))
// e.g. { dialect: 'rss', version: 'rss20' }
```

```go
j := jsonic.Make()
j.UseDefaults(feed.Feed, feed.Defaults, map[string]any{"format": "raw"})
got, _ := j.Parse(rssSource)
det := feed.Detect(got)
// e.g. {Dialect: "rss", Version: "rss20"}
```

### How to read fields that the Atom conversion drops

The default conversion is lossy by design (see
[Explanation](#what-conversion-loses) below). If your application
needs both the convenient Atom shape *and* a stray RSS-only field,
parse twice into the same source — once for each format — or use
`format: 'raw'` plus your own extraction. The recommended path is to
parse `'native'` and convert in your own code only when you need the
Atom shape.


---

## Reference

### Plugin registration

**TypeScript**

```typescript
import { Feed } from '@jsonic/feed'
const j = Jsonic.make().use(Feed, options?)
const result = j(src)
```

**Go**

```go
j := jsonic.Make()
err := j.UseDefaults(feed.Feed, feed.Defaults, opts)
result, err := j.Parse(src)
```

### Options

| Key      | Type                          | Default  | Effect                                                |
|----------|-------------------------------|----------|-------------------------------------------------------|
| `format` | `'atom' \| 'native' \| 'raw'` | `'atom'` | Output shape: normalised Atom, dialect-native, or raw XML element tree |

### Result types

The `format` option determines which type the parser returns:

| `format`   | TypeScript                                  | Go                                          |
|------------|---------------------------------------------|---------------------------------------------|
| `'atom'`   | `AtomFeed`                                  | `feed.AtomFeed`                             |
| `'native'` | `AtomFeed \| Rss2Feed \| Rss1Feed`          | `feed.AtomFeed` / `feed.Rss2Feed` / `feed.Rss1Feed` |
| `'raw'`    | `XmlElement` (from `@jsonic/xml`)           | `map[string]any`                            |

### Atom shape (RFC 4287)

```typescript
type AtomFeed = {
  format: 'atom'
  version: '1.0' | '0.3' | string
  id?: string
  title?: AtomText
  updated?: string
  authors?: AtomPerson[]
  contributors?: AtomPerson[]
  categories?: AtomCategory[]
  generator?: AtomGenerator
  icon?: string
  logo?: string
  rights?: AtomText
  subtitle?: AtomText
  links?: AtomLink[]
  entries: AtomEntry[]
}

type AtomEntry = {
  id?: string
  title?: AtomText
  updated?: string
  published?: string
  authors?: AtomPerson[]
  contributors?: AtomPerson[]
  categories?: AtomCategory[]
  content?: AtomContent
  links?: AtomLink[]
  rights?: AtomText
  summary?: AtomText
  source?: Partial<AtomFeed>
}

type AtomText      = { type: 'text' | 'html' | 'xhtml'; value: string }
type AtomPerson    = { name: string; uri?: string; email?: string }
type AtomLink      = { href: string; rel?: string; type?: string;
                       hreflang?: string; title?: string; length?: number }
type AtomCategory  = { term: string; scheme?: string; label?: string }
type AtomGenerator = { uri?: string; version?: string; value: string }
type AtomContent   = { type: string; src?: string; value?: string }
```

The Go structs (`AtomFeed`, `AtomEntry`, …) carry equivalent JSON
tags so they marshal to the same shape. See
[`go/feed.go`](go/feed.go) for the full set, including the native
RSS 2.0 (`Rss2Feed`, `Rss2Item`, …) and RSS 1.0 (`Rss1Feed`,
`Rss1Item`, …) types.

### Detection helper

```typescript
function detect(root: XmlElement):
  { dialect: 'atom' | 'rss' | 'rdf' | 'unknown'
    version: 'atom10' | 'atom03' | 'rss20' | 'rss092' |
             'rss091u' | 'rss091n' | 'rss10' | 'rss090' |
             'unknown' }
```

```go
func Detect(root any) Detection
type Detection struct { Dialect, Version string }
```

### Mapping: RSS 2.x / 0.92 / 0.91 → Atom

| RSS source                | Atom target                                        |
|---------------------------|----------------------------------------------------|
| `channel/title`           | `feed.title` (`type: 'text'`)                      |
| `channel/description`     | `feed.subtitle` (`type: 'text'`)                   |
| `channel/link`            | `feed.id` and `feed.links[]` (`rel: 'alternate'`)  |
| `channel/copyright`       | `feed.rights`                                      |
| `channel/lastBuildDate`   | `feed.updated`                                     |
| `channel/pubDate`         | `feed.updated` (fallback)                          |
| `channel/managingEditor`  | `feed.authors[0]` (parsed as `email (Name)`)       |
| `channel/generator`       | `feed.generator.value`                             |
| `channel/image/url`       | `feed.logo`                                        |
| `item/guid`               | `entry.id`                                         |
| `item/link` (no `guid`)   | `entry.id` (fallback) and `entry.links[]`          |
| `item/description`        | `entry.summary` (`type: 'html'`)                   |
| `item/pubDate`            | `entry.published` and `entry.updated`              |
| `item/author`             | `entry.authors[0]`                                 |
| `item/enclosure`          | `entry.links[]` with `rel: 'enclosure'`            |
| `item/comments`           | `entry.links[]` with `rel: 'replies'`              |
| `item/category`           | `entry.categories[].term` (+ `scheme` from domain) |

### Mapping: RDF (RSS 1.0 / 0.90) → Atom

| RDF source            | Atom target                                       |
|-----------------------|---------------------------------------------------|
| `channel/@rdf:about`  | `feed.id`                                         |
| `channel/title`       | `feed.title`                                      |
| `channel/description` | `feed.subtitle`                                   |
| `channel/link`        | `feed.links[]` (`rel: 'alternate'`)               |
| `image/url`           | `feed.logo`                                       |
| `item/@rdf:about`     | `entry.id`                                        |
| `item/title`          | `entry.title`                                     |
| `item/link`           | `entry.links[]` (`rel: 'alternate'`)              |

### Atom 0.3 → 1.0 element renames

| Atom 0.3    | Atom 1.0    |
|-------------|-------------|
| `tagline`   | `subtitle`  |
| `modified`  | `updated`   |
| `issued`    | `published` |
| `copyright` | `rights`    |


---

## Explanation

### Why default to Atom?

Atom 1.0 (RFC 4287) is a strict superset of what every flavour of
RSS expresses, with consistent typed elements (`AtomText` carries
its content type, `AtomLink` carries `rel`/`type`/`length`, dates
are well-defined). RSS, by contrast, is a small family of related
formats with overlapping but inconsistent shapes — RSS 0.91 has no
`guid`, 0.92 added `enclosure` and `category`, 1.0 is RDF, 2.0
added `cloud` and `ttl`. Picking one shape for downstream code to
target avoids per-dialect branching, and Atom is the obvious
candidate because it can carry everything the others express.

The result is that 95% of feed-consuming code can ignore the source
dialect entirely. The remaining 5% — applications that genuinely
need RSS-only metadata — opt into `format: 'native'`.

### What conversion loses

Mapping RSS to Atom is not bijective. The default conversion drops:

- `ttl`, `cloud`, `skipHours`, `skipDays` (RSS 2.x channel-level
  scheduling hints — Atom has no equivalent)
- `guid/@isPermaLink` (true/false flag; the value becomes `entry.id`
  but the boolean is dropped)
- `image/title`, `image/link`, `image/width`, `image/height`
  (Atom's `logo` is just a URL)
- `textInput` (an obsolete RSS UI element with no Atom counterpart)
- `category/@domain` becomes `category.scheme`, which is the
  intended mapping but loses the original RSS naming

If any of these matter, parse with `format: 'native'` and read the
dialect-specific structure directly.

### Composition with `@jsonic/xml`

The plugin layers on top of [`@jsonic/xml`](https://github.com/jsonicjs/xml)
in three tiers:

```
src ──► @jsonic/xml ──► native parser ──► Atom converter
        (XmlElement)    (Rss2Feed/...)    (AtomFeed)
        format:'raw'    format:'native'   format:'atom'  (default)
```

Each tier is exposed by a `format` option, so you can stop at
whichever level your application needs. Internally, the Feed plugin
calls `jsonic.use(Xml)` itself and registers a `bc` (before-close)
hook on the `xml` rule that runs after `@jsonic/xml`'s own
`@xml-bc`. The hook gates on `r.child.node` — the same idiom
`@xml-bc` uses — so it runs exactly once even when the grammar's
trailing-whitespace recursion fires `bc` again.

### Cross-language parity through shared fixtures

The TypeScript and Go implementations are kept in sync through
[`test/specs/`](test/specs/): each `<name>.xml` ships with a
`<name>.detect.json`, `<name>.atom.json`, and an optional
`<name>.native.json`. Both test suites enumerate the directory,
parse each input, and JSON-deep-compare the result against the
expectation after a marshal/unmarshal round-trip (which collapses
property-ordering and pointer-vs-value differences). Adding a
fixture covers both languages immediately.

A subset of the well-formed feed corpus from
[`kurtmckee/feedparser`](https://github.com/kurtmckee/feedparser)
is also vendored at
[`test/feedparser-wellformed/`](test/feedparser-wellformed/) under
BSD 2-Clause; both languages run the same no-error and targeted
value checks against it.


---

## Acknowledgments

- [kurtmckee/feedparser](https://github.com/kurtmckee/feedparser) by
  Kurt McKee and Mark Pilgrim — vendored well-formed corpus, BSD
  2-Clause. See [THIRD_PARTY_NOTICES.md](./THIRD_PARTY_NOTICES.md).


## License

MIT. Copyright (c) 2021-2025 Richard Rodger and contributors.
