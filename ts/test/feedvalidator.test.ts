/* Copyright (c) 2021-2026 Richard Rodger and other contributors, MIT License */

// Conformance against the rubys/feedvalidator corpus — the test suite behind
// the W3C Feed Validation Service, and the most widely cited third-party
// corpus for RSS/Atom.
//
//   upstream: https://github.com/rubys/feedvalidator
//   pinned:   2a8050b950594464b3923af249623b614774c138
//   fetch:    ./scripts/fetch-feedvalidator.sh   (NOT committed to this repo)
//
// Every testcase carries its own upstream `Expect:` annotation. This suite
// uses the WHOLE tree — no hand-picked subset — and classifies each file by
// that annotation plus its document element, both read from the source text
// so the classification cannot be biased by the parser under test:
//
//   1. `Expect: SAXError`  -> upstream declares the document NOT well-formed
//                             XML, so the parser MUST reject it.
//   2. otherwise, feed root (feed / rss / RDF)
//                          -> well-formed RSS or Atom, so the parser MUST
//                             accept it and MUST report the right dialect.
//   3. otherwise            -> KML / OpenSearch / OPML / RSS 1.1 / APP
//                             documents. @tabnas/feed's README claims RSS
//                             0.90-2.0 and Atom 0.3/1.0 only, so these are
//                             outside the claim: counted and printed, and
//                             deliberately NOT asserted either way. The
//                             count is printed on every run so the bucket
//                             cannot grow unnoticed.

import { test, describe } from 'node:test'
import assert from 'node:assert'
import { join } from 'node:path'

import { Tabnas } from '@tabnas/parser'
import { jsonic } from '@tabnas/jsonic'
import { Feed, detect } from '../dist/feed.js'

import {
  requireCorpus, walkXml, read, rel, rootLocalName, isFeedRoot, fmtFails,
} from './corpus.js'

// Fails loudly (throws at import time) when the corpus has not been fetched.
const SUITE = requireCorpus('feedvalidator', 'fetch-feedvalidator.sh')
const FILES = walkXml(join(SUITE, 'testcases'))

assert.ok(
  1000 < FILES.length,
  `feedvalidator corpus looks truncated: ${FILES.length} .xml files under ${SUITE}. ` +
  `Re-run ./scripts/fetch-feedvalidator.sh`,
)

const SAX_RE = /Expect:\s*[^\n]*SAXError/

// Four corpus documents are objectively NOT well-formed but carry an
// `Expect:` naming a validator-level diagnostic instead of `SAXError` —
// upstream annotates the thing the case is *about*, and a real SAX parse of
// any of these fails first. They are moved into the must-REJECT bucket, not
// excused: the violation is quoted from the file, and the set is asserted to
// be exactly this list, so a fifth one cannot be added silently and a fixed
// one cannot stay listed. Verified by reading the source, not by asking our
// own parser.
const NOT_WELL_FORMED: Record<string, string> = {
  // <copyright type="application/xhtml+xml"> ... </rights>
  'atom/must/feed_copyright_is_inline.xml':
    'XML 1.0 3 WFC "Element Type Match": <copyright> is closed by </rights>',
  // <copyright type="text/html" mode="xml"> ... </rights>
  'atom/must/feed_copyright_is_inline_2.xml':
    'XML 1.0 3 WFC "Element Type Match": <copyright> is closed by </rights>',
  // <sx:sharing .../> is self-closing, then a stray </sx:sharing> follows,
  // so the end tag lands on the still-open <feed>.
  'ext/feedsync/sharing_until_rfc822.xml':
    'XML 1.0 3 WFC "Element Type Match": stray </sx:sharing> after a self-closed <sx:sharing/>',
  // <invalid:tag xmlns:bogus="tag:foo.bar"/> — declares `bogus`, uses
  // `invalid`. The Expect is about the bogus namespace URI, but the
  // undeclared prefix is a hard error first.
  'atom/6.1/invalid-namespace.xml':
    'Namespaces in XML 1.0 NSC "Prefix Declared": prefix `invalid` is never declared',
}

