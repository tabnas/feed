/* Copyright (c) 2021-2026 Richard Rodger and other contributors, MIT License */

// Conformance against the kurtmckee/feedparser corpus.
//
//   upstream: https://github.com/kurtmckee/feedparser  (BSD 2-Clause)
//   pinned:   a22c5521cbb109871f1a2318948581901bd47e26
//   fetch:    ./scripts/fetch-feedparser.sh   (NOT committed to this repo)
//
// `scripts/fetch-corpus.mjs` already fetched this corpus for the npm
// `pretest` hook, but until now nothing read it: `test/feedparser/` was
// downloaded on every run and then ignored. This file is the consumer, and
// `go/conformance_test.go` is its twin, so the two runtimes cannot drift.
//
// Four checks, all of which must run:
//
//   1. wellformed/  -> must PARSE.
//   2. wellformed/  -> must produce the VALUE the upstream `Expect:`
//                      annotation states. "It did not throw" is not a
//                      conformance result; see ./expect-eval.ts for the
//                      evaluator and its documented limits.
//   3. wellformed/  -> the detected dialect must match the document element,
//                      and the detected version must match the upstream
//                      `version == '...'` annotation.
//   4. illformed/   -> must be REJECTED, except for an explicitly enumerated
//                      set of documents whose defect is outside what a
//                      string-input XML parser can see (see below).
//
// Nothing here skips. If the corpus is absent `requireCorpus` throws with
// instructions, because a conformance test that quietly does not run is
// worse than no test at all.

import { test, describe } from 'node:test'
import assert from 'node:assert'
import { join } from 'node:path'

import { Tabnas } from '@tabnas/parser'
import { jsonic } from '@tabnas/jsonic'
import { Feed, detect } from '../dist/feed.js'
import type { AtomFeed } from '../dist/feed.js'

import {
  requireCorpus, walkXml, read, rel, isFeedRoot, rootLocalName, fmtFails,
} from './corpus.js'
import { parseExpect, resolve } from './expect-eval.js'

const SUITE = requireCorpus('feedparser', 'fetch-feedparser.sh')
const WELLFORMED = join(SUITE, 'wellformed')
const ILLFORMED = join(SUITE, 'illformed')

const wfAll = walkXml(WELLFORMED)
const illAll = walkXml(ILLFORMED)

// Floors on the corpus itself. A truncated or half-fetched corpus would
// otherwise shrink every denominator below and turn the whole file green
// while measuring almost nothing.
assert.ok(1000 < wfAll.length,
  `feedparser wellformed corpus looks truncated: ${wfAll.length} files. ` +
  `Re-run ./scripts/fetch-feedparser.sh`)
assert.ok(0 < illAll.length,
  `feedparser illformed corpus missing. Re-run ./scripts/fetch-feedparser.sh`)

// Documents whose root is not feed/rss/RDF are outside the README's claim
// (RSS 0.90-2.0 + Atom 0.3/1.0) and are counted and printed, not asserted.
const wf = wfAll.filter((p) => isFeedRoot(read(p)))
const wfOutOfClaim = wfAll.length - wf.length

const atomParse = new Tabnas().use(jsonic).use(Feed)
const rawParse = new Tabnas().use(jsonic).use(Feed, { format: 'raw' })


describe('feedparser: well-formed documents must PARSE', () => {
  test(`${wf.length} documents`, () => {
    const fails: string[] = []
    for (const p of wf) {
      try {
        const f = atomParse.parse(read(p)) as AtomFeed
        if (!f || 'atom' !== f.format) fails.push(`${rel(p)}: not an atom-shaped result`)
      } catch (e: any) {
        fails.push(`${rel(p)}: ${String(e?.message).split('\n')[0]}`)
      }
    }
    console.log(
      `feedparser wellformed: ${wf.length - fails.length}/${wf.length} parsed` +
      ` (+${wfOutOfClaim} non-RSS/Atom roots, outside the claim, not asserted)`)
    assert.deepEqual(fails, [],
      `parse failures (${fails.length}/${wf.length}):\n${fmtFails(fails)}`)
  })
})


// The VALUE check — a ratchet, not a pass/fail line.
//
// Measured on `main` at 7d2103f (2026-08-09): 375 of 1360 machine-checkable
// upstream assertions hold. 1360 is the number of documents whose `Expect:`
// this harness can both parse AND map onto the Atom shape; the rest are
// counted and printed, never silently absorbed.
//
// So this suite does NOT assert "all values are correct" — that would be red
// and unmergeable, and pretending otherwise is how a suite ends up disabled.
// It asserts the floor holds and can only be RAISED. Raise both numbers when
// you fix the parser; never lower either to get green. Lowering the
// denominator is caught too: dropping value checks to improve the ratio is
// exactly the failure mode a bare percentage invites.
const VALUE_CORRECT_FLOOR = 375
const VALUE_CHECKED_FLOOR = 1360

