# Reference

Complete, dry reference for `@tabnas/feed`: the public API, the single
option, the result types, the dialect-to-Atom mapping tables, and the
grammar (syntax) the plugin accepts. For a guided introduction see the
[tutorial](tutorial.md); for task recipes see the [how-to
guide](guide.md); for rationale see [concepts](concepts.md).

## Exports

`@tabnas/feed` exports one value plus one function and a set of types:

| Export   | Kind     | Description                                       |
| -------- | -------- | ------------------------------------------------- |
| `Feed`   | `Plugin` | The plugin to register on a `Tabnas` engine.      |
| `detect` | function | `detect(root: XmlElement) => { dialect, version }`. |

Exported types: `FeedFormat`, `FeedDialect`, `FeedVersion`,
`FeedOptions`, `FeedResult`, `AtomFeed`, `AtomEntry`, `AtomText`,
`AtomPerson`, `AtomLink`, `AtomCategory`, `AtomGenerator`,
`AtomContent`, `Rss2Feed`, `Rss2Item`, `Rss2Category`, `Rss2Enclosure`,
`Rss2Guid`, `Rss2Source`, `Rss2Image`, `Rss2Cloud`, `Rss2TextInput`,
`Rss1Feed`, `Rss1Item`, `Rss1Image`, `Rss1TextInput`.

## Registration and the parse entry

`Feed` has no factory of its own — it is registered on a `Tabnas`
engine instance with `.use()`. Parsing is the engine's `parse` method;
there is no separate feed-parse function.

```ts
import { Tabnas } from '@tabnas/parser'
import { jsonic } from '@tabnas/jsonic'
import { Feed } from '@tabnas/feed'

const j = new Tabnas()
  .use(jsonic)              // lexer + relaxed grammar
  .use(Feed, options?)     // pulls in @tabnas/xml, hooks conversion

const result = j.parse(source)   // FeedResult
```

`.use(jsonic)` must come first (it installs the lexer the XML grammar
needs). `Feed` calls `tn.use(Xml)` internally, so you do **not** have
to register `@tabnas/xml` yourself — though installing it as a
dependency is required.

## Options

`FeedOptions` has exactly one field.

| Key      | Type                          | Default  | Effect                                            |
| -------- | ----------------------------- | -------- | ------------------------------------------------- |
| `format` | `'atom' \| 'native' \| 'raw'` | `'atom'` | Output shape (see below).                         |

```ts
type FeedFormat = 'atom' | 'native' | 'raw'
type FeedOptions = { format?: FeedFormat }
```

- `'atom'` (default) — every dialect is normalised to a single
  Atom-shaped `AtomFeed`.
- `'native'` — the dialect-specific structure, with no cross-dialect
  normalisation: `AtomFeed`, `Rss2Feed`, or `Rss1Feed`.
- `'raw'` — the underlying `XmlElement` tree from `@tabnas/xml`, with
  no feed interpretation at all.

An unknown `format` string is not validated; pass only the three
documented values.

## Result types by `format`

`parse` returns a `FeedResult`. The `format` option fixes which member:

```ts
type FeedResult = AtomFeed | Rss2Feed | Rss1Feed | XmlElement
```

| `format`   | Returned type                            | Discriminant       |
| ---------- | ---------------------------------------- | ------------------ |
| `'atom'`   | `AtomFeed`                               | `format === 'atom'`|
| `'native'` | `AtomFeed \| Rss2Feed \| Rss1Feed`       | `format` field     |
| `'raw'`    | `XmlElement` (from `@tabnas/xml`)        | `localName` etc.   |

For `'native'`, narrow on the `format` field: `'atom'` → `AtomFeed`,
`'rss'` → `Rss2Feed`, `'rdf'` → `Rss1Feed`.

## `detect`

```ts
function detect(root: XmlElement): {
  dialect: 'atom' | 'rss' | 'rdf' | 'unknown'
  version:
    | 'atom10' | 'atom03'
    | 'rss20' | 'rss092' | 'rss091u' | 'rss091n'
    | 'rss10' | 'rss090'
    | 'unknown'
}
```

