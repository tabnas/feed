# Reference (Go)

Complete, dry reference for the `feed` Go package: the public API, the
single option, the result types, the dialect-to-Atom mapping tables,
and the grammar (syntax) the plugin accepts. For a guided introduction
see the [tutorial](tutorial.md); for task recipes see the [how-to
guide](guide.md); for rationale and TS differences see
[concepts](concepts.md).

Import path: `github.com/tabnas/feed/go`, package name `feed`.

## Public API

| Symbol                                | Kind     | Description                                  |
| ------------------------------------- | -------- | -------------------------------------------- |
| `Feed(j *tabnasjsonic.Jsonic, opts map[string]any) error` | func | Plugin entry point; register with `UseDefaults`. |
| `Defaults`                            | `map[string]any` | `{"format": "atom"}` — merged with caller options. |
| `Detect(root any) Detection`          | func     | Report dialect/version of a raw XML root.    |
| `Convert(root any, format string) (any, error)` | func | Turn a raw element tree into the requested shape. |
| `Version`                             | `string` | Package version (`"0.1.0"`), mirrors `ts/package.json`. |

Public types: `Detection`, `AtomFeed`, `AtomEntry`, `AtomEntrySource`,
`AtomText`, `AtomPerson`, `AtomLink`, `AtomCategory`, `AtomGenerator`,
`AtomContent`, `Rss2Feed`, `Rss2Item`, `Rss2Category`, `Rss2Enclosure`,
`Rss2Guid`, `Rss2Source`, `Rss2Image`, `Rss2Cloud`, `Rss2TextInput`,
`Rss1Feed`, `Rss1Item`, `Rss1Image`, `Rss1TextInput`.

## Registration and the parse entry

`Feed` has no factory of its own — register it on a `jsonic` instance
with `UseDefaults`. Parsing is the instance's `Parse` method.

```go
j := tabnasjsonic.Make()
err := j.UseDefaults(tabnasfeed.Feed, tabnasfeed.Defaults, opts) // opts optional
result, err := j.Parse(source)                        // (any, error)
```

`UseDefaults` merges each supplied option map over the preceding ones
(so `Defaults` first, then caller `opts`), giving caller options
precedence. Pass no extra map for defaults-only. `Feed` calls
`j.UseDefaults(tabnasxml.Xml, tabnasxml.Defaults)` internally, so the XML plugin is
installed for you — you still need `github.com/tabnas/xml/go` as a
dependency. `Feed` is idempotent: it short-circuits if already applied
to the instance.

## Options

Exactly one option is supported.

| Key      | Type     | Default  | Effect                                          |
| -------- | -------- | -------- | ----------------------------------------------- |
| `format` | `string` | `"atom"` | Output shape (see below).                       |

```go
var Defaults = map[string]any{"format": "atom"}
```

- `"atom"` (default) — every dialect is normalised to a single
  `tabnasfeed.AtomFeed`.
- `"native"` — the dialect-specific struct, with no cross-dialect
  normalisation: `tabnasfeed.AtomFeed`, `tabnasfeed.Rss2Feed`, or `tabnasfeed.Rss1Feed`.
- `"raw"` — the underlying `map[string]any` element tree from the XML
  plugin, with no feed interpretation.

An empty or absent `format` defaults to `"atom"`. Any other string is
treated as `"atom"` by the conversion (it returns the normalised shape).

## Result types by `format`

`Parse` returns `(any, error)`. The `format` option fixes the concrete
type behind the `any`:

| `format`   | Concrete type returned by `Parse`                   |
| ---------- | --------------------------------------------------- |
| `"atom"`   | `tabnasfeed.AtomFeed`                                     |
| `"native"` | `tabnasfeed.AtomFeed` / `tabnasfeed.Rss2Feed` / `tabnasfeed.Rss1Feed` |
| `"raw"`    | `map[string]any` (the XML element tree)             |

For `"native"`, type-switch on the result (or assert when you know the
dialect): `"atom"` → `AtomFeed`, `"rss"` → `Rss2Feed`, `"rdf"` →
`Rss1Feed`.

