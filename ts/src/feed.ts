/* Copyright (c) 2021-2025 Richard Rodger and other contributors, MIT License */

// Plugin that parses RSS (0.90, 0.91, 0.92, 1.0, 2.0) and Atom (0.3, 1.0)
// feeds. Built on top of @jsonic/xml. By default all feed dialects are
// converted to a normalized Atom-shaped result; pass `{ format: 'native' }`
// to keep the source dialect's structure, or `{ format: 'raw' }` to get
// back the underlying XmlElement tree from @jsonic/xml.

import { Jsonic, Plugin } from 'jsonic'
import { Xml } from '@jsonic/xml'
import type { XmlElement } from '@jsonic/xml'


// --- Public types ---------------------------------------------------------

type FeedFormat = 'atom' | 'native' | 'raw'

type FeedDialect = 'atom' | 'rss' | 'rdf' | 'unknown'

type FeedVersion =
  | 'atom10'
  | 'atom03'
  | 'rss20'
  | 'rss092'
  | 'rss091u'
  | 'rss091n'
  | 'rss10'
  | 'rss090'
  | 'unknown'

type FeedOptions = {
  // Output shape:
  //   'atom'   - normalized Atom-shaped object (the default)
  //   'native' - dialect-specific structure (Atom*/Rss2*/Rss1*)
  //   'raw'    - the XmlElement tree from @jsonic/xml
  format?: FeedFormat
}

// Atom (RFC 4287) ----------------------------------------------------------

type AtomText = {
  type: 'text' | 'html' | 'xhtml'
  value: string
}

type AtomPerson = {
  name: string
  uri?: string
  email?: string
}

type AtomLink = {
  href: string
  rel?: string
  type?: string
  hreflang?: string
  title?: string
  length?: number
}

type AtomCategory = {
  term: string
  scheme?: string
  label?: string
}

type AtomGenerator = {
  uri?: string
  version?: string
  value: string
}