`detect` inspects a raw `XmlElement` root (use `format: 'raw'` to
obtain one) and reports its dialect and version. It never throws; an
unrecognised root returns `{ dialect: 'unknown', version: 'unknown' }`.

How the version is decided:

| Root element | Condition                                     | Result                  |
| ------------ | --------------------------------------------- | ----------------------- |
| `feed`       | namespace `http://purl.org/atom/ns#`          | `atom` / `atom03`       |
| `feed`       | otherwise                                     | `atom` / `atom10`       |
| `rss`        | `@version` `2.0` / `2.0.1` / `2.0.2`          | `rss` / `rss20`         |
| `rss`        | `@version` `0.92` / `0.93` / `0.94`           | `rss` / `rss092`        |
| `rss`        | `@version` `0.91`                             | `rss` / `rss091u`       |
| `rss`        | any other / missing `@version`                | `rss` / `rss20`         |
| `RDF`        | channel namespace `http://my.netscape.com/rdf/simple/0.9/` | `rdf` / `rss090` |
| `RDF`        | otherwise                                     | `rdf` / `rss10`         |
| anything else| —                                             | `unknown` / `unknown`   |

`rss091n` (the Netscape 0.91 variant) is a possible `FeedVersion`
value but is not produced by `detect`: bare `version="0.91"` is
reported as `rss091u` (Userland), the dominant variant in the wild.

## Atom shape (RFC 4287)

The default and most-used output. `format` and `version` are always
present; everything else is optional and omitted when absent.

```ts
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

`AtomText.type` distinguishes plain `text` from `html` (markup as a
string) and `xhtml` (the inner body of the Atom `<div>`, re-serialised
to a string).

## Native types

Used only with `format: 'native'`. The full field lists are in
[`src/feed.ts`](../src/feed.ts); the headline shapes:

```ts
type Rss2Feed = {
  format: 'rss'
  version: '2.0' | '0.92' | '0.91' | string
  title: string; link: string; description: string
  language?: string; copyright?: string
  managingEditor?: string; webMaster?: string
  pubDate?: string; lastBuildDate?: string
  categories?: Rss2Category[]; generator?: string; docs?: string
  cloud?: Rss2Cloud; ttl?: number
  image?: Rss2Image; textInput?: Rss2TextInput
  skipHours?: number[]; skipDays?: string[]
  items: Rss2Item[]
}

type Rss2Item = {
  title?: string; link?: string; description?: string; author?: string
  categories?: Rss2Category[]; comments?: string
  enclosure?: Rss2Enclosure; guid?: Rss2Guid
  pubDate?: string; source?: Rss2Source
}

type Rss1Feed = {
  format: 'rdf'
  version: '1.0' | '0.90' | string
  about?: string; title: string; link: string; description?: string
  image?: Rss1Image; textInput?: Rss1TextInput
  items: Rss1Item[]
}

