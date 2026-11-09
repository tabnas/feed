/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

// A parse from a reader, a window of the source at a time (admin ADR-30;
// tabnas/parser's doc/chunked-source.md, "Proving it"), against the same
// text parsed whole. `make()` declares `lex.window`: the tokens are
// @tabnas/xml's matcher's, which reads the text only through the lexer's
// window, and this plugin's one hook converts the finished element tree,
// so a reader is read a window at a time.
//
// Every row of the shared fixtures and every document of the three feed
// corpora (feedparser-wellformed, feedvalidator and feedparser) is parsed
// whole and from a reader that hands its bytes over in pieces of 1, 3 and
// 64 bytes, under the engine's window and under windows whose margin and
// block are a few units, so that every construct meets the window's edge;
// a long feed, whose long tokens run past the engine's window, in pieces
// of 4 KiB too. The value, the tokens the lexer gives, the rule passes,
// and an error's code, position, row, column, token and message must be
// the same. An error's excerpt is not compared: a parse from a reader
// shows a long line only as far back as the window reaches
// (tabnas/parser's DIVERGENCE.md).
//
// go/window_test.go and rs/tests/window_test.rs hold the Go and Rust ports
// to the same.

import { describe, test } from 'node:test'
import assert from 'node:assert'
import { existsSync } from 'node:fs'
import { join } from 'node:path'

import { Tabnas } from '@tabnas/parser'
import { findSpecDir, loadSpecDir } from '@tabnas/support'

import { Feed, make } from '../dist/feed'
import { REPO, read, rel, requireCorpus, walkXml } from './corpus'

// A reader that hands its bytes over at most `size` at a time.
function pieces(bytes: Uint8Array, size: number) {
  let at = 0
  return {
    read(buf: Uint8Array): number {
      const n = Math.min(size, buf.length, bytes.length - at)
      buf.set(bytes.subarray(at, at + n), 0)
      at += n
      return n
    },
  }
}

// A read of a source: the pieces a reader hands its bytes over in, and the
// window it is read through, the engine's own (64 KiB) where `margin` and
// `block` are absent, or one whose margin and block are a few units, which
// the hidden `parseReaderWithSizes` sets, its reads as small as the pieces.
type Reading = { size: number; margin?: number; block?: number }

const PIECES = [1, 3, 64]
const WINDOWS = [
  {},
  { margin: 1, block: 1 },
  { margin: 3, block: 2 },
  { margin: 64, block: 64 },
]

// A fixture row is read in every pieces size through every window.
const ROW_READS: Reading[] =
  PIECES.flatMap((size) => WINDOWS.map((window) => ({ size, ...window })))

// A corpus document in pieces of 1, 3 and 64 bytes through the engine's
// window, and through windows of 1 and 1, 3 and 2, and 64 and 64 units in
// pieces of 1, 3 and 64.
const DOCUMENT_READS: Reading[] = [
  { size: 1 },
  { size: 3 },
  { size: 64 },
  { size: 1, margin: 1, block: 1 },
  { size: 3, margin: 3, block: 2 },
  { size: 64, margin: 64, block: 64 },
]

// A long feed in pieces of 1, 3, 64 and 4096 bytes through the engine's
// window, whose edges its long tokens straddle, and through a window of 64
// and 64 units.
const LONG_READS: Reading[] = [
  { size: 1 },
  { size: 3 },
  { size: 64 },
  { size: 4096 },
  { size: 64, margin: 64, block: 64 },
]

type Log = { tokens: string[]; rules: string[] }

// `tn` with a log of the tokens the lexer gives and the rule passes.
function logged(tn: Tabnas, log: Log): Tabnas {
  tn.sub({
    lex: (tkn: any) => {
      log.tokens.push(
        tkn.name + ' ' + JSON.stringify(tkn.src) + ' ' +
        JSON.stringify(tkn.val) + ' ' + tkn.sI + ' ' + tkn.rI + ':' + tkn.cI,
      )
    },
    ruleDone: (rule: any, _ctx: any, done: any) => {
      log.rules.push(rule.name + ' ' + done.state + ' ' + rule.d)
    },
  })
  return tn
}