type AtomContent = {
  type: string
  src?: string
  value?: string
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

// RSS 2.0 / 0.92 / 0.91 ----------------------------------------------------

type Rss2Category = {
  domain?: string
  value: string
}

type Rss2Enclosure = {
  url: string
  length?: number
  type?: string
}

type Rss2Guid = {
  isPermaLink?: boolean
  value: string
}

type Rss2Source = {
  url?: string
  value: string
}

type Rss2Image = {
  url: string
  title: string
  link: string
  width?: number
  height?: number
  description?: string
}

type Rss2Cloud = {
  domain: string
  port: number
  path: string
  registerProcedure: string
  protocol: string
}

type Rss2TextInput = {
  title: string
  description: string
  name: string
  link: string
}

type Rss2Item = {
  title?: string
  link?: string
  description?: string
  author?: string
  categories?: Rss2Category[]
  comments?: string
  enclosure?: Rss2Enclosure
  guid?: Rss2Guid
  pubDate?: string
  source?: Rss2Source
}

type Rss2Feed = {
  format: 'rss'
  version: '2.0' | '0.92' | '0.91' | string
  title: string
  link: string
  description: string
  language?: string
  copyright?: string
  managingEditor?: string
  webMaster?: string
  pubDate?: string
  lastBuildDate?: string
  categories?: Rss2Category[]
  generator?: string
  docs?: string
  cloud?: Rss2Cloud
  ttl?: number
  image?: Rss2Image
  textInput?: Rss2TextInput
  skipHours?: number[]
  skipDays?: string[]
  items: Rss2Item[]
}

// RSS 1.0 / 0.90 (RDF) -----------------------------------------------------

type Rss1Item = {
  about?: string
  title: string
  link: string
  description?: string
}

type Rss1Image = {
  about?: string
  title: string
  link: string
  url: string
}

type Rss1TextInput = {
  about?: string
  title: string
  description: string
  name: string
  link: string
}

type Rss1Feed = {
  format: 'rdf'
  version: '1.0' | '0.90' | string
  about?: string
  title: string
  link: string
  description?: string
  image?: Rss1Image
  textInput?: Rss1TextInput
  items: Rss1Item[]
}

type FeedResult = AtomFeed | Rss2Feed | Rss1Feed | XmlElement


// --- Namespaces -----------------------------------------------------------

const NS_ATOM_10 = 'http://www.w3.org/2005/Atom'
const NS_ATOM_03 = 'http://purl.org/atom/ns#'
const NS_RSS_10 = 'http://purl.org/rss/1.0/'
const NS_RSS_090 = 'http://my.netscape.com/rdf/simple/0.9/'
const NS_RDF = 'http://www.w3.org/1999/02/22-rdf-syntax-ns#'


// --- XmlElement helpers ---------------------------------------------------

function isElement(node: unknown): node is XmlElement {
  return (
    !!node &&
    typeof node === 'object' &&
    'localName' in (node as object) &&
    'children' in (node as object)
  )
}

function findChild(
  el: XmlElement | undefined,
  localName: string,
  namespace?: string,
): XmlElement | undefined {
  if (!el) return undefined
  for (const c of el.children) {
    if (
      isElement(c) &&
      c.localName === localName &&
      (namespace === undefined || c.namespace === namespace)
    ) {
      return c
    }
  }
  return undefined
}

function findChildren(
  el: XmlElement | undefined,
  localName: string,
  namespace?: string,
): XmlElement[] {
  const out: XmlElement[] = []
  if (!el) return out
  for (const c of el.children) {
    if (
      isElement(c) &&
      c.localName === localName &&
      (namespace === undefined || c.namespace === namespace)
    ) {
      out.push(c)
    }
  }
  return out
}

// Concatenate text node contents (ignores nested elements).
function textOf(el: XmlElement | undefined): string {
  if (!el) return ''
  let out = ''
  for (const c of el.children) {
    if (typeof c === 'string') out += c
  }
  return out.trim()
}

// Serialize children (text + nested elements) back to a string. Used for
// xhtml/html content where the full body matters.
function innerXml(el: XmlElement | undefined): string {
  if (!el) return ''
  let out = ''
  for (const c of el.children) {
    if (typeof c === 'string') out += c
    else out += serializeElement(c)
  }
  return out
}

function serializeElement(el: XmlElement): string {
  const attrs = Object.entries(el.attributes || {})
    .map(([k, v]) => ` ${k}="${escapeAttr(v)}"`)
    .join('')
  const inner = innerXml(el)
  if (inner === '') return `<${el.name}${attrs}/>`
  return `<${el.name}${attrs}>${inner}</${el.name}>`
}

function escapeAttr(v: string): string {
  return String(v)
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}


// --- Format detection -----------------------------------------------------

function detect(root: XmlElement): { dialect: FeedDialect; version: FeedVersion } {
  const name = root.localName

  if (name === 'feed') {
    if (root.namespace === NS_ATOM_03) return { dialect: 'atom', version: 'atom03' }
    return { dialect: 'atom', version: 'atom10' }
  }

  if (name === 'rss') {
    const v = (root.attributes && root.attributes.version) || ''
    if (v === '2.0' || v === '2.0.1' || v === '2.0.2') {
      return { dialect: 'rss', version: 'rss20' }
    }
    if (v === '0.92' || v === '0.93' || v === '0.94') {
      return { dialect: 'rss', version: 'rss092' }
    }
    if (v === '0.91') {
      // Distinguish Userland vs. Netscape variants by DOCTYPE; absent that,
      // assume Userland (the dominant one in the wild).
      return { dialect: 'rss', version: 'rss091u' }
    }
    return { dialect: 'rss', version: 'rss20' }
  }

  if (name === 'RDF') {
    // Look at the channel's namespace to pick RSS 1.0 vs 0.90.
    const channel = findChild(root, 'channel')
    if (channel?.namespace === NS_RSS_090) {
      return { dialect: 'rdf', version: 'rss090' }
    }
    return { dialect: 'rdf', version: 'rss10' }
  }

  return { dialect: 'unknown', version: 'unknown' }
}


// --- Atom (native) parser -------------------------------------------------

function parsePerson(el: XmlElement): AtomPerson {
  return {
    name: textOf(findChild(el, 'name')),
    uri: textOf(findChild(el, 'uri')) || textOf(findChild(el, 'url')) || undefined,
    email: textOf(findChild(el, 'email')) || undefined,
  }
}

function parseLink(el: XmlElement): AtomLink {
  const a = el.attributes || {}
  const link: AtomLink = { href: a.href || '' }
  if (a.rel) link.rel = a.rel
  if (a.type) link.type = a.type
  if (a.hreflang) link.hreflang = a.hreflang
  if (a.title) link.title = a.title
  if (a.length) link.length = parseInt(a.length, 10)
  return link
}

function parseCategory(el: XmlElement): AtomCategory {
  const a = el.attributes || {}
  return {
    term: a.term || textOf(el),
    scheme: a.scheme,
    label: a.label,
  }
}

function parseText(el: XmlElement | undefined): AtomText | undefined {
  if (!el) return undefined
  const a = el.attributes || {}
  const type = (a.type as AtomText['type']) || 'text'
  if (type === 'xhtml') {
    // Take the inner body of the xhtml <div>, if present
    const div = findChild(el, 'div')
    return { type, value: innerXml(div || el) }
  }
  return { type, value: textOf(el) }
}

function parseContent(el: XmlElement): AtomContent {
  const a = el.attributes || {}
  const type = a.type || 'text'
  const out: AtomContent = { type }
  if (a.src) out.src = a.src
  if (type === 'xhtml') {
    const div = findChild(el, 'div')
    out.value = innerXml(div || el)
  } else {
    out.value = textOf(el)
  }
  return out
}

function parseGenerator(el: XmlElement): AtomGenerator {
  const a = el.attributes || {}
  return {
    uri: a.uri,
    version: a.version,
    value: textOf(el),
  }
}

function parseAtomEntry(el: XmlElement, atom03: boolean): AtomEntry {
  const entry: AtomEntry = {}

  const id = findChild(el, 'id')
  if (id) entry.id = textOf(id)

  const title = findChild(el, 'title')
  if (title) entry.title = parseText(title)

  // Atom 1.0 uses 'updated' / 'published'; Atom 0.3 uses 'modified' / 'issued'
  const updated = findChild(el, atom03 ? 'modified' : 'updated')
  if (updated) entry.updated = textOf(updated)
  const published = findChild(el, atom03 ? 'issued' : 'published')
  if (published) entry.published = textOf(published)

  const authors = findChildren(el, 'author').map(parsePerson)
  if (authors.length) entry.authors = authors
  const contributors = findChildren(el, 'contributor').map(parsePerson)
  if (contributors.length) entry.contributors = contributors

  const categories = findChildren(el, 'category').map(parseCategory)
  if (categories.length) entry.categories = categories

  const links = findChildren(el, 'link').map(parseLink)
  if (links.length) entry.links = links

  const content = findChild(el, 'content')
  if (content) entry.content = parseContent(content)

  const rights = findChild(el, atom03 ? 'copyright' : 'rights')
  if (rights) entry.rights = parseText(rights)

  const summary = findChild(el, atom03 ? 'tagline' : 'summary')
  if (summary) entry.summary = parseText(summary)

  return entry
}

function parseAtom(root: XmlElement, version: 'atom10' | 'atom03'): AtomFeed {
  const atom03 = version === 'atom03'
  const feed: AtomFeed = {
    format: 'atom',
    version: atom03 ? '0.3' : '1.0',
    entries: [],
  }

  const id = findChild(root, 'id')
  if (id) feed.id = textOf(id)

  const title = findChild(root, 'title')
  if (title) feed.title = parseText(title)

  const updated = findChild(root, atom03 ? 'modified' : 'updated')
  if (updated) feed.updated = textOf(updated)

  const authors = findChildren(root, 'author').map(parsePerson)
  if (authors.length) feed.authors = authors

  const contributors = findChildren(root, 'contributor').map(parsePerson)
  if (contributors.length) feed.contributors = contributors

  const categories = findChildren(root, 'category').map(parseCategory)
  if (categories.length) feed.categories = categories

  const links = findChildren(root, 'link').map(parseLink)
  if (links.length) feed.links = links

  const generator = findChild(root, 'generator')
  if (generator) feed.generator = parseGenerator(generator)

  const icon = findChild(root, 'icon')
  if (icon) feed.icon = textOf(icon)

  const logo = findChild(root, 'logo')
  if (logo) feed.logo = textOf(logo)

  const rights = findChild(root, atom03 ? 'copyright' : 'rights')
  if (rights) feed.rights = parseText(rights)

  const subtitle = findChild(root, atom03 ? 'tagline' : 'subtitle')
  if (subtitle) feed.subtitle = parseText(subtitle)

  feed.entries = findChildren(root, 'entry').map((e) => parseAtomEntry(e, atom03))

  return feed
}


// --- RSS 2.x / 0.91 / 0.92 (native) parser --------------------------------

function parseRss2Image(el: XmlElement): Rss2Image {
  const url = textOf(findChild(el, 'url'))
  const title = textOf(findChild(el, 'title'))
  const link = textOf(findChild(el, 'link'))
  const out: Rss2Image = { url, title, link }
  const w = textOf(findChild(el, 'width'))
  if (w) out.width = parseInt(w, 10)
  const h = textOf(findChild(el, 'height'))
  if (h) out.height = parseInt(h, 10)
  const d = textOf(findChild(el, 'description'))
  if (d) out.description = d
  return out
}

function parseRss2Cloud(el: XmlElement): Rss2Cloud {
  const a = el.attributes || {}
  return {
    domain: a.domain || '',
    port: parseInt(a.port || '0', 10),
    path: a.path || '',
    registerProcedure: a.registerProcedure || '',
    protocol: a.protocol || '',
  }
}

function parseRss2TextInput(el: XmlElement): Rss2TextInput {
  return {
    title: textOf(findChild(el, 'title')),
    description: textOf(findChild(el, 'description')),
    name: textOf(findChild(el, 'name')),
    link: textOf(findChild(el, 'link')),
  }
}

function parseRss2Item(el: XmlElement): Rss2Item {
  const item: Rss2Item = {}

  const title = findChild(el, 'title')
  if (title) item.title = textOf(title)

  const link = findChild(el, 'link')
  if (link) item.link = textOf(link)

  const desc = findChild(el, 'description')
  if (desc) item.description = textOf(desc)

  const author = findChild(el, 'author')
  if (author) item.author = textOf(author)

  const categories = findChildren(el, 'category').map((c) => ({
    domain: c.attributes?.domain,
    value: textOf(c),
  }))
  if (categories.length) item.categories = categories

  const comments = findChild(el, 'comments')
  if (comments) item.comments = textOf(comments)

  const enc = findChild(el, 'enclosure')
  if (enc) {
    const a = enc.attributes || {}
    const out: Rss2Enclosure = { url: a.url || '' }
    if (a.length) out.length = parseInt(a.length, 10)
    if (a.type) out.type = a.type
    item.enclosure = out
  }

  const guid = findChild(el, 'guid')
  if (guid) {
    const a = guid.attributes || {}
    const g: Rss2Guid = { value: textOf(guid) }
    if ('isPermaLink' in a) g.isPermaLink = a.isPermaLink !== 'false'
    item.guid = g
  }

  const pubDate = findChild(el, 'pubDate')
  if (pubDate) item.pubDate = textOf(pubDate)

  const src = findChild(el, 'source')
  if (src) {
    item.source = {
      url: src.attributes?.url,
      value: textOf(src),
    }
  }

  return item
}

function parseRss2(root: XmlElement, version: FeedVersion): Rss2Feed {
  const channel = findChild(root, 'channel') || root
  const v: Rss2Feed['version'] =
    version === 'rss092' ? '0.92' :
      version === 'rss091u' || version === 'rss091n' ? '0.91' : '2.0'

  const feed: Rss2Feed = {
    format: 'rss',
    version: v,
    title: textOf(findChild(channel, 'title')),
    link: textOf(findChild(channel, 'link')),
    description: textOf(findChild(channel, 'description')),
    items: [],
  }

  const language = findChild(channel, 'language')
  if (language) feed.language = textOf(language)
  const copyright = findChild(channel, 'copyright')
  if (copyright) feed.copyright = textOf(copyright)
  const managingEditor = findChild(channel, 'managingEditor')
  if (managingEditor) feed.managingEditor = textOf(managingEditor)
  const webMaster = findChild(channel, 'webMaster')
  if (webMaster) feed.webMaster = textOf(webMaster)
  const pubDate = findChild(channel, 'pubDate')
  if (pubDate) feed.pubDate = textOf(pubDate)
  const lastBuildDate = findChild(channel, 'lastBuildDate')
  if (lastBuildDate) feed.lastBuildDate = textOf(lastBuildDate)

  const categories = findChildren(channel, 'category').map((c) => ({
    domain: c.attributes?.domain,
    value: textOf(c),
  }))
  if (categories.length) feed.categories = categories

  const generator = findChild(channel, 'generator')
  if (generator) feed.generator = textOf(generator)

  const docs = findChild(channel, 'docs')
  if (docs) feed.docs = textOf(docs)

  const cloud = findChild(channel, 'cloud')
  if (cloud) feed.cloud = parseRss2Cloud(cloud)

  const ttl = findChild(channel, 'ttl')
  if (ttl) feed.ttl = parseInt(textOf(ttl), 10)

  const image = findChild(channel, 'image')
  if (image) feed.image = parseRss2Image(image)

  const textInput = findChild(channel, 'textInput') || findChild(channel, 'textinput')
  if (textInput) feed.textInput = parseRss2TextInput(textInput)

  const skipHours = findChild(channel, 'skipHours')
  if (skipHours) {
    feed.skipHours = findChildren(skipHours, 'hour').map((h) => parseInt(textOf(h), 10))
  }
  const skipDays = findChild(channel, 'skipDays')
  if (skipDays) {
    feed.skipDays = findChildren(skipDays, 'day').map(textOf)
  }

  // Items can sit on the channel (RSS 0.92/2.0) or, very rarely, on the rss
  // root in some malformed feeds.
  const items = findChildren(channel, 'item')
  feed.items = items.map(parseRss2Item)

  return feed
}


// --- RSS 1.0 / 0.90 (native) parser ---------------------------------------

function parseRss1Image(el: XmlElement): Rss1Image {
  return {
    about: el.attributes?.['rdf:about'],
    title: textOf(findChild(el, 'title')),
    link: textOf(findChild(el, 'link')),
    url: textOf(findChild(el, 'url')),
  }
}

function parseRss1TextInput(el: XmlElement): Rss1TextInput {
  return {
    about: el.attributes?.['rdf:about'],
    title: textOf(findChild(el, 'title')),
    description: textOf(findChild(el, 'description')),
    name: textOf(findChild(el, 'name')),
    link: textOf(findChild(el, 'link')),
  }
}

function parseRss1Item(el: XmlElement): Rss1Item {
  const item: Rss1Item = {
    title: textOf(findChild(el, 'title')),
    link: textOf(findChild(el, 'link')),
  }
  const about = el.attributes?.['rdf:about']
  if (about) item.about = about
  const desc = findChild(el, 'description')
  if (desc) item.description = textOf(desc)
  return item
}

function parseRss1(root: XmlElement, version: FeedVersion): Rss1Feed {
  const channel = findChild(root, 'channel')
  const items = findChildren(root, 'item')

  const feed: Rss1Feed = {
    format: 'rdf',
    version: version === 'rss090' ? '0.90' : '1.0',
    title: textOf(findChild(channel, 'title')),
    link: textOf(findChild(channel, 'link')),
    items: items.map(parseRss1Item),
  }

  const about = channel?.attributes?.['rdf:about']
  if (about) feed.about = about

  const desc = findChild(channel, 'description')
  if (desc) feed.description = textOf(desc)

  const image = findChild(root, 'image')
  if (image) feed.image = parseRss1Image(image)

  const textInput = findChild(root, 'textinput') || findChild(root, 'textInput')
  if (textInput) feed.textInput = parseRss1TextInput(textInput)

  return feed
}


// --- Native -> Atom conversions -------------------------------------------

function rss2ToAtom(rss: Rss2Feed): AtomFeed {
  const out: AtomFeed = {
    format: 'atom',
    version: '1.0',
    entries: [],
  }

  if (rss.title) out.title = { type: 'text', value: rss.title }
  if (rss.description) out.subtitle = { type: 'text', value: rss.description }
  if (rss.copyright) out.rights = { type: 'text', value: rss.copyright }

  // Use the RSS link as the channel id when no other identity is available;
  // Atom requires an id but RSS doesn't have a direct equivalent, so this
  // is a best-effort mapping.
  if (rss.link) out.id = rss.link

  // Updated: prefer lastBuildDate, fall back to pubDate.
  if (rss.lastBuildDate) out.updated = rss.lastBuildDate
  else if (rss.pubDate) out.updated = rss.pubDate

  if (rss.generator) {
    out.generator = { value: rss.generator }
  }

  if (rss.image?.url) out.logo = rss.image.url

  const links: AtomLink[] = []
  if (rss.link) links.push({ href: rss.link, rel: 'alternate' })
  if (links.length) out.links = links

  if (rss.categories?.length) {
    out.categories = rss.categories.map((c) => ({
      term: c.value,
      scheme: c.domain,
    }))
  }

  // managingEditor / webMaster look like "email (Name)" or "email"; preserve
  // the email and name where possible.
  if (rss.managingEditor || rss.webMaster) {
    const persons: AtomPerson[] = []
    const me = rss.managingEditor || rss.webMaster
    if (me) persons.push(parseRssPerson(me))
    out.authors = persons
  }

  out.entries = rss.items.map(rss2ItemToAtomEntry)

  return out
}

function parseRssPerson(s: string): AtomPerson {
  // RSS authors look like "you@example.com (Your Name)" or just an email.
  const m = s.match(/^\s*([^\s()]+@\S+)\s*(?:\(([^)]+)\))?\s*$/)
  if (m) {
    return { name: m[2] || m[1], email: m[1] }
  }
  return { name: s.trim() }
}

