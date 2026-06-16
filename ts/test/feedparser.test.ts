/* Copyright (c) 2021-2025 Richard Rodger and other contributors, MIT License */

// Uses well-formed feed samples vendored from kurtmckee/feedparser
// (Copyright (C) 2010-2025 Kurt McKee, 2002-2008 Mark Pilgrim, BSD 2-Clause)
// at test/feedparser-wellformed/. The upstream LICENSE is preserved alongside;
// see THIRD_PARTY_NOTICES.md.

import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readdirSync, readFileSync, existsSync } from 'node:fs'
import { join } from 'node:path'

import { Tabnas } from '@tabnas/parser'
import { jsonic } from '@tabnas/jsonic'
import { Feed, detect } from '../dist/feed.js'
import type {
  AtomFeed,
  FeedDialect,
  FeedVersion,
} from '../dist/feed.js'

const SUITE_DIR = join(__dirname, '..', '..', 'test', 'feedparser-wellformed')

// Read all .xml files in a subdirectory.
function loadDir(name: string): { file: string; src: string }[] {
  const dir = join(SUITE_DIR, name)
  if (!existsSync(dir)) return []
  return readdirSync(dir)
    .filter((f) => f.endsWith('.xml'))
    .sort()
    .map((f) => ({ file: f, src: readFileSync(join(dir, f), 'utf8') }))
}

const ATOM10 = loadDir('atom10')
const ATOM = loadDir('atom')
const RSS = loadDir('rss')
const RDF = loadDir('rdf')

// Reusable parsers — atom output (default) and raw output for detection.
const atomParse = new Tabnas().use(jsonic).use(Feed)
const rawParse = new Tabnas().use(jsonic).use(Feed, { format: 'raw' })


describe('feedparser-wellformed - dialect / version detection', () => {
  if (!existsSync(SUITE_DIR)) {
    test('suite unavailable', () => {
      console.warn(`feedparser-wellformed not vendored at ${SUITE_DIR}; skipping.`)
    })
    return
  }

  function check(
    files: { file: string; src: string }[],
    expect: { dialect: FeedDialect; version: FeedVersion | FeedVersion[] },
    label: string,
  ) {
    test(label, () => {
      const fails: string[] = []
      for (const { file, src } of files) {
        const root: any = rawParse.parse(src)
        const got = detect(root)
        const versionsOk = Array.isArray(expect.version)
          ? expect.version.includes(got.version)
          : got.version === expect.version
        if (got.dialect !== expect.dialect || !versionsOk) {
          fails.push(`  ${file}: got ${got.dialect}/${got.version}`)
        }
      }
      assert.deepEqual(fails, [], `${label} mismatches:\n${fails.join('\n')}`)
    })
  }

  check(ATOM10, { dialect: 'atom', version: 'atom10' }, 'atom10/* -> atom/atom10')
  check(ATOM, { dialect: 'atom', version: 'atom03' }, 'atom/* -> atom/atom03')
  check(
    RSS,
    { dialect: 'rss', version: ['rss20', 'rss092', 'rss091u', 'rss091n'] },
    'rss/* -> rss/(any 0.91/0.92/2.0)',
  )
  check(
    RDF,
    { dialect: 'rdf', version: ['rss10', 'rss090'] },
    'rdf/* -> rdf/(rss10 or rss090)',
  )
})


describe('feedparser-wellformed - parse without error', () => {
  if (!existsSync(SUITE_DIR)) return

  function checkAll(set: { file: string; src: string }[], label: string) {
    test(`all ${label} files parse to a feed`, () => {
      const fails: { file: string; err: string }[] = []
      for (const { file, src } of set) {
        try {
          const f = atomParse.parse(src) as AtomFeed
          if (f.format !== 'atom') {
            fails.push({ file, err: `format=${f.format}` })
          }
        } catch (e: any) {
          fails.push({ file, err: e?.message || String(e) })
        }
      }
      assert.deepEqual(
        fails,
        [],
        `${label} failures:\n${fails.map((x) => `  ${x.file}: ${x.err}`).join('\n')}`,
      )
    })
  }

  checkAll(ATOM10, 'atom10')
  checkAll(ATOM, 'atom')
  checkAll(RSS, 'rss')
  checkAll(RDF, 'rdf')
})


