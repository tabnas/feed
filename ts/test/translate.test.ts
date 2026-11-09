/* Copyright (c) 2021-2026 Richard Rodger, MIT License */

// The translation parts (admin ADR-27): what the manifest names and what
// the package hands a host are the same texts. ../embed-translate.js
// generates src/translate.ts from tabnas.plugin.json and alchemy/*.alc,
// so a part changed without `npm run embed` fails here.

import * as assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import * as path from 'node:path'
import { test } from 'node:test'

import { translate } from '../dist/feed.js'

const root = path.resolve(__dirname, '..', '..')
const read = (rel: string) => readFileSync(path.join(root, rel), 'utf8')

// The names a library of alchemy definitions defines, in its order.
function definitions (text: string): string[] {
  return text
    .split('\n')
    .filter((line) => line.startsWith('def '))
    .map((line) => line.slice(4).split(/\s/)[0])
}

test('translation parts expose the manifest, the render, the embedding and the transducer', () => {
  const parts = translate()
  assert.ok(parts)
  assert.equal(parts.manifest, read('tabnas.plugin.json'))
  assert.equal(parts.lift, undefined)
  assert.equal(parts.render?.entry, 'feed-render')
  assert.equal(parts.render?.source, read('alchemy/render.alc'))
  assert.equal(parts.embed?.entry, 'feed-embed')
  assert.equal(parts.embed?.source, read('alchemy/embed.alc'))
  assert.equal(parts.transduce?.entry, 'feed-transduce')
  assert.equal(parts.transduce?.source, read('alchemy/transduce.alc'))
})

test('the manifest says what the parts are', () => {
  const spec = JSON.parse(read('tabnas.plugin.json'))
  assert.equal(spec.languageId, 'feed')
  const t = spec.translate
  assert.equal(t.reads, 'tree')
  assert.equal(t.writes, 'tree')
  assert.equal(t.root, 'object')
  assert.equal(t.schema, 'feed')
  assert.equal(t.embed, 'alchemy/embed.alc')
  assert.equal(t.render, 'alchemy/render.alc')
  assert.equal(t.transduce, 'alchemy/transduce.alc')
  assert.equal(t.lift, undefined)
  // A host that reads parser events reads the tree through the transducer
  // part as the passes come, so no reason is given for reading it whole.
  assert.equal(t.whole, undefined)
  assert.ok(0 < t.loss.length)
  for (const line of t.loss) {
    assert.match(line, /^[A-Z].*\.$/, `${line} is not a sentence`)
  }
})

// A host links the render, the embedding and the transducer part with its
// own program and other formats' parts into one namespace, so every
// definition is named for the format, the transducer's feed-transduce-...,
// no file defines an export, and no name is defined in two of them.
test('the parts are libraries named for the format', () => {
  const parts = translate()
  const render = definitions(parts?.render?.source ?? '')
  const embed = definitions(parts?.embed?.source ?? '')
  const transduce = definitions(parts?.transduce?.source ?? '')
  assert.ok(render.includes('feed-render'), render.join(' '))
  assert.ok(embed.includes('feed-embed'), embed.join(' '))
  assert.ok(embed.includes('feed-unembed'), embed.join(' '))
  assert.ok(transduce.includes('feed-transduce'), transduce.join(' '))
  for (const name of [...render, ...embed, ...transduce]) {
    assert.ok(name.startsWith('feed-'), `${name} is not named for feed`)
  }
  for (const name of transduce) {
    assert.ok(name.startsWith('feed-transduce'), `${name} is not named for the transducer part`)
  }
  for (const [a, b] of [[embed, render], [transduce, render], [transduce, embed]]) {
    for (const name of a) {
      assert.ok(!b.includes(name), `${name} is defined by two parts`)
    }
  }
})

// The transducer part reads a host's parser events, the grammar's rule
// passes, through its one entry point, and every failure of its own is one
// a host knows: a document the part does not stream (fail :unstreamable,
// which a host reads whole), a root the reader refuses (fail :invalid),
// and passes no document makes (fail :protocol-order).
test('the transducer reads parser events and fails in a host\'s codes', () => {
  const text = translate()?.transduce?.source ?? ''
  assert.ok(text.includes('\ndef feed-transduce [input]\n'), text)
  assert.ok(text.includes('(parser-events input)'), text)
  const codes = [...text.matchAll(/fail (:[a-z-]+)/g)].map((m) => m[1])
  assert.ok(codes.includes(':unstreamable'), codes.join(' '))
  for (const code of codes) {
    assert.ok([':unstreamable', ':invalid', ':protocol-order'].includes(code), `the part fails with ${code}`)
  }
})