// What a parse showed: its value, or its error without the excerpt
// (`src`, the line it is on); and the tokens and rule passes `log`
// gathered, which is empty again after.
function seen(log: Log, parse: () => any) {
  let outcome: string
  try {
    const value = parse()
    outcome = 'ok ' + (undefined === value ? 'undefined' : JSON.stringify(value))
  } catch (e: any) {
    if (null == e || 'function' !== typeof e.toJSON) {
      outcome = 'throw ' + String(e && e.message)
    } else {
      const { src: _excerpt, ...rest } = e.toJSON()
      outcome = 'err ' + JSON.stringify(rest)
    }
  }
  const tokens = log.tokens
  const rules = log.rules
  log.tokens = []
  log.rules = []
  return { outcome, tokens, rules }
}

// `input` parsed whole and from a reader in each of `reads`; the count of
// reader parses compared. The text is the one its UTF-8 bytes decode to,
// so that a lone surrogate, which no reader gives, cannot set the two
// apart; a byte order mark stays the source's U+FEFF, as the engine
// decodes it.
function differential(
  tn: Tabnas, log: Log, where: string, input: string, reads: Reading[],
): number {
  const bytes = new Uint8Array(Buffer.from(input, 'utf8'))
  const text = new TextDecoder('utf-8', { ignoreBOM: true }).decode(bytes)
  const whole = seen(log, () => tn.parse(text))
  for (const { size, margin, block } of reads) {
    const read = seen(log, () => undefined === margin || undefined === block
      ? tn.parseReader(pieces(bytes, size))
      : tn.parseReaderWithSizes(pieces(bytes, size), { margin, block, chunk: size }))
    const at = where + ' in pieces of ' + size +
      (undefined === margin ? '' : ', margin ' + margin + ', block ' + block)
    assert.equal(read.outcome, whole.outcome, at)
    assert.deepEqual(read.tokens, whole.tokens, at)
    assert.deepEqual(read.rules, whole.rules, at)
  }
  return reads.length
}

// A feed of every kind of token, the long ones longer than the engine's
// 64 KiB window, and characters past ASCII among them, so that tokens of
// every kind straddle the window's edges as a reader hands the text over.
// The same document as `longFeed` in go/window_test.go and `long_feed` in
// rs/tests/window_test.rs.
function longFeed(): string {
  const parts: string[] = []
  parts.push('\uFEFF<?xml version="1.0" encoding="utf-8"?>\n')
  parts.push('<!DOCTYPE feed [\n')
  for (let i = 0; i < 1300; i++) {
    parts.push('  <!ENTITY e' + i + ' "entity \u20ac ' + i + '"> <!-- ' + i + ' > ] \' -->\n')
  }
  parts.push(']>\n')
  parts.push('<!-- ' + 'a comment past the window, \u00e9 \ud834\udd1e - '.repeat(2000) + '-->\n')
  parts.push('<feed xmlns="http://www.w3.org/2005/Atom" xml:lang="en">\n')
  parts.push('  <title type="text">A long feed &amp; its entries</title>\n')
  parts.push('  <id>urn:uuid:60a76c80-d399-11d9-b93C-0003939e0af6</id>\n')
  parts.push('  <updated>2026-11-09T12:00:00Z</updated>\n')
  parts.push('  <link rel="self" href="http://example.org/feed?' +
    'q=a%20long%20query%20past%20the%20window&amp;'.repeat(1600) + '"/>\n')
  for (let i = 0; i < 1000; i++) {
    parts.push('  <entry>\n    <title>Entry ' + i + ' &#' + (60 + (i % 40)) + '; t\u00ebxt</title>\n' +
      '    <id>urn:uuid:' + i + '</id>\n    <updated>2026-11-09T12:00:00Z</updated>\n' +
      '    <author><name>Au\u00f0or ' + i + '</name></author>\r\n' +
      '    <summary type="html">&lt;p&gt;summary ' + i + '&lt;/p&gt;</summary>\n  </entry>\n')
    if (500 === i) {
      parts.push('  <entry>\n    <title>The long one</title>\n    <id>urn:uuid:long</id>\n' +
        '    <updated>2026-11-09T12:00:00Z</updated>\n' +
        '    <content type="html"><![CDATA[' + '<b>cdata past the window</b> \u00e7 ]] > '.repeat(2200) +
        ']]></content>\n' +
        '    <summary>' + 'character data past the window, \u00e5 &amp; &lt;x&gt; \r\n'.repeat(1300) +
        '</summary>\n' +
        '    <?pi ' + 'an instruction past the window \u00f1 ? > '.repeat(1800) + '?>\n' +
        '    <category term="t" label="' + 'a label past the window \u00fc '.repeat(2600) + '"/>\n' +
        '  </entry>\n')
    }
  }
  parts.push('</feed>\n<!-- after -->\n')
  return parts.join('')
}