describe('feedparser: well-formed documents must produce the EXPECTED VALUE', () => {
  test('upstream `Expect:` annotations hold at or above the recorded floor', () => {
    const fails: string[] = []
    const unmapped = new Map<string, number>()
    let supported = 0, correct = 0, unmappedFiles = 0
    for (const p of wf) {
      const src = read(p)
      const clauses = parseExpect(src)
      if (null === clauses) continue
      supported++
      let f: any
      try {
        f = atomParse.parse(src)
      } catch (e: any) {
        fails.push(`${rel(p)}: parse threw: ${String(e?.message).split('\n')[0]}`)
        continue
      }
      let sawUnmapped = false
      let bad: string | null = null
      for (const c of clauses) {
        const r = resolve(f, c.steps)
        if (!r.ok) {
          sawUnmapped = true
          unmapped.set(r.why, (unmapped.get(r.why) ?? 0) + 1)
          continue
        }
        if (r.value !== c.want) {
          bad = `${c.path} = ${JSON.stringify(r.value)}, expected ${JSON.stringify(c.want)}`
          break
        }
      }
      if (null !== bad) fails.push(`${rel(p)}: ${bad}`)
      else if (sawUnmapped) unmappedFiles++
      else correct++
    }
    const checked = correct + fails.length
    const top = [...unmapped.entries()].sort((a, b) => b[1] - a[1]).slice(0, 15)
    console.log(
      `feedparser values: ${correct}/${checked} correct` +
      ` (${supported} of ${wf.length} files have a machine-checkable Expect;` +
      ` ${unmappedFiles} use paths this harness does not map — known gap)\n` +
      `  top unmapped paths: ${top.map(([k, v]) => `${k}(${v})`).join(' ')}`)

    assert.ok(VALUE_CHECKED_FLOOR <= checked,
      `only ${checked} value assertions were evaluated, was ${VALUE_CHECKED_FLOOR}. ` +
      `Value checks were LOST — fix the mapping, do not lower the floor.`)
    assert.ok(VALUE_CORRECT_FLOOR <= correct,
      `${correct}/${checked} upstream value assertions hold, was ` +
      `${VALUE_CORRECT_FLOOR}. This is a REGRESSION.\n` +
      `Sample of the ${fails.length} current failures:\n${fmtFails(fails)}`)
    if (VALUE_CORRECT_FLOOR < correct) {
      console.log(
        `NOTE: value conformance improved to ${correct}/${checked};` +
        ` raise VALUE_CORRECT_FLOOR (and the Go twin) to ${correct}.`)
    }
  })
})


describe('feedparser: ill-formed documents must be REJECTED', () => {
  // Upstream's `illformed/` directory is keyed off feedparser's `bozo` flag,
  // which is much broader than XML well-formedness. These documents are
  // ACCEPTED by @tabnas/feed, and each is listed with the reason, so the set
  // is a statement rather than an excuse. The set is asserted to be EXACTLY
  // this list: a newly-accepted ill-formed document turns the suite red, and
  // so does a listed document that starts being rejected (which means the
  // entry must be deleted). Do not add an entry to get green.
  const ACCEPTED: Record<string, string> = {
    // Well-formed XML carrying an invalid DOCTYPE. Upstream itself annotates
    // this `Expect: not bozo and feed['title'] == 'found'` — accepting it is
    // the CORRECT behaviour; it sits in illformed/ for a different reason.
    'always_strip_doctype.xml':
      'well-formed; upstream Expect is `not bozo`, so accepting is correct',

    // Declared-vs-actual character encoding mismatches. Detecting these
    // requires the raw byte stream and a charset detector; @tabnas/feed is
    // handed an already-decoded JavaScript/Go string, so the evidence is
    // gone before the parser sees it.
    'chardet/big5.xml': 'encoding detection: needs raw bytes, not a decoded string',
    'chardet/eucjp.xml': 'encoding detection: needs raw bytes, not a decoded string',
    'chardet/euckr.xml': 'encoding detection: needs raw bytes, not a decoded string',
    'chardet/gb2312.xml': 'encoding detection: needs raw bytes, not a decoded string',
    'chardet/koi8r.xml': 'encoding detection: needs raw bytes, not a decoded string',
    'chardet/shiftjis.xml': 'encoding detection: needs raw bytes, not a decoded string',
    'chardet/tis620.xml': 'encoding detection: needs raw bytes, not a decoded string',
    'chardet/windows1255.xml': 'encoding detection: needs raw bytes, not a decoded string',

    // GeoRSS / GML coordinate errors. Well-formed XML; the defect is in the
    // meaning of an extension element @tabnas/feed does not model.
    'geo/georss_point_no_coords.xml': 'GeoRSS semantics, not XML well-formedness',
    'geo/georss_polygon_insufficient_coords.xml': 'GeoRSS semantics, not XML well-formedness',
    'geo/gml_point.xml': 'GML semantics, not XML well-formedness',

    // Well-formed iso-8859-7 document; upstream records that the non-ASCII
    // date crashed its own date parser. @tabnas/feed does not parse dates.
    'http_high_bit_date.xml': 'upstream records a date-parser crash, not a well-formedness defect',
  }
  const ILL_PREFIX = 'test/feedparser/illformed/'

  test(`${illAll.length} documents`, () => {
    const unexpectedlyAccepted: string[] = []
    const noLongerAccepted: string[] = []
    for (const p of illAll) {
      const key = rel(p).slice(ILL_PREFIX.length)
      let threw = false
      try { atomParse.parse(read(p)) } catch { threw = true }
      if (!threw && undefined === ACCEPTED[key]) {
        unexpectedlyAccepted.push(`ACCEPTED but upstream marks it ill-formed: ${key}`)
      }
      if (threw && undefined !== ACCEPTED[key]) {
        noLongerAccepted.push(
          `now REJECTED (good) — delete its ACCEPTED entry: ${key}`)
      }
    }
    const rejected = illAll.length - Object.keys(ACCEPTED).length
    console.log(
      `feedparser illformed: ${rejected}/${illAll.length} rejected` +
      ` (${Object.keys(ACCEPTED).length} enumerated as outside a string-input` +
      ` XML parser's reach)`)
    assert.deepEqual(unexpectedlyAccepted, [],
      `must-reject failures:\n${fmtFails(unexpectedlyAccepted)}`)
    assert.deepEqual(noLongerAccepted, [],
      `the ACCEPTED list is now stale:\n${fmtFails(noLongerAccepted)}`)
  })
})


