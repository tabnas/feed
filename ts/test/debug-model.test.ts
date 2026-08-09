/* Copyright (c) 2021-2026 Richard Rodger and other contributors, MIT License */

// Composition test: the feed grammar plugin layered with the official
// @tabnas/debug plugin. @tabnas/debug is a DECLARED devDependency of this
// package, so it must be present. This used to SKIP when it could not be
// resolved, which meant a broken install silently removed the whole
// composition suite while the run still reported green. It now FAILS with
// instructions instead. Set TABNAS_DEBUG_PATH to point at a sibling
// checkout's built plugin if it is not installed under node_modules.

import { test, describe } from 'node:test'
import assert from 'node:assert'

import { Tabnas } from '@tabnas/parser'
import { jsonic } from '@tabnas/jsonic'
import { Feed } from '../dist/feed.js'

// This package compiles to CommonJS (see tsconfig module=nodenext, no
// "type":"module"), so `require` is available at runtime; declare it so the
// TS compiler is satisfied without pulling in import.meta.
declare const require: (id: string) => any

function loadDebug(): any {
  const candidates = [process.env.TABNAS_DEBUG_PATH, '@tabnas/debug'].filter(
    Boolean,
  ) as string[]
  for (const c of candidates) {
    try {
      return require(c).Debug
    } catch {
      /* try next */
    }
  }
  return null
}

const Debug = loadDebug()
if (!Debug) {
  throw new Error(
    '\n\n@tabnas/debug could not be resolved, but it is a declared devDependency ' +
    'of this package and this composition test must not be skipped.\n' +
    'Install it (npm i in ts/) or set TABNAS_DEBUG_PATH to a built sibling checkout.\n',
  )
}


describe('compose: feed + @tabnas/debug', () => {
  test('parses normally with the debug plugin installed', () => {
    const tn = new Tabnas().use(jsonic).use(Feed)
    tn.use(Debug, { print: false, trace: false })
    const f: any = tn.parse(
      '<feed xmlns="http://www.w3.org/2005/Atom"><title>Hi</title></feed>',
    )
    assert.equal(f.format, 'atom')
    assert.equal(f.title?.value, 'Hi')
  })

  test('debug.model() returns the structured grammar', () => {
    const tn = new Tabnas().use(jsonic).use(Feed)
    tn.use(Debug, { print: false, trace: false })
    const m: any = tn.debug.model()

    // The structured rule set: feed is built on @tabnas/xml, whose grammar
    // is the xml / element / content / child rules. The feed plugin only
    // hooks the existing `xml` rule's bc, so it adds no rules of its own.
    assert.deepStrictEqual(
      m.rules.map((r: any) => r.name).sort(),
      ['child', 'content', 'element', 'xml'],
    )

    // Entry rule.
    assert.equal(m.config.start, 'xml')

    // The plugin stack lists the feed plugin and the xml plugin it pulls in.
    assert.ok(
      m.plugins.some((p: any) => p.name === 'Feed'),
      'plugins should list Feed',
    )
    assert.ok(
      m.plugins.some((p: any) => p.name === 'Xml'),
      'plugins should list Xml',
    )

    // Structural facts of the xml grammar: the start rule opens an element,
    // and elements recurse into other elements via content -> child.
    const xml = m.rules.find((r: any) => r.name === 'xml')
    assert.ok(
      xml.open.some((a: any) => a.push === 'element'),
      'xml should push element',
    )
    const child = m.rules.find((r: any) => r.name === 'child')
    assert.ok(
      child.open.some((a: any) => a.push === 'element'),
      'child should push element (nested elements)',
    )

    // The grammar portion is JSON-serialisable and round-trips.
    assert.deepStrictEqual(
      JSON.parse(JSON.stringify({ rules: m.rules })).rules,
      m.rules,
    )
  })
})
