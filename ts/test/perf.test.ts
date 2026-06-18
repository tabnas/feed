/* Copyright (c) 2021-2025 Richard Rodger and other contributors, MIT License */

// Performance regression guard. The feed package exposes the Feed plugin,
// not a package-level convenience parse: callers build a parser themselves
// (new Tabnas().use(jsonic).use(Feed)) and are expected to REUSE it across
// parses. Building that parser (the xml plugin + grammar) dominates a parse,
// so rebuilding it on every parse is many times slower than reusing one
// instance.
//
// This test pins that expectation: it times N parses that rebuild a fresh
// parser each iteration against N parses that reuse a single parser, on the
// SAME machine in the SAME run. The comparison is machine-INDEPENDENT (both
// sides scale together on a slow CI box) and there is deliberately NO
// wall-clock budget. If someone later adds a convenience parse() that
// rebuilds the grammar per call — the bug fixed across these plugins — a
// test of this shape would catch it the same way.

import { test, describe } from 'node:test'
import assert from 'node:assert'

import { Tabnas } from '@tabnas/parser'
import { jsonic } from '@tabnas/jsonic'
import { Feed } from '../dist/feed.js'

function makeFeedParser() {
  return new Tabnas().use(jsonic).use(Feed)
}

describe('feed - performance', () => {
  test('reusing a parser is much faster than rebuilding one per parse', () => {
    const src =
      '<rss version="2.0"><channel><title>x</title>' +
      '<item><title>i</title></item></channel></rss>'
    const n = 300

    // Warm both paths so the comparison is steady-state.
    for (let i = 0; i < 20; i++) makeFeedParser().parse(src)
    const reused = makeFeedParser()
    for (let i = 0; i < 20; i++) reused.parse(src)

    // Rebuild-per-parse: the anti-pattern (what a non-cached convenience
    // parse() would do internally).
    const t0 = process.hrtime.bigint()
    for (let i = 0; i < n; i++) makeFeedParser().parse(src)
    const rebuild = Number(process.hrtime.bigint() - t0)

    // Reuse one parser: the documented, efficient usage.
    const t1 = process.hrtime.bigint()
    for (let i = 0; i < n; i++) reused.parse(src)
    const reuse = Number(process.hrtime.bigint() - t1)

    const speedup = rebuild / reuse
    // Reusing a parser must be far cheaper than rebuilding it per parse.
    // Building the grammar dominates, so rebuild is many times slower here;
    // requiring at least a 4x speedup catches a regression where instance
    // reuse stops paying off (e.g. the grammar is rebuilt on every parse)
    // without depending on absolute wall-clock speed.
    assert.ok(
      reuse * 4 <= rebuild,
      `reusing a feed parser is not meaningfully faster than rebuilding one ` +
        `per parse: ${n} reuse parses took ${reuse}ns vs ${rebuild}ns ` +
        `rebuilding each time (speedup ${speedup.toFixed(1)}x, want >=4x). ` +
        `Building the parser (xml plugin + grammar) should dominate, so reuse ` +
        `should win big; reuse a single parser across parses.`,
    )
    console.log(
      `rebuild-per-parse=${(rebuild / 1e6).toFixed(2)}ms  ` +
        `reuse=${(reuse / 1e6).toFixed(2)}ms  speedup=${speedup.toFixed(2)}x`,
    )
  })
})
