# Tutorial — your first feed parse

This walks you from nothing to a parsed feed, then shows you the one
idea that makes `@tabnas/feed` useful: whatever dialect you feed in,
you get the *same* Atom-shaped object out. Follow it in order — each
step builds on the last.

For a recipe-style index of individual tasks, see the
[how-to guide](guide.md). For exhaustive signatures, options, and the
grammar, see the [reference](reference.md). For how it all works, see
[concepts](concepts.md).

## 1. Install

`@tabnas/feed` is a plugin for the [`tabnas`](https://github.com/tabnas/parser)
parsing engine. Install it together with the engine, the jsonic
grammar it builds on, and the XML plugin it layers over:

```bash
npm install @tabnas/feed @tabnas/parser @tabnas/jsonic @tabnas/xml
```

## 2. Parse a feed

Register the plugin on a `Tabnas` engine instance and call `parse`:

```js
const { Tabnas } = require('@tabnas/parser')
const { jsonic } = require('@tabnas/jsonic')
const { Feed } = require('@tabnas/feed')

const j = new Tabnas().use(jsonic).use(Feed)

const result = j.parse(`
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
  </rss>
`)

result.title              // => { type: 'text', value: 'My Blog' }
result.entries.length     // => 1
```

In TypeScript the import is the same, and you can type the result:

```ts
import { Tabnas } from '@tabnas/parser'
import { jsonic } from '@tabnas/jsonic'
import { Feed, type AtomFeed } from '@tabnas/feed'

const j = new Tabnas().use(jsonic).use(Feed)
const result = j.parse(rssSource) as AtomFeed
```

## 3. Read the result — it is Atom-shaped

You handed in **RSS 2.0**, but `result` came back in **Atom shape**.
That is the whole point: the plugin normalises every dialect to one
structure so the rest of your code never branches on the source format.

Notice three things in the output above:

- `result.title` is an `AtomText` — `{ type, value }` — not a bare
  string. Atom text carries its content type (`text`, `html`, or
  `xhtml`).
- The RSS `<guid>` became the entry's stable `id`.
- The RSS `<link>` became an Atom link with `rel: 'alternate'`.

```js
const { Tabnas } = require('@tabnas/parser')
const { jsonic } = require('@tabnas/jsonic')
const { Feed } = require('@tabnas/feed')

const j = new Tabnas().use(jsonic).use(Feed)
const result = j.parse(
  '<rss version="2.0"><channel><title>My Blog</title>' +
  '<link>https://example.com/</link><description>Posts</description>' +
  '<item><title>Hello</title><link>https://example.com/1</link>' +
  '<guid>https://example.com/1</guid></item></channel></rss>'
)

result.entries[0].id           // => 'https://example.com/1'
result.entries[0].links[0]     // => { href: 'https://example.com/1', rel: 'alternate' }
```

## 4. Parse a different dialect — same shape out

Now hand in an **Atom 1.0** document. The output object has the same
shape, so your reading code does not change:

```js
const { Tabnas } = require('@tabnas/parser')
const { jsonic } = require('@tabnas/jsonic')
const { Feed } = require('@tabnas/feed')

const j = new Tabnas().use(jsonic).use(Feed)
const result = j.parse(
  '<feed xmlns="http://www.w3.org/2005/Atom">' +
  '<title>Example Feed</title>' +
  '<entry><title>T</title>' +
  '<content type="html">&lt;p&gt;hi&lt;/p&gt;</content></entry>' +
  '</feed>'
)

result.title                   // => { type: 'text', value: 'Example Feed' }
result.entries[0].content      // => { type: 'html', value: '<p>hi</p>' }
```

The HTML content was XML-escaped in the source (`&lt;p&gt;`); the
parser decodes it back to `<p>hi</p>` and tags it `type: 'html'` so
you know not to trust it as plain text.

## 5. Catch an error

If the root element is not a feed the plugin recognises, `parse`
throws. Catch it like any other exception:

```js
const { Tabnas } = require('@tabnas/parser')
const { jsonic } = require('@tabnas/jsonic')
const { Feed } = require('@tabnas/feed')

const j = new Tabnas().use(jsonic).use(Feed)

let threw = false
try {
  j.parse('<not-a-feed/>')
} catch (err) {
  threw = /unrecognized root/.test(err.message)
}
threw                          // => true
```

The recognised root elements are `<feed>` (Atom), `<rss>` (RSS
0.91/0.92/2.0), and `<RDF>` (RSS 0.90/1.0). Anything else is rejected.

## Where to go next

- [How-to guide](guide.md) — focused recipes (native shape, raw tree,
  dialect detection, error handling).
- [Reference](reference.md) — every option, the full Atom/native
  types, the dialect mapping tables, and the accepted grammar.
- [Concepts](concepts.md) — why it defaults to Atom, what conversion
  loses, and how the plugin rides on `@tabnas/xml`.
