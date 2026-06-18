# How-to guide

Short, task-focused recipes. Each is self-contained and assumes you
have the plugin installed (see the [tutorial](tutorial.md) for the
basics). For full type lists, options, and the mapping tables, follow
the links into the [reference](reference.md).

Every recipe starts from a `Tabnas` engine with `jsonic` and `Feed`
registered:

```js
const { Tabnas } = require('@tabnas/parser')
const { jsonic } = require('@tabnas/jsonic')
const { Feed } = require('@tabnas/feed')
```

## Use it as a plugin

`Feed` is a [`tabnas`](https://github.com/tabnas/parser) plugin. Register
it after `jsonic` (it supplies the lexer/grammar) and `Feed` will pull
in `@tabnas/xml` for you. Then `parse` any feed source:

```js
const { Tabnas } = require('@tabnas/parser')
const { jsonic } = require('@tabnas/jsonic')
const { Feed } = require('@tabnas/feed')

const j = new Tabnas().use(jsonic).use(Feed)
const feed = j.parse('<rss version="2.0"><channel><title>x</title></channel></rss>')

feed.format        // => 'atom'
feed.version       // => '1.0'
```

The instance is reusable — call `j.parse(...)` as many times as you
like.

## Keep the source dialect's structure

The default Atom shape is lossy (see [what conversion
loses](concepts.md#what-conversion-loses)). When you need RSS-specific
fields like `ttl`, `cloud`, or `skipDays`, register the plugin with
`format: 'native'`:

```js
const { Tabnas } = require('@tabnas/parser')
const { jsonic } = require('@tabnas/jsonic')
const { Feed } = require('@tabnas/feed')

const j = new Tabnas().use(jsonic).use(Feed, { format: 'native' })
const native = j.parse(
  '<rss version="2.0"><channel><title>x</title>' +
  '<link>http://x</link><description>d</description>' +
  '<ttl>60</ttl></channel></rss>'
)

native.format      // => 'rss'
native.version     // => '2.0'
native.ttl         // => 60
```

The native return type is a discriminated union on `format`:

| Input dialect         | Native type | `format` | `version`            |
| --------------------- | ----------- | -------- | -------------------- |
| Atom 1.0 / 0.3        | `AtomFeed`  | `atom`   | `1.0` / `0.3`        |
| RSS 2.0 / 0.92 / 0.91 | `Rss2Feed`  | `rss`    | `2.0` / `0.92` / `0.91` |
| RSS 1.0 / 0.90        | `Rss1Feed`  | `rdf`    | `1.0` / `0.90`       |

In TypeScript, narrow on the `format` field (or assert the type when
you already know the dialect):

```ts
import { type Rss2Feed } from '@tabnas/feed'
const native = j.parse(rssSource) as Rss2Feed
// native.ttl, native.cloud, native.skipDays
```

## Access the raw XML tree

When even the native shape is not enough — for example you need a
non-standard namespace extension like `<media:content>` — drop down to
the raw element tree from `@tabnas/xml` with `format: 'raw'`:

```js
const { Tabnas } = require('@tabnas/parser')
const { jsonic } = require('@tabnas/jsonic')
const { Feed } = require('@tabnas/feed')

const j = new Tabnas().use(jsonic).use(Feed, { format: 'raw' })
const tree = j.parse('<rss version="2.0"><channel><title>x</title></channel></rss>')

tree.localName     // => 'rss'
```

The tree is an `XmlElement`: it has `name`, `localName`, `prefix`,
`namespace`, `attributes`, and a `children` array of strings (text
nodes) and nested elements.

## Detect a feed's dialect without converting

`detect` reports the dialect and version of a raw XML root. Parse with
`format: 'raw'`, then call it:

```js
const { Tabnas } = require('@tabnas/parser')
const { jsonic } = require('@tabnas/jsonic')
const { Feed, detect } = require('@tabnas/feed')

const j = new Tabnas().use(jsonic).use(Feed, { format: 'raw' })

const rss = j.parse('<rss version="2.0"><channel><title>x</title></channel></rss>')
detect(rss)   // => { dialect: 'rss', version: 'rss20' }

const atom = j.parse('<feed xmlns="http://www.w3.org/2005/Atom"/>')
detect(atom)  // => { dialect: 'atom', version: 'atom10' }
```

An unrecognised root returns `{ dialect: 'unknown', version: 'unknown' }`
rather than throwing — `detect` never raises.

## Handle parse errors

A failed parse throws. Two kinds of failure can occur:

- **XML-level** — malformed markup (unterminated tag, mismatched
  close) is rejected by `@tabnas/xml` before the feed conversion runs.
- **Feed-level** — a well-formed XML document whose root is not
  `<feed>`, `<rss>`, or `<RDF>` throws
  `unrecognized root element ...`.

```js
const { Tabnas } = require('@tabnas/parser')
const { jsonic } = require('@tabnas/jsonic')
const { Feed } = require('@tabnas/feed')

const j = new Tabnas().use(jsonic).use(Feed)

let message = ''
try {
  j.parse('<not-a-feed/>')
} catch (err) {
  message = err.message
}
/unrecognized root/.test(message)   // => true
```

## Read fields the Atom conversion drops

The default conversion is lossy by design. If your application needs
both the convenient Atom shape *and* a stray RSS-only field, the
recommended path is to register the plugin with `format: 'native'` and
do any Atom-style mapping in your own code only where you need it —
that way you never lose a field you might want. Parsing the same source
twice (once per format) also works but does the XML lexing twice.