## `Detect` and `Convert`

```go
func Detect(root any) Detection
type Detection struct {
    Dialect string `json:"dialect"`
    Version string `json:"version"`
}
```

`Detect` accepts the `map[string]any` element shape (use
`format: "raw"`). It never errors; an unrecognised root returns
`Detection{Dialect: "unknown", Version: "unknown"}`.

`Dialect` is one of `"atom"`, `"rss"`, `"rdf"`, `"unknown"`. `Version`
is one of `"atom10"`, `"atom03"`, `"rss20"`, `"rss092"`, `"rss091u"`,
`"rss091n"`, `"rss10"`, `"rss090"`, `"unknown"`.

How the version is decided:

| Root element | Condition                                     | Result            |
| ------------ | --------------------------------------------- | ----------------- |
| `feed`       | namespace `http://purl.org/atom/ns#`          | `atom` / `atom03` |
| `feed`       | otherwise                                     | `atom` / `atom10` |
| `rss`        | `version` `2.0` / `2.0.1` / `2.0.2`           | `rss` / `rss20`   |
| `rss`        | `version` `0.92` / `0.93` / `0.94`            | `rss` / `rss092`  |
| `rss`        | `version` `0.91`                              | `rss` / `rss091u` |
| `rss`        | any other / missing `version`                 | `rss` / `rss20`   |
| `RDF`        | channel namespace `http://my.netscape.com/rdf/simple/0.9/` | `rdf` / `rss090` |
| `RDF`        | otherwise                                     | `rdf` / `rss10`   |
| anything else| —                                             | `unknown` / `unknown` |

`rss091n` is a possible `Version` value but is not produced by
`Detect`; bare `version="0.91"` is reported as `rss091u`.

```go
func Convert(root any, format string) (any, error)
```

`Convert` is what the plugin runs internally: it turns a raw element
tree into the requested shape. `format: "raw"` returns `root`
unchanged; an unrecognised root yields an error.

## Atom shape (RFC 4287)

The default and most-used output. JSON tags are chosen so
`json.Marshal` matches the TypeScript `JSON.stringify` shape.

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

`AtomText.Type` distinguishes plain `text` from `html` (markup as a
string) and `xhtml` (the inner body of the Atom `<div>`, re-serialised
to a string). `AtomEntrySource` is a slim `AtomFeed` (no `Entries`)
used for the Atom-source element an RSS `<item>/<source>` maps to.

Pointer fields (`*AtomText`, `*AtomGenerator`, `*AtomContent`) are
`nil` when the source omits them — always nil-check before
dereferencing.

## Native types

Used only with `format: "native"`. See [`tabnasfeed.go`](../tabnasfeed.go) for the
full field lists; the headline shapes:

```go
type Rss2Feed struct {
    Format, Version                       string
    Title, Link, Description              string
    Language, Copyright                   string
    ManagingEditor, WebMaster             string
    PubDate, LastBuildDate                string
    Categories                            []Rss2Category
    Generator, Docs                       string
    Cloud                                 *Rss2Cloud
    TTL                                   int
    Image                                 *Rss2Image
    TextInput                             *Rss2TextInput
    SkipHours                             []int
    SkipDays                              []string
    Items                                 []Rss2Item
}

type Rss2Item struct {
    Title, Link, Description, Author string
    Categories                       []Rss2Category
    Comments                         string
    Enclosure                        *Rss2Enclosure
    Guid                             *Rss2Guid
    PubDate                          string
    Source                           *Rss2Source
}

type Rss1Feed struct {
    Format, Version           string
    About, Title, Link, Description string
    Image                     *Rss1Image
    TextInput                 *Rss1TextInput
    Items                     []Rss1Item
}

type Rss1Item struct { About, Title, Link, Description string }
```

## Mapping: RSS 2.x / 0.92 / 0.91 → Atom

