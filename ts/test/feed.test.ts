/* Copyright (c) 2021-2025 Richard Rodger and other contributors, MIT License */

// TS-only behaviour checks: error paths, the `raw` output mode, and the
// shape of the plugin registration. The bulk of dialect-by-dialect
// expectations lives in test/specs/ (driven by both this language and
// the Go test suite).

import { test, describe } from 'node:test'
import assert from 'node:assert'

import { Jsonic } from 'jsonic'
import { Feed, detect } from '../dist/feed.js'
import type { AtomFeed } from '../dist/feed.js'


describe('feed - errors', () => {
  test('unrecognized root element throws', () => {
    const j = Jsonic.make().use(Feed)
    assert.throws(() => j('<not-a-feed/>'), /unrecognized root/)
  })
})


describe('feed - raw format', () => {
  test('returns the underlying XmlElement tree', () => {
    const j = Jsonic.make().use(Feed, { format: 'raw' })
    const root: any = j('<feed xmlns="http://www.w3.org/2005/Atom"><title>Hi</title></feed>')
    assert.equal(root.localName, 'feed')
    assert.equal(root.namespace, 'http://www.w3.org/2005/Atom')
    assert.ok(Array.isArray(root.children))
  })

  test('detect on raw output identifies the dialect', () => {
    const j = Jsonic.make().use(Feed, { format: 'raw' })
    const root: any = j('<rss version="2.0"><channel><title>x</title></channel></rss>')
    assert.deepEqual(detect(root), { dialect: 'rss', version: 'rss20' })
  })
})


describe('feed - plugin form', () => {
  test('default is atom', () => {
    const j = Jsonic.make().use(Feed)
    const f = j('<feed xmlns="http://www.w3.org/2005/Atom"/>') as AtomFeed
    assert.equal(f.format, 'atom')
    assert.equal(f.version, '1.0')
  })

  test('format=native preserves the dialect shape', () => {
    const j = Jsonic.make().use(Feed, { format: 'native' })
    const f: any = j('<rss version="2.0"><channel><title>x</title></channel></rss>')
    assert.equal(f.format, 'rss')
    assert.equal(f.version, '2.0')
  })

  test('parse runs cleanly when the input has trailing whitespace', () => {
    // bc on the xml rule fires twice in this case; the plugin must guard
    // against re-converting the already-converted result.
    const j = Jsonic.make().use(Feed)
    const f = j('<feed xmlns="http://www.w3.org/2005/Atom"><title>Hi</title></feed>\n  \n') as AtomFeed
    assert.equal(f.format, 'atom')
    assert.equal(f.title?.value, 'Hi')
  })
})
