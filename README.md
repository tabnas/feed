# @tabnas/feed

<!-- tabnas-badges -->
[![npm](https://tabnas.github.io/status/badges/feed-npm.svg)](https://www.npmjs.com/package/@tabnas/feed)
[![CI](https://github.com/tabnas/feed/actions/workflows/ci.yml/badge.svg)](https://github.com/tabnas/feed/actions/workflows/ci.yml)
[![go](https://tabnas.github.io/status/badges/feed-go.svg)](https://pkg.go.dev/github.com/tabnas/feed/go)
[![tabnas standard](https://tabnas.github.io/status/badges/feed-standard.svg)](https://tabnas.github.io/status/)
<!-- /tabnas-badges -->

Parses RSS (0.90, 0.91, 0.92, 1.0, 2.0) and Atom (0.3, 1.0) syndication
feeds into a typed structure. By default every dialect is normalised to
a single **Atom-shaped** result, so the same downstream code can consume
feeds from any source. It is a plugin for the
[`tabnas`](https://github.com/tabnas/parser) parsing engine, built on top
of [`@tabnas/xml`](https://github.com/tabnas/xml), and ships in two
languages with identical output.

## Install

```bash
# TypeScript / JavaScript
npm install @tabnas/feed @tabnas/parser @tabnas/jsonic @tabnas/xml

# Go
go get github.com/tabnas/feed/go
```

## One tiny example

**TypeScript** — hand in RSS, get Atom shape out:

```ts
import { Tabnas } from '@tabnas/parser'
import { jsonic } from '@tabnas/jsonic'
import { Feed } from '@tabnas/feed'

const j = new Tabnas().use(jsonic).use(Feed)
const feed = j.parse('<rss version="2.0"><channel><title>My Blog</title></channel></rss>')

feed.title    // { type: 'text', value: 'My Blog' }
feed.format   // 'atom'
```

**Go** — the same, returning a typed struct:

```go
j := tabnasjsonic.Make()
j.UseDefaults(tabnasfeed.Feed, tabnasfeed.Defaults)
got, _ := j.Parse(`<rss version="2.0"><channel><title>My Blog</title></channel></rss>`)
f := got.(tabnasfeed.AtomFeed)
fmt.Println(f.Title.Value) // My Blog
```

The input was RSS 2.0 but the result is in Atom shape — `title` is an
`AtomText` (`{ type, value }`), and the whole object follows RFC 4287.
Pass `{ format: 'native' }` to keep the source dialect's structure, or
`{ format: 'raw' }` for the underlying XML element tree.

## Documentation

Full docs follow the four [Diátaxis](https://diataxis.fr) quadrants,
one file each, per language:

| Quadrant   | TypeScript                              | Go                                      |
| ---------- | --------------------------------------- | --------------------------------------- |
| Tutorial   | [ts/doc/tutorial.md](ts/doc/tutorial.md) | [go/doc/tutorial.md](go/doc/tutorial.md) |
| How-to     | [ts/doc/guide.md](ts/doc/guide.md)       | [go/doc/guide.md](go/doc/guide.md)       |
| Reference  | [ts/doc/reference.md](ts/doc/reference.md) | [go/doc/reference.md](go/doc/reference.md) |
| Concepts   | [ts/doc/concepts.md](ts/doc/concepts.md) | [go/doc/concepts.md](go/doc/concepts.md) |

Language hubs: [`ts/README.md`](ts/README.md) and
[`go/README.md`](go/README.md).

## Repository layout

| Path                                                            | Description                                  |
| -------------------------------------------------------------- | -------------------------------------------- |
| [`ts/`](ts/)                                                   | Canonical TypeScript / JavaScript implementation. |
| [`go/`](go/)                                                   | Go port (`github.com/tabnas/feed/go`).       |
| [`test/specs/`](test/specs/)                                  | Cross-language fixtures, run by both runtimes. |
| [`test/feedparser-wellformed/`](test/feedparser-wellformed/)  | Vendored well-formed corpus (BSD 2-Clause).  |

## Grammar diagram

`@tabnas/feed` contributes no grammar of its own — the accepted syntax
is the XML grammar from `@tabnas/xml`. The installed grammar as a
railroad / syntax diagram, generated with
[`@tabnas/railroad`](https://github.com/tabnas/railroad):

![feed grammar railroad diagram](ts/doc/grammar.svg)

A vertical ASCII version is in [`ts/doc/grammar.txt`](ts/doc/grammar.txt).

## License

MIT. Copyright (c) Richard Rodger and contributors.