// Dialect / version detection. Both oracles come from the corpus, never
// from us: the document element read out of the source text, and the
// upstream `Expect: ... version == 'X'` annotation.
describe('feedparser: dialect / version detection', () => {
  test('dialect matches the document element', () => {
    const want: Record<string, string> = { feed: 'atom', rss: 'rss', RDF: 'rdf' }
    const fails: string[] = []
    for (const p of wf) {
      const src = read(p)
      try {
        const got = detect(rawParse.parse(src) as any)
        const w = want[rootLocalName(src)]
        if (got.dialect !== w) {
          fails.push(`${rel(p)}: ${got.dialect} (root <${rootLocalName(src)}>)`)
        }
      } catch (e: any) {
        fails.push(`${rel(p)}: ${String(e?.message).split('\n')[0]}`)
      }
    }
    console.log(`feedparser dialect: ${wf.length - fails.length}/${wf.length} correct`)
    assert.deepEqual(fails, [],
      `dialect failures (${fails.length}/${wf.length}):\n${fmtFails(fails)}`)
  })

  // Five documents where `detect()` disagrees with upstream. Each is listed
  // with what we currently report, so the set is exact in both directions:
  // a sixth disagreement is red, and fixing one of these is red until its
  // entry is deleted.
  const VERSION_KNOWN_WRONG: Record<string, string> = {
    // RSS 0.90 is an RDF document; detect() reports the RDF-era RSS 1.0.
    'rss/rss_version_090.xml': 'rss10',
    // Netscape vs Userland 0.91 are told apart by the DOCTYPE, which
    // detect() does not read.
    'rss/rss_version_091_netscape.xml': 'rss091u',
    // 0.93 / 0.94 are not modelled; both collapse onto 0.92.
    'rss/rss_version_093.xml': 'rss092',
    'rss/rss_version_094.xml': 'rss092',
    // <rss> with no version attribute: upstream reports the bare 'rss'.
    'rss/rss_version_missing.xml': 'rss20',
  }
  const WF_PREFIX = 'test/feedparser/wellformed/'

  test("upstream `version == '...'` annotations hold", () => {
    const fails: string[] = []
    const staleEntries: string[] = []
    let checked = 0
    for (const p of wf) {
      const src = read(p)
      const m = src.match(/version == '([a-z0-9]+)'/)
      if (!m) continue
      checked++
      const key = rel(p).slice(WF_PREFIX.length)
      let got: string
      try {
        got = detect(rawParse.parse(src) as any).version as string
      } catch (e: any) {
        got = `THREW: ${String(e?.message).split('\n')[0]}`
      }
      const known = VERSION_KNOWN_WRONG[key]
      if (got === m[1]) {
        if (undefined !== known) {
          staleEntries.push(`now correct — delete its VERSION_KNOWN_WRONG entry: ${key}`)
        }
      } else if (known !== got) {
        fails.push(`${key}: ${got}, upstream says ${m[1]}` +
          (undefined === known ? '' : ` (recorded as ${known})`))
      }
    }
    assert.ok(0 < checked, 'no version annotations found — corpus wrong?')
    const wrong = Object.keys(VERSION_KNOWN_WRONG).length
    console.log(`feedparser version: ${checked - wrong}/${checked} correct` +
      ` (${wrong} enumerated disagreements)`)
    assert.deepEqual(fails, [],
      `version failures (${fails.length}/${checked}):\n${fmtFails(fails)}`)
    assert.deepEqual(staleEntries, [],
      `VERSION_KNOWN_WRONG is stale:\n${fmtFails(staleEntries)}`)
  })
})