const saxFiles: string[] = []
const feedFiles: string[] = []
const outOfClaim: string[] = []
const seenNwf = new Set<string>()
for (const p of FILES) {
  const src = read(p)
  const key = rel(p).replace(/^test\/feedvalidator\/testcases\//, '')
  if (undefined !== NOT_WELL_FORMED[key]) {
    seenNwf.add(key)
    saxFiles.push(p)
  } else if (SAX_RE.test(src)) saxFiles.push(p)
  else if (isFeedRoot(src)) feedFiles.push(p)
  else outOfClaim.push(p)
}

// Guard the reclassification: every listed path must exist in the corpus, so
// a rename upstream turns into a red test rather than a silent exemption.
assert.deepEqual(
  [...seenNwf].sort(), Object.keys(NOT_WELL_FORMED).sort(),
  'NOT_WELL_FORMED lists a path that is not in the fetched corpus',
)

const atomParse = new Tabnas().use(jsonic).use(Feed)
const rawParse = new Tabnas().use(jsonic).use(Feed, { format: 'raw' })

// The dialect each testcase directory is about, from the corpus layout —
// upstream's own classification, not ours.
function expectedDialect(p: string): string | null {
  const r = rel(p)
  if (r.includes('/testcases/atom/')) return 'atom'
  if (r.includes('/testcases/rss20/')) return 'rss'
  return null
}


describe('feedvalidator: invalid documents must be REJECTED', () => {
  test(`${saxFiles.length} not-well-formed documents ` +
    `(${saxFiles.length - seenNwf.size} annotated "Expect: SAXError" ` +
    `+ ${seenNwf.size} reclassified)`, () => {
    assert.ok(0 < saxFiles.length, 'no SAXError cases found — corpus wrong?')
    const fails: string[] = []
    for (const p of saxFiles) {
      let threw = false
      try { atomParse.parse(read(p)) } catch { threw = true }
      if (!threw) fails.push(`ACCEPTED but not well-formed: ${rel(p)}`)
    }
    console.log(
      `feedvalidator invalid: ${saxFiles.length - fails.length}/${saxFiles.length} rejected`)
    assert.deepEqual(fails, [],
      `must-reject failures (${fails.length}/${saxFiles.length}):\n${fmtFails(fails)}`)
  })
})


describe('feedvalidator: valid documents must be ACCEPTED', () => {
  test(`${feedFiles.length} well-formed RSS/Atom documents parse`, () => {
    const fails: string[] = []
    for (const p of feedFiles) {
      try {
        const f: any = atomParse.parse(read(p))
        if (!f || 'atom' !== f.format) {
          fails.push(`${rel(p)}: no atom-shaped result (got ${JSON.stringify(f).slice(0, 60)})`)
        }
      } catch (e: any) {
        fails.push(`${rel(p)}: ${String(e?.message).split('\n')[0]}`)
      }
    }
    console.log(
      `feedvalidator valid: ${feedFiles.length - fails.length}/${feedFiles.length} accepted` +
      ` (+${outOfClaim.length} documents outside the RSS/Atom claim, not asserted:` +
      ` ${[...new Set(outOfClaim.map((p) => rootLocalName(read(p))))].sort().join(', ')})`)
    assert.deepEqual(fails, [],
      `must-accept failures (${fails.length}/${feedFiles.length}):\n${fmtFails(fails)}`)
  })

  // Accepting is only half a value check: the parse must also report the
  // dialect the corpus directory says the document is.
  test('detected dialect matches the corpus directory', () => {
    const fails: string[] = []
    let checked = 0
    for (const p of feedFiles) {
      const want = expectedDialect(p)
      if (null === want) continue
      checked++
      try {
        const got = detect(rawParse.parse(read(p)) as any)
        if (got.dialect !== want) {
          fails.push(`${rel(p)}: dialect=${got.dialect}/${got.version}, want ${want}`)
        }
      } catch (e: any) {
        fails.push(`${rel(p)}: ${String(e?.message).split('\n')[0]}`)
      }
    }
    console.log(`feedvalidator detect: ${checked - fails.length}/${checked} correct dialect`)
    assert.deepEqual(fails, [],
      `dialect-detection failures (${fails.length}/${checked}):\n${fmtFails(fails)}`)
  })
})
