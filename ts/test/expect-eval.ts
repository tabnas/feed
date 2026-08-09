/* Copyright (c) 2021-2026 Richard Rodger and other contributors, MIT License */

// Evaluates the upstream `Expect:` annotation carried by every
// kurtmckee/feedparser test document, against a @tabnas/feed `atom`-format
// result.
//
// This is what turns "it parsed" into a real VALUE check: the expected value
// comes from the third-party corpus, not from us.
//
// feedparser's annotations are Python expressions over feedparser's OWN
// normalized result shape. Two consequences, both deliberate and visible:
//
//  * SUPPORTED FORM. Only `not bozo and <path> == '<string>'` (any number of
//    `and`-joined clauses) is evaluated. Tuple comparisons (`*_parsed` time
//    tuples), `len()`, `has_key()`, dict literals and bare truthiness are
//    not. Those files are counted as `unsupported` and reported.
//
//  * PATH MAPPING. feedparser's key names are mapped onto the Atom shape
//    @tabnas/feed produces (`feed['tagline']` -> `subtitle.value`, and so
//    on). Paths with no mapping are counted as `unmapped` and the distinct
//    paths are printed, so the gap is visible rather than silent. The known
//    unmapped families are:
//      - `*_detail['base']` / `['language']` — xml:base and xml:lang
//        propagation, which @tabnas/feed does not model.
//      - `feed['language']`, `feed['publisher*']`, `feed['docs']`, ... —
//        RSS channel metadata that the Atom normalization drops.
//    These are a known gap, recorded here in writing, not a pass.

export type Clause = { path: string; steps: (string | number)[]; want: string }

