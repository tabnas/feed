/* Copyright (c) 2026 Richard Rodger, MIT License */

// The exported VERSION must equal package.json "version".
//
// This is the CI check for version drift. It exists because the constant HAS
// drifted: @tabnas/json exported Version = '1.0.0' for several releases while
// the package shipped 0.4.x, because nothing rewrote it and AGENTS.md wrongly
// claimed `make publish-go` kept it in sync. A release that bumps
// package.json and forgets the constant now fails here.

import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

// Read package.json at runtime rather than importing it: it sits outside the
// test rootDir. Any failure here is a hard failure, never a skip — a version
// check that silently does not run is the failure mode being designed out.
function readPackageJson(): { name: string; version: string } {
  const path = join(__dirname, '..', 'package.json')
  let raw: string
  try {
    raw = readFileSync(path, 'utf8')
  } catch (err) {
    throw new Error(
      `cannot read ${path}, so VERSION cannot be checked: ${err}`)
  }
  const pkg = JSON.parse(raw)
  if ('string' !== typeof pkg.version || '' === pkg.version) {
    throw new Error(`${path} has no version field`)
  }
  return pkg
}

// Resolve through the package root exactly as a consumer would, so that a
// VERSION which exists in src but is not reachable from the package entry
// point still fails.
const api: any = require('..')
const pkg = readPackageJson()


describe('version', () => {
  test('VERSION matches package.json', () => {
    assert.equal(
      api.VERSION,
      pkg.version,
      `VERSION drift: ${pkg.name} exports ${api.VERSION} but package.json is ` +
        `${pkg.version}. Both are rewritten by admin/publish.sh at release; ` +
        `if you bumped one by hand, bump the other.`,
    )
  })

  test('VERSION is exported and looks like a semver', () => {
    assert.equal(
      typeof api.VERSION, 'string', 'VERSION must be exported as a string')
    assert.match(api.VERSION, /^\d+\.\d+\.\d+/, 'VERSION must be a semver')
  })
})
