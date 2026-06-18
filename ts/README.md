# @tabnas/feed

A plugin for the [`tabnas`](https://github.com/tabnas/parser) parsing
engine — built on [`@tabnas/xml`](https://github.com/tabnas/xml) — that
parses syndication feeds (**RSS 0.90, 0.91, 0.92, 1.0, 2.0** and **Atom
0.3, 1.0**) into a typed structure. By default every dialect is
normalised to an Atom-shaped result, so the same downstream code can
consume feeds from any source.

[![npm version](https://img.shields.io/npm/v/@tabnas/feed.svg)](https://npmjs.com/package/@tabnas/feed)
[![build](https://github.com/tabnas/feed/actions/workflows/build.yml/badge.svg)](https://github.com/tabnas/feed/actions/workflows/build.yml)

| ![Voxgig](https://www.voxgig.com/res/img/vgt01r.png) | This open source module is sponsored and supported by [Voxgig](https://www.voxgig.com). |
| ---------------------------------------------------- | --------------------------------------------------------------------------------------- |

## Install

```bash
npm install @tabnas/feed @tabnas/parser @tabnas/jsonic @tabnas/xml
```

## One tiny example

```js
const { Tabnas } = require('@tabnas/parser')
const { jsonic } = require('@tabnas/jsonic')
const { Feed } = require('@tabnas/feed')

const j = new Tabnas().use(jsonic).use(Feed)
const feed = j.parse('<rss version="2.0"><channel><title>My Blog</title></channel></rss>')

feed.title    // => { type: 'text', value: 'My Blog' }
feed.format   // => 'atom'
```

The input was RSS 2.0 but the result is in Atom shape — `title` is an
`AtomText` (`{ type, value }`), and the whole object follows RFC 4287.
Pass `{ format: 'native' }` to keep the source dialect's structure, or
`{ format: 'raw' }` for the underlying `XmlElement` tree.

## Documentation

Full docs follow the four [Diátaxis](https://diataxis.fr) quadrants:

- [Tutorial](doc/tutorial.md) — your first feed parse, step by step.
- [How-to guide](doc/guide.md) — recipes: native shape, raw tree,
  dialect detection, error handling.
- [Reference](doc/reference.md) — the API, the `format` option, the
  Atom/native types, mapping tables, and the accepted grammar.
- [Concepts](doc/concepts.md) — why it defaults to Atom, what
  conversion loses, and how it rides on `@tabnas/xml`.

The Go port lives in [`../go/`](../go/) with its own
[docs](../go/doc/); the project [main README](../README.md) covers both
languages.

## License

MIT. Copyright (c) Richard Rodger and contributors.