// Targeted value checks against vendored corpus files. We match feedparser's
// own Description/Expect comments (translated to our shape).
describe('feedparser-wellformed - targeted value checks', () => {
  if (!existsSync(SUITE_DIR)) return

  function findFile(set: { file: string; src: string }[], name: string) {
    return set.find((x) => x.file === name)?.src
  }

  test('atom10/entry_title -> entries[0].title.value == "Example Atom"', () => {
    const src = findFile(ATOM10, 'entry_title.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.equal(f.entries[0].title?.value, 'Example Atom')
  })

  test('atom10/entry_author_email -> entries[0].authors[0].email', () => {
    const src = findFile(ATOM10, 'entry_author_email.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.equal(f.entries[0].authors?.[0].email, 'me@example.com')
  })

  test('atom10/entry_author_name -> entries[0].authors[0].name', () => {
    const src = findFile(ATOM10, 'entry_author_name.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.equal(f.entries[0].authors?.[0].name, 'Example author')
  })

  test('atom10/entry_id -> entries[0].id', () => {
    const src = findFile(ATOM10, 'entry_id.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.ok(f.entries[0].id, 'id present')
  })

  test('atom10/entry_link_href -> entries[0].links[0].href', () => {
    const src = findFile(ATOM10, 'entry_link_href.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.ok(f.entries[0].links?.[0].href, 'href present')
  })

  test('atom/entry_title -> entries[0].title.value', () => {
    const src = findFile(ATOM, 'entry_title.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.ok(f.entries[0].title?.value, 'title present')
  })

  test('atom/entry_issued -> entries[0].published', () => {
    const src = findFile(ATOM, 'entry_issued.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.ok(f.entries[0].published, 'published present')
  })

  test('atom/entry_modified -> entries[0].updated', () => {
    const src = findFile(ATOM, 'entry_modified.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.ok(f.entries[0].updated, 'updated present')
  })

  test('rss/channel_title -> title.value == "Example feed"', () => {
    const src = findFile(RSS, 'channel_title.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.equal(f.title?.value, 'Example feed')
  })

  test('rss/item_title -> entries[0].title.value', () => {
    const src = findFile(RSS, 'item_title.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.equal(f.entries[0].title?.value, 'Item 1 title')
  })

  test('rss/item_link -> entries[0].links[0].href', () => {
    const src = findFile(RSS, 'item_link.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.ok(f.entries[0].links?.[0].href, 'href present')
  })

  test('rss/item_guid -> entries[0].id', () => {
    const src = findFile(RSS, 'item_guid.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.ok(f.entries[0].id, 'id present')
  })

  test('rss/item_enclosure_url -> entries[0].links has rel=enclosure', () => {
    const src = findFile(RSS, 'item_enclosure_url.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    const enc = f.entries[0].links?.find((l) => l.rel === 'enclosure')
    assert.ok(enc, 'enclosure link present')
  })

  test('rdf/rdf_channel_title -> title.value == "Example feed"', () => {
    const src = findFile(RDF, 'rdf_channel_title.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.equal(f.title?.value, 'Example feed')
  })

  test('rdf/rdf_item_title -> entries[0].title.value == "Example title"', () => {
    const src = findFile(RDF, 'rdf_item_title.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.equal(f.entries[0].title?.value, 'Example title')
  })

  test('rdf/rss090_channel_title -> title.value == "Example title"', () => {
    const src = findFile(RDF, 'rss090_channel_title.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.equal(f.title?.value, 'Example title')
  })

  test('rdf/rdf_item_rdf_about -> entries[0].id is the rdf:about URI', () => {
    const src = findFile(RDF, 'rdf_item_rdf_about.xml')
    assert.ok(src, 'fixture present')
    const f = atomParse.parse(src!) as AtomFeed
    assert.equal(f.entries[0].id, 'http://example.org/1')
  })
})