function rss2ItemToAtomEntry(item: Rss2Item): AtomEntry {
  const entry: AtomEntry = {}

  if (item.title) entry.title = { type: 'text', value: item.title }
  if (item.description) entry.summary = { type: 'html', value: item.description }
  if (item.pubDate) {
    entry.published = item.pubDate
    entry.updated = item.pubDate
  }

  // guid is the natural id; otherwise fall back to link.
  if (item.guid) entry.id = item.guid.value
  else if (item.link) entry.id = item.link

  const links: AtomLink[] = []
  if (item.link) links.push({ href: item.link, rel: 'alternate' })
  if (item.enclosure?.url) {
    const link: AtomLink = { href: item.enclosure.url, rel: 'enclosure' }
    if (item.enclosure.type) link.type = item.enclosure.type
    if (item.enclosure.length !== undefined) link.length = item.enclosure.length
    links.push(link)
  }
  if (item.comments) {
    links.push({ href: item.comments, rel: 'replies', type: 'text/html' })
  }
  if (links.length) entry.links = links

  if (item.author) entry.authors = [parseRssPerson(item.author)]

  if (item.categories?.length) {
    entry.categories = item.categories.map((c) => ({
      term: c.value,
      scheme: c.domain,
    }))
  }

  if (item.source) {
    const src: Partial<AtomFeed> = { format: 'atom', version: '1.0' }
    if (item.source.value) src.title = { type: 'text', value: item.source.value }
    if (item.source.url) src.links = [{ href: item.source.url, rel: 'self' }]
    entry.source = src
  }

  return entry
}