| RSS source               | Atom target                                          |
| ------------------------ | ---------------------------------------------------- |
| `channel/title`          | `Title` (`Type: "text"`)                             |
| `channel/description`    | `Subtitle` (`Type: "text"`)                          |
| `channel/link`           | `ID` and `Links[]` (`Rel: "alternate"`)              |
| `channel/copyright`      | `Rights`                                             |
| `channel/lastBuildDate`  | `Updated`                                            |
| `channel/pubDate`        | `Updated` (fallback when no `lastBuildDate`)         |
| `channel/managingEditor` | `Authors[0]` (parsed `email (Name)`; else `webMaster`) |
| `channel/generator`      | `Generator.Value`                                    |
| `channel/image/url`      | `Logo`                                               |
| `channel/category`       | `Categories[].Term` (+ `Scheme` from `domain`)       |
| `item/guid`              | `Entries[i].ID`                                      |
| `item/link` (no `guid`)  | `Entries[i].ID` (fallback) and `Links[]`             |
| `item/description`       | `Entries[i].Summary` (`Type: "html"`)                |
| `item/pubDate`           | `Entries[i].Published` *and* `Updated`               |
| `item/author`            | `Entries[i].Authors[0]` (parsed `email (Name)`)      |
| `item/enclosure`         | `Entries[i].Links[]` with `Rel: "enclosure"`         |
| `item/comments`          | `Entries[i].Links[]` with `Rel: "replies"`, `Type: "text/html"` |
| `item/category`          | `Entries[i].Categories[].Term` (+ `Scheme` from `domain`) |
| `item/source`            | `Entries[i].Source` (slim Atom feed: `Title`, self `Link`) |

## Mapping: RDF (RSS 1.0 / 0.90) → Atom

| RDF source            | Atom target                          |
| --------------------- | ------------------------------------ |
| `channel/@rdf:about`  | `ID`                                 |
| `channel/title`       | `Title`                              |
| `channel/description` | `Subtitle`                           |
| `channel/link`        | `Links[]` (`Rel: "alternate"`)       |
| `image/url`           | `Logo`                               |
| `item/@rdf:about`     | `Entries[i].ID` (else `item/link`)   |
| `item/title`          | `Entries[i].Title`                   |
| `item/description`    | `Entries[i].Summary` (`Type: "text"`)|
| `item/link`           | `Entries[i].Links[]` (`Rel: "alternate"`) |

## Atom 0.3 → 1.0 element renames

Reading Atom 0.3 maps the old element names onto the 1.0 struct, so a
0.3 feed produces the same `AtomFeed` fields (with `Version: "0.3"`):

| Atom 0.3    | Atom 1.0    |
| ----------- | ----------- |
| `tagline`   | `subtitle`  |
| `modified`  | `updated`   |
| `issued`    | `published` |
| `copyright` | `rights`    |

## Grammar (accepted syntax)

`feed` contributes **no grammar rules of its own**. The syntax it
accepts is exactly the XML grammar from
[`github.com/tabnas/xml/go`](https://github.com/tabnas/xml): a feed is
an XML document whose single root element is `<feed>`, `<rss>`, or
`<RDF>`. The feed plugin hooks the XML grammar's close phase and runs
detection plus conversion on the resulting element tree.

The grammar rules (`xml`, `element`, `content`, `child`) and tokens:

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
| `XSC` | XML self-closing tag `<name .../>`             |

Ignored tokens: `CM` (comment), `SP` (whitespace), `LN` (newline),
`XIG` (ignored markup — comment, processing instruction, or DOCTYPE).

The railroad / syntax diagram is in
[`../../ts/doc/grammar.svg`](../../ts/doc/grammar.svg) (vertical ASCII
in [`../../ts/doc/grammar.txt`](../../ts/doc/grammar.txt)).

Accepted root elements:

| Root      | Dialects                              |
| --------- | ------------------------------------- |
| `<feed>`  | Atom 1.0 (default), Atom 0.3 (by namespace) |
| `<rss>`   | RSS 2.0 (default), 0.92, 0.91 (by `version`) |
| `<RDF>`   | RSS 1.0 (default), 0.90 (by channel namespace) |

Any other root element makes `Parse` return an error:
`feed: unrecognized root element "<name>"; expected one of feed, rss,
RDF` (unless `format: "raw"`, which returns the tree without detection).
