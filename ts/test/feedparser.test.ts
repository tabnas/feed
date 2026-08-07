/* Copyright (c) 2021-2026 Richard Rodger and other contributors, MIT License */

// Conformance against the kurtmckee/feedparser corpus.
//
//   upstream: https://github.com/kurtmckee/feedparser  (BSD 2-Clause)
//   pinned:   a22c5521cbb109871f1a2318948581901bd47e26
//   fetch:    ./scripts/fetch-feedparser.sh   (NOT committed to this repo)
//
// Three halves, all of which must run:
//
//   1. tests/wellformed/  -> must PARSE.
//   2. tests/wellformed/  -> must produce the VALUE the upstream `Expect:`
//                            annotation states. Asserting only "it didn't
//                            throw" is worthless; see ./expect-eval.ts for
//                            the evaluator and its documented limits.
//   3. tests/illformed/   -> must be REJECTED.
//
// Nothing here skips. If the corpus is absent the suite throws with
// instructions, because a conformance test that quietly does not run is
// worse than no test at all.

import { test, describe } from 'node:test'
import assert from 'node:assert'
import { join } from 'node:path'

import { Tabnas } from '@tabnas/parser'
import { jsonic } from '@tabnas/jsonic'
import { Feed, detect } from '../dist/feed.js'
import type { AtomFeed, FeedDialect, FeedVersion } from '../dist/feed.js'

import { requireCorpus, walkXml, read, rel, isFeedRoot, rootLocalName, fmtFails } from './corpus.js'
import { parseExpect, resolve } from './expect-eval.js'

const SUITE = requireCorpus('feedparser', 'fetch-feedparser.sh')
const WELLFORMED = join(SUITE, 'wellformed')
const ILLFORMED = join(SUITE, 'illformed')

const wfAll = walkXml(WELLFORMED)
const illAll = walkXml(ILLFORMED)

assert.ok(1000 < wfAll.length,
  `feedparser wellformed corpus looks truncated: ${wfAll.length} files. ` +
  `Re-run ./scripts/fetch-feedparser.sh`)
assert.ok(0 < illAll.length,
  `feedparser illformed corpus missing. Re-run ./scripts/fetch-feedparser.sh`)

// Documents whose root is not feed/rss/RDF are outside the README's claim
// (RSS 0.90-2.0 + Atom 0.3/1.0) and are reported, not asserted.
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


describe('feedparser: well-formed documents must produce the EXPECTED VALUE', () => {
  test('upstream `Expect:` annotations hold', () => {
    const fails: string[] = []
    const unmapped = new Map<string, number>()
    let supported = 0, checked = 0, unmappedFiles = 0
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
      else checked++
    }
    const top = [...unmapped.entries()].sort((a, b) => b[1] - a[1]).slice(0, 15)
    console.log(
      `feedparser values: ${checked}/${checked + fails.length} correct` +
      ` (${supported} of ${wf.length} files have a machine-checkable Expect;` +
      ` ${unmappedFiles} use paths this harness does not map — known gap)\n` +
      `  top unmapped paths: ${top.map(([k, v]) => `${k}(${v})`).join(' ')}`)
    assert.deepEqual(fails, [],
      `value failures (${fails.length}/${checked + fails.length}):\n${fmtFails(fails)}`)
  })
})


describe('feedparser: ill-formed documents must be REJECTED', () => {
  // NOTE (known gap, deliberately left RED rather than narrowed):
  // feedparser's `bozo` flag is broader than XML well-formedness — it also
  // covers declared-vs-actual character-encoding mismatches (illformed/chardet/*)
  // and GeoRSS/GML coordinate errors (illformed/geo/*), neither of which a
  // string-input XML parser can or should detect. Those cases are left IN the
  // corpus and left FAILING. Do not remove them to get green; either the
  // parser learns to reject them or the gap is stated here in writing.
  test(`${illAll.length} documents`, () => {
    const fails: string[] = []
    for (const p of illAll) {
      let threw = false
      try { atomParse.parse(read(p)) } catch { threw = true }
      if (!threw) fails.push(`ACCEPTED but upstream marks it ill-formed: ${rel(p)}`)
    }
    console.log(
      `feedparser illformed: ${illAll.length - fails.length}/${illAll.length} rejected`)
    assert.deepEqual(fails, [],
      `must-reject failures (${fails.length}/${illAll.length}):\n${fmtFails(fails)}`)
  })
})