type Rss1Item = {
  about?: string; title: string; link: string; description?: string
}
```

## Mapping: RSS 2.x / 0.92 / 0.91 → Atom

| RSS source               | Atom target                                          |
| ------------------------ | ---------------------------------------------------- |
| `channel/title`          | `feed.title` (`type: 'text'`)                        |
| `channel/description`    | `feed.subtitle` (`type: 'text'`)                     |
| `channel/link`           | `feed.id` and `feed.links[]` (`rel: 'alternate'`)    |
| `channel/copyright`      | `feed.rights`                                        |
| `channel/lastBuildDate`  | `feed.updated`                                       |
| `channel/pubDate`        | `feed.updated` (fallback when no `lastBuildDate`)    |
| `channel/managingEditor` | `feed.authors[0]` (parsed `email (Name)`; else `webMaster`) |
| `channel/generator`      | `feed.generator.value`                               |
| `channel/image/url`      | `feed.logo`                                          |
| `channel/category`       | `feed.categories[].term` (+ `scheme` from `@domain`) |
| `item/guid`              | `entry.id`                                           |
| `item/link` (no `guid`)  | `entry.id` (fallback) and `entry.links[]` (`rel: 'alternate'`) |
| `item/description`       | `entry.summary` (`type: 'html'`)                     |
| `item/pubDate`           | `entry.published` *and* `entry.updated`              |
| `item/author`            | `entry.authors[0]` (parsed `email (Name)`)           |
| `item/enclosure`         | `entry.links[]` with `rel: 'enclosure'`              |
| `item/comments`          | `entry.links[]` with `rel: 'replies'`, `type: 'text/html'` |
| `item/category`          | `entry.categories[].term` (+ `scheme` from `@domain`) |
| `item/source`            | `entry.source` (slim Atom feed: `title`, self `link`) |

## Mapping: RDF (RSS 1.0 / 0.90) → Atom

| RDF source            | Atom target                          |
| --------------------- | ------------------------------------ |
| `channel/@rdf:about`  | `feed.id`                            |
| `channel/title`       | `feed.title`                         |
| `channel/description` | `feed.subtitle`                      |
| `channel/link`        | `feed.links[]` (`rel: 'alternate'`)  |
| `image/url`           | `feed.logo`                          |
| `item/@rdf:about`     | `entry.id` (else `item/link`)        |
| `item/title`          | `entry.title`                        |
| `item/description`    | `entry.summary` (`type: 'text'`)     |
| `item/link`           | `entry.links[]` (`rel: 'alternate'`) |

## Atom 0.3 → 1.0 element renames

When reading Atom 0.3 the plugin maps the old element names onto the
1.0 shape, so a 0.3 feed produces the same `AtomFeed` keys (with
`version: '0.3'`):

| Atom 0.3    | Atom 1.0    |
| ----------- | ----------- |
| `tagline`   | `subtitle`  |
| `modified`  | `updated`   |
| `issued`    | `published` |
| `copyright` | `rights`    |

## Grammar (accepted syntax)

`@tabnas/feed` contributes **no grammar rules of its own**. The syntax
it accepts is exactly the XML grammar from
[`@tabnas/xml`](https://github.com/tabnas/xml): a feed is an XML
document whose single root element is `<feed>`, `<rss>`, or `<RDF>`.
The feed plugin hooks the XML grammar's close phase and runs detection
plus conversion on the resulting element tree — it never changes how
text is tokenised or how elements nest.

The grammar rules (`xml`, `element`, `content`, `child`) and their
tokens:

```
xml      = ( "TX" / element )*
element  = "XSC" / ( "XOP" content "XCL" )
content  = child*
child    = "TX" / element
```

| Token | Meaning                                         |
| ----- | ----------------------------------------------- |
| `TX`  | text / character data between tags (entities decoded) |
| `XOP` | XML open tag `<name ...>`                        |
| `XCL` | XML closing tag `</name>`                        |
| `XSC` | XML self-closing tag `<name .../>`              |

Ignored tokens: `CM` (comment), `SP` (whitespace), `LN` (newline),
`XIG` (ignored markup — comment, processing instruction, or DOCTYPE).

The railroad / syntax diagram is in
[`grammar.svg`](grammar.svg) (vertical ASCII in
[`grammar.txt`](grammar.txt)). It is the diagram of the installed XML
grammar, regenerated with
[`@tabnas/railroad`](https://github.com/tabnas/railroad).

Accepted root elements and what each becomes:

| Root      | Dialects                              |
| --------- | ------------------------------------- |
| `<feed>`  | Atom 1.0 (default), Atom 0.3 (by namespace) |
| `<rss>`   | RSS 2.0 (default), 0.92, 0.91 (by `@version`) |
| `<RDF>`   | RSS 1.0 (default), 0.90 (by channel namespace) |

Any other root element throws
`unrecognized root element "<name>"; expected one of feed, rss, RDF`
(unless `format: 'raw'`, which returns the tree without detection).
