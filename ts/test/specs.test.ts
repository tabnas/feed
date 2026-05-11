/* Copyright (c) 2021-2025 Richard Rodger and other contributors, MIT License */

// Drives the cross-language fixtures in test/specs/. Each fixture pairs a
// .xml input with one or more expected JSON outputs (.atom.json,
// .native.json, .detect.json) and is run by both this test file and
// go/feed_test.go. See test/specs/README.md.

import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readdirSync, readFileSync, existsSync } from 'node:fs'
import { join } from 'node:path'

import { Jsonic } from 'jsonic'
import { Feed, detect } from '../dist/feed.js'

const SPECS_DIR = join(__dirname, '..', '..', 'test', 'specs')

const atomParse = Jsonic.make().use(Feed)
const nativeParse = Jsonic.make().use(Feed, { format: 'native' })
const rawParse = Jsonic.make().use(Feed, { format: 'raw' })

// canon JSON-roundtrips a value so deep-equal sees the same shape regardless
// of property ordering or class identity.
function canon<T>(v: T): T {
  return JSON.parse(JSON.stringify(v))
}

const specs = existsSync(SPECS_DIR)
  ? readdirSync(SPECS_DIR)
      .filter((f) => f.endsWith('.xml'))
      .map((f) => f.replace(/\.xml$/, ''))
      .sort()
  : []


describe('specs - dialect / version detection', () => {
  if (specs.length === 0) {
    test('no specs', () => {
      console.warn(`no specs in ${SPECS_DIR}`)
    })
    return
  }
  for (const name of specs) {
    const detectPath = join(SPECS_DIR, name + '.detect.json')
    if (!existsSync(detectPath)) continue
    test(name, () => {
      const src = readFileSync(join(SPECS_DIR, name + '.xml'), 'utf8')
      const root: any = rawParse(src)
      const got = detect(root)
      const want = JSON.parse(readFileSync(detectPath, 'utf8'))
      assert.deepEqual(canon(got), want)
    })
  }
})


describe('specs - atom (default) output', () => {
  for (const name of specs) {
    const expectedPath = join(SPECS_DIR, name + '.atom.json')
    if (!existsSync(expectedPath)) continue
    test(name, () => {
      const src = readFileSync(join(SPECS_DIR, name + '.xml'), 'utf8')
      const got = atomParse(src)
      const want = JSON.parse(readFileSync(expectedPath, 'utf8'))
      assert.deepEqual(canon(got), want)
    })
  }
})


describe('specs - native output', () => {
  for (const name of specs) {
    const expectedPath = join(SPECS_DIR, name + '.native.json')
    if (!existsSync(expectedPath)) continue
    test(name, () => {
      const src = readFileSync(join(SPECS_DIR, name + '.xml'), 'utf8')
      const got = nativeParse(src)
      const want = JSON.parse(readFileSync(expectedPath, 'utf8'))
      assert.deepEqual(canon(got), want)
    })
  }
})
