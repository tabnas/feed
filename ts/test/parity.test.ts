/* Copyright (c) 2025 Richard Rodger and other contributors, MIT License */

// Cross-runtime conformance, driven by the shared `test/spec/*.tsv` fixtures
// at the repo root (see ../../test/AGENTS.md).
//
// The fixture loader, the escape codec, the `ERROR:` contract and the row
// loop all come from @tabnas/support, whose Go half `go/parity_test.go`
// uses to run the SAME files — so the two implementations cannot drift
// without one of them going red, and neither can the two loaders.
//
// What is left here is only what is specific to feed: the two fixture
// modes, and what an `ERROR:` cell means.

import { Tabnas } from '@tabnas/parser'
import { jsonic } from '@tabnas/jsonic'
import { findSpecDir, loadSpecDir, makeRunner } from '@tabnas/support'

import { Feed, detect } from '../dist/feed'

// Flatten through JSON so class identity, property order and — the one
// that matters here — keys whose value is `undefined` do not affect the
// structural comparison. The feed model sets absent fields explicitly, and
// `{a: 1, b: undefined}` is not the same object as `{a: 1}` to a
// key-counting comparison, though it renders identically. The same
// normalisation go/parity_test.go does.
//
// `undefined` itself is left alone: a fixture that says `null` must still
// not be satisfied by a parse that produced nothing.
function jsonFlatten(v: unknown): unknown {
  if (undefined === v) return v
  try {
    return JSON.parse(JSON.stringify(v))
  }
  catch {
    return v
  }
}

// A fixture's SECOND COLUMN HEADER says what it asserts: `expected` is the
// parsed feed, `detect` is the format name the raw parse is recognised as.
// That is per file, which is why there is a runner per file rather than
// one over the directory.
for (const spec of loadSpecDir(findSpecDir(__dirname))) {
  const mode = spec.header[1]
  if ('expected' !== mode && 'detect' !== mode) {
    throw new Error(
      `${spec.file}: unknown second column ${JSON.stringify(mode)}`)
  }

  makeRunner({
    parse: (input, row) => {
      const raw = row.named('opts')
      const opts = '' === raw.trim() ? {} : JSON.parse(raw)

      // Detection is asserted over the RAW parse, so those fixtures pin
      // the format rather than passing their own options.
      const parsed = new Tabnas()
        .use(jsonic)
        .use(Feed, 'detect' === mode ? { format: 'raw' } : opts)
        .parse(input)

      return 'detect' === mode ? detect(parsed as any) : parsed
    },

    // feed's `ERROR:<want>` cells hold a fragment of the MESSAGE — 'unrecognized
    // root element "kml"', 'character data is not allowed outside the root
    // element' — rather than an error code. These rejections come from the
    // feed layer's own validation, which reports what is wrong in prose
    // rather than through a code the engine assigns. A bare `ERROR` still
    // accepts any failure.
    matchError: (err: any, want) => String(err?.message).includes(want),

    normalize: jsonFlatten,

    expected: mode,
  }).spec(spec)
}