describe('window', () => {
  test('make reads a reader a window at a time', () => {
    assert.equal((make().options as any).lex.window, true)
    assert.equal((make({ format: 'native' }).options as any).lex.window, true)
    // The plugin itself declares nothing.
    assert.notEqual((new Tabnas().use(Feed).options as any).lex.window, true)
  })

  test('every fixture row parses the same from a reader', () => {
    const log: Log = { tokens: [], rules: [] }
    const files = loadSpecDir(findSpecDir(__dirname))
    let rows = 0
    let compared = 0
    files.forEach((file, index) => {
      const mode = file.header[1]
      for (const row of file.rows) {
        const raw = row.named('opts')
        const opts = 'detect' === mode
          ? { format: 'raw' }
          : '' === (raw || '').trim() ? {} : JSON.parse(raw)
        const tn = logged(make(opts), log)
        compared += differential(tn, log, row.where(), row.unescNamed('input'), ROW_READS)
        rows++
      }
      console.log(
        'window fixtures: ' + (index + 1) + ' of ' + files.length + ' files (' +
        Math.floor(((index + 1) * 100) / files.length) + '%), ' + compared + ' reader parses',
      )
    })
    assert.ok(50 < rows, String(rows))
    assert.equal(compared, rows * ROW_READS.length)
  })

  test('every document of the feed corpora parses the same from a reader', () => {
    const log: Log = { tokens: [], rules: [] }
    const tn = logged(make(), log)
    const wellformed = join(REPO, 'test', 'feedparser-wellformed')
    assert.ok(existsSync(wellformed), wellformed)
    const files = [
      ...walkXml(wellformed),
      ...walkXml(requireCorpus('feedvalidator', 'fetch-feedvalidator.sh')),
      ...walkXml(requireCorpus('feedparser', 'fetch-feedparser.sh')),
    ]
    assert.ok(3500 < files.length, String(files.length))
    let compared = 0
    let next = 0
    files.forEach((file, index) => {
      compared += differential(tn, log, rel(file), read(file), DOCUMENT_READS)
      if (index + 1 === files.length || next <= index) {
        next = index + 500
        console.log(
          'window corpora: ' + (index + 1) + ' of ' + files.length + ' documents (' +
          Math.floor(((index + 1) * 100) / files.length) + '%), ' + compared + ' reader parses',
        )
      }
    })
  })

  test('a long feed parses the same from a reader', () => {
    const log: Log = { tokens: [], rules: [] }
    const tn = logged(make(), log)
    const text = longFeed()
    assert.ok(512 * 1024 < text.length, String(text.length))
    const value = tn.parse(text)
    log.tokens = []
    log.rules = []
    assert.equal(value.entries.length, 1001)
    assert.equal(differential(tn, log, 'the long feed', text, LONG_READS), LONG_READS.length)
  })

  test('a long document fails the same from a reader', () => {
    // A construct that runs past the engine's window and is then refused,
    // and a long document that is no feed: the error is the whole text's.
    const log: Log = { tokens: [], rules: [] }
    const tn = logged(make(), log)
    const pad = '<title>' + 'x\u00e9 '.repeat(30000) + '</title>'
    const bad = [
      '<feed xmlns="http://www.w3.org/2005/Atom">' + pad + '<!-- ' + 'c '.repeat(40000),
      '<feed xmlns="http://www.w3.org/2005/Atom">' + pad + '<subtitle>' +
        't '.repeat(40000) + ']]></subtitle></feed>',
      '<kml>' + pad + '</kml>',
      '<rss version="2.0"><channel>' + pad + '</channel></rss>' + 'trailing '.repeat(10000),
    ]
    let compared = 0
    bad.forEach((text, index) => {
      const whole = seen(log, () => tn.parse(text))
      assert.ok(!whole.outcome.startsWith('ok '), whole.outcome.slice(0, 200))
      compared += differential(tn, log, 'long error document ' + index, text, LONG_READS)
    })
    assert.equal(compared, bad.length * LONG_READS.length)
  })
})