// Parse the annotation. Returns null when the file has no `Expect:` or the
// expression is outside the supported form.
export function parseExpect(src: string): Clause[] | null {
  const m = src.match(/Expect:[ \t]*(.*)/)
  if (!m) return null
  let e = m[1].trim()
  const PRE = 'not bozo and '
  if (!e.startsWith(PRE)) return null
  e = e.slice(PRE.length).trim()

  const clauses: Clause[] = []
  for (let part of e.split(/\s+and\s+/)) {
    part = part.trim().replace(/^\((.*)\)$/, '$1').trim()
    const cm = part.match(
      /^((?:feed|entries\[\d+\])(?:\[(?:'[^']*'|\d+)\])*)\s*==\s*'((?:[^'\\]|\\.)*)'$/)
    if (!cm) return null
    const steps: (string | number)[] = []
    const head = cm[1].match(/^(?:feed|entries\[(\d+)\])/) as RegExpMatchArray
    if (undefined === head[1]) steps.push('feed')
    else steps.push('entries', Number(head[1]))
    for (const s of cm[1].slice(head[0].length).matchAll(/\[(?:'([^']*)'|(\d+))\]/g)) {
      steps.push(undefined === s[1] ? Number(s[2]) : s[1])
    }
    clauses.push({
      path: cm[1],
      steps,
      want: cm[2].replace(/\\'/g, "'").replace(/\\"/g, '"').replace(/\\\\/g, '\\'),
    })
  }
  return 0 < clauses.length ? clauses : null
}


// feedparser text-construct keys -> the Atom-shaped property holding them.
// `description`/`info`/`tagline` are the Atom 0.3 / RSS spellings.
const FEED_TEXT: Record<string, string> = {
  title: 'title',
  subtitle: 'subtitle', tagline: 'subtitle', description: 'subtitle', info: 'subtitle',
  rights: 'rights', copyright: 'rights',
}
const ENTRY_TEXT: Record<string, string> = {
  title: 'title',
  summary: 'summary', description: 'summary',
  rights: 'rights', copyright: 'rights',
}
const PERSON: Record<string, string> = {
  name: 'name', email: 'email', href: 'uri', url: 'uri', uri: 'uri',
}
// feedparser reports text-construct types as MIME types; the Atom shape
// keeps RFC 4287's `text` / `html` / `xhtml` tokens.
const MIME: Record<string, string> = {
  text: 'text/plain', html: 'text/html', xhtml: 'application/xhtml+xml',
}

export type Resolved =
  | { ok: true; value: unknown }
  | { ok: false; why: string }

function firstAlternate(links: any): unknown {
  if (!Array.isArray(links)) return undefined
  const a = links.find((l: any) => !l.rel || 'alternate' === l.rel)
  return a && a.href
}

function textAt(obj: any, prop: string, tail: (string | number)[]): Resolved {
  const t = obj && obj[prop]
  if (0 === tail.length) return { ok: true, value: t && t.value }
  if (1 === tail.length && 'value' === tail[0]) return { ok: true, value: t && t.value }
  if (1 === tail.length && 'type' === tail[0]) {
    return { ok: true, value: t && MIME[t.type] }
  }
  return { ok: false, why: 'detail.' + tail.join('.') }
}

// Resolve one feedparser path against an atom-format @tabnas/feed result.
export function resolve(feed: any, steps: (string | number)[]): Resolved {
  let obj: any
  let path: (string | number)[]
  const isEntry = 'entries' === steps[0]
  if (isEntry) {
    const es = feed && feed.entries
    if (!Array.isArray(es) || undefined === es[steps[1] as number]) {
      return { ok: false, why: 'no such entry' }
    }
    obj = es[steps[1] as number]
    path = steps.slice(2)
  } else {
    obj = feed
    path = steps.slice(1)
  }
  if (0 === path.length) return { ok: false, why: 'whole-object comparison' }

  const k = path[0]
  const tail = path.slice(1)
  const n = (x: unknown) => 'number' === typeof x

  if ('string' === typeof k) {
    const TEXT = isEntry ? ENTRY_TEXT : FEED_TEXT
    const detail = k.endsWith('_detail') ? k.slice(0, -'_detail'.length) : null
    if (TEXT[k]) return textAt(obj, TEXT[k], tail)
    if (detail && TEXT[detail]) return textAt(obj, TEXT[detail], tail)

    if (('id' === k || 'guid' === k) && 0 === tail.length) return { ok: true, value: obj.id }
    if ('updated' === k && 0 === tail.length) return { ok: true, value: obj.updated }
    if ('published' === k && 0 === tail.length) return { ok: true, value: obj.published }
    if ('link' === k && 0 === tail.length) return { ok: true, value: firstAlternate(obj.links) }

    if ('links' === k && 2 === tail.length && n(tail[0]) &&
      ['href', 'rel', 'type', 'title'].includes(tail[1] as string)) {
      const l = (obj.links || [])[tail[0] as number]
      return { ok: true, value: l && l[tail[1] as string] }
    }
    if ('author_detail' === k && 1 === tail.length && PERSON[tail[0] as string]) {
      const a = (obj.authors || [])[0]
      return { ok: true, value: a && a[PERSON[tail[0] as string]] }
    }
    if (('authors' === k || 'contributors' === k) && 2 === tail.length && n(tail[0]) &&
      PERSON[tail[1] as string]) {
      const a = (obj[k] || [])[tail[0] as number]
      return { ok: true, value: a && a[PERSON[tail[1] as string]] }
    }
    if ('tags' === k && 2 === tail.length && n(tail[0]) &&
      ['term', 'scheme', 'label'].includes(tail[1] as string)) {
      const c = (obj.categories || [])[tail[0] as number]
      return { ok: true, value: c && c[tail[1] as string] }
    }
    if ('generator' === k && 0 === tail.length) {
      return { ok: true, value: obj.generator && obj.generator.value }
    }
    if ('content' === k && 2 === tail.length && n(tail[0]) &&
      ['value', 'type'].includes(tail[1] as string)) {
      if (0 !== tail[0]) return { ok: false, why: 'content[n>0]' }
      const c = obj.content
      if ('type' === tail[1]) return { ok: true, value: c && (MIME[c.type] || c.type) }
      return { ok: true, value: c && c.value }
    }
    if ('image' === k && 1 === tail.length && 'href' === tail[0]) {
      return { ok: true, value: obj.logo }
    }
    if ('source' === k && 0 < tail.length) {
      const s = obj.source
      if (!s) return { ok: true, value: undefined }
      return resolve({ entries: [], ...s }, ['feed', ...tail])
    }
  }
  return { ok: false, why: (isEntry ? 'entry.' : 'feed.') + path.join('.') }
}