function rss1ToAtom(rss: Rss1Feed): AtomFeed {
  const out: AtomFeed = {
    format: 'atom',
    version: '1.0',
    entries: [],
  }

  if (rss.title) out.title = { type: 'text', value: rss.title }
  if (rss.description) out.subtitle = { type: 'text', value: rss.description }
  if (rss.about) out.id = rss.about
  else if (rss.link) out.id = rss.link
  if (rss.link) out.links = [{ href: rss.link, rel: 'alternate' }]
  if (rss.image?.url) out.logo = rss.image.url

  out.entries = rss.items.map((it) => {
    const e: AtomEntry = {}
    if (it.title) e.title = { type: 'text', value: it.title }
    if (it.description) e.summary = { type: 'text', value: it.description }
    e.id = it.about || it.link
    if (it.link) e.links = [{ href: it.link, rel: 'alternate' }]
    return e
  })

  return out
}


// --- Top-level convert ----------------------------------------------------

function convert(root: XmlElement, opts: Required<FeedOptions>): FeedResult {
  if (opts.format === 'raw') return root

  const { dialect, version } = detect(root)

  if (dialect === 'unknown') {
    throw new Error(
      `feed: unrecognized root element "${root.localName}"; expected one of feed, rss, RDF`,
    )
  }

  // Native parse first.
  let native: AtomFeed | Rss2Feed | Rss1Feed
  if (dialect === 'atom') {
    native = parseAtom(root, version === 'atom03' ? 'atom03' : 'atom10')
  } else if (dialect === 'rss') {
    native = parseRss2(root, version)
  } else {
    native = parseRss1(root, version)
  }

  if (opts.format === 'native') return native

  // Atom (default): convert to Atom shape.
  if (native.format === 'atom') return native
  if (native.format === 'rss') return rss2ToAtom(native)
  return rss1ToAtom(native)
}