// Dialect / version detection.
//
// Two oracles, both the corpus's own, never invented here:
//
//  * `wellformed/atom10/` and `wellformed/rdf/` are version-homogeneous
//    upstream directories, so the directory names the expected version.
//    `wellformed/atom/` and `wellformed/rss/` are NOT — e.g.
//    atom/entry_published_parsed.xml declares the Atom 1.0 namespace and
//    rss/rss_version_090.xml is an RDF document — so no directory-level
//    version expectation is asserted for those two.
//  * The upstream `Expect: ... version == 'X'` annotation, wherever a
//    document carries one. That is feedparser's own recorded answer and it
//    covers every dialect the corpus exercises.
describe('feedparser: dialect / version detection', () => {
  const groups: [string, FeedDialect, FeedVersion[]][] = [
    ['atom10', 'atom', ['atom10']],
    ['rdf', 'rdf', ['rss10', 'rss090']],
  ]
  for (const [sub, dialect, versions] of groups) {
    test(`wellformed/${sub}/* -> ${dialect}`, () => {
      const files = walkXml(join(WELLFORMED, sub))
      assert.ok(0 < files.length, `wellformed/${sub} is empty — corpus wrong?`)
      const fails: string[] = []
      for (const p of files) {
        try {
          const got = detect(rawParse.parse(read(p)) as any)
          if (got.dialect !== dialect || !versions.includes(got.version)) {
            fails.push(`${rel(p)}: ${got.dialect}/${got.version}`)
          }
        } catch (e: any) {
          fails.push(`${rel(p)}: ${String(e?.message).split('\n')[0]}`)
        }
      }
      assert.deepEqual(fails, [],
        `${sub} detection failures (${fails.length}/${files.length}):\n${fmtFails(fails)}`)
    })
  }

  // The dialect implied by the document element, read from the source text.
  test('dialect matches the document element', () => {
    const want: Record<string, string> = { feed: 'atom', rss: 'rss', RDF: 'rdf' }
    const fails: string[] = []
    for (const p of wf) {
      const src = read(p)
      try {
        const got = detect(rawParse.parse(src) as any)
        const w = want[rootLocalName(src)]
        if (got.dialect !== w) fails.push(`${rel(p)}: ${got.dialect} (root <${rootLocalName(src)}>)`)
      } catch (e: any) {
        fails.push(`${rel(p)}: ${String(e?.message).split('\n')[0]}`)
      }
    }
    console.log(`feedparser dialect: ${wf.length - fails.length}/${wf.length} correct`)
    assert.deepEqual(fails, [],
      `dialect failures (${fails.length}/${wf.length}):\n${fmtFails(fails)}`)
  })

  // The upstream `Expect: ... version == 'X'` annotation.
  test("upstream `version == '...'` annotations hold", () => {
    const fails: string[] = []
    let checked = 0
    for (const p of wf) {
      const src = read(p)
      const m = src.match(/version == '([a-z0-9]+)'/)
      if (!m) continue
      checked++
      try {
        const got = detect(rawParse.parse(src) as any)
        if (got.version !== m[1]) fails.push(`${rel(p)}: ${got.version}, upstream says ${m[1]}`)
      } catch (e: any) {
        fails.push(`${rel(p)}: ${String(e?.message).split('\n')[0]}`)
      }
    }
    assert.ok(0 < checked, 'no version annotations found — corpus wrong?')
    console.log(`feedparser version: ${checked - fails.length}/${checked} correct`)
    assert.deepEqual(fails, [],
      `version failures (${fails.length}/${checked}):\n${fmtFails(fails)}`)
  })
})