function withDefaults(o?: FeedOptions): Required<FeedOptions> {
  return { format: o?.format ?? 'atom' }
}


// --- jsonic Plugin --------------------------------------------------------

const Feed: Plugin = function Feed(jsonic, options) {
  jsonic.use(Xml)
  const opts = withDefaults(options as FeedOptions | undefined)

  // The xml rule's bc fires once per close, including extra times when
  // `r: xml` recurses to consume trailing whitespace. Mirror @xml-bc's
  // own guard: only run when an element was actually parsed in *this*
  // iteration (r.child.node is set), so the conversion happens exactly
  // once on the same iteration that @xml-bc copied the element to root.
  jsonic.rule('xml', (rs) => {
    rs.bc(true, (rule: any, ctx: any) => {
      const child = rule.child
      if (!child || !child.node) return
      const root = ctx.root()
      if (isElement(root.node)) {
        root.node = convert(root.node, opts)
      }
    })
  })
}


export { Feed, detect }

export type {
  FeedFormat,
  FeedDialect,
  FeedVersion,
  FeedOptions,
  FeedResult,
  AtomFeed,
  AtomEntry,
  AtomText,
  AtomPerson,
  AtomLink,
  AtomCategory,
  AtomGenerator,
  AtomContent,
  Rss2Feed,
  Rss2Item,
  Rss2Category,
  Rss2Enclosure,
  Rss2Guid,
  Rss2Source,
  Rss2Image,
  Rss2Cloud,
  Rss2TextInput,
  Rss1Feed,
  Rss1Item,
  Rss1Image,
  Rss1TextInput,
}
