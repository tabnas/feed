// Fetch a third-party conformance corpus at a PINNED commit.
//
// The corpora are NEVER committed to this repo (project rule: no vendored
// third-party test corpus). They land in gitignored directories under test/.
// Only this script and the pinned SHAs below are tracked.
//
// Implemented in Node rather than shell so `npm pretest` works on Windows CI
// too; scripts/fetch-feedparser.sh and scripts/fetch-feedvalidator.sh are
// thin wrappers around it and remain the documented entry points.
//
// Usage:  node scripts/fetch-corpus.mjs [feedparser|feedvalidator|all]
//
// Idempotent: a corpus already at the pinned SHA is left alone.

import { execFileSync } from 'node:child_process'
import { mkdtempSync, rmSync, mkdirSync, cpSync, copyFileSync, existsSync, readFileSync, writeFileSync, readdirSync, statSync } from 'node:fs'
import { join, dirname, resolve } from 'node:path'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'

const REPO = resolve(dirname(fileURLToPath(import.meta.url)), '..')

const SUITES = {
  feedparser: {
    url: 'https://github.com/kurtmckee/feedparser',
    // Pinned commit. Bump deliberately; never a branch name.
    commit: 'a22c5521cbb109871f1a2318948581901bd47e26',
    dest: 'test/feedparser',
    copy: [['tests/wellformed', 'wellformed'], ['tests/illformed', 'illformed']],
    license: ['LICENSE'],
    note: 'BSD 2-Clause (LICENSE, verbatim from upstream root)',
  },
  feedvalidator: {
    url: 'https://github.com/rubys/feedvalidator',
    // Pinned commit. Bump deliberately; never a branch name.
    commit: '2a8050b950594464b3923af249623b614774c138',
    dest: 'test/feedvalidator',
    // The WHOLE testcases/ tree — no selection, no filtering. The harness
    // classifies each case from its own upstream `Expect:` annotation.
    copy: [['testcases', 'testcases']],
    license: ['LICENSE', 'LICENSE.txt', 'COPYING'],
    note: 'see LICENSE (verbatim from upstream root)',
  },
}

function git(args, cwd) {
  return execFileSync('git', args, { cwd, stdio: ['ignore', 'pipe', 'inherit'] }).toString().trim()
}

function countXml(dir) {
  let n = 0
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name)
    if (e.isDirectory()) n += countXml(p)
    else if (e.name.endsWith('.xml')) n++
  }
  return n
}

function fetchSuite(name) {
  const s = SUITES[name]
  const dest = join(REPO, s.dest)
  const stamp = join(dest, 'UPSTREAM')

  if (existsSync(stamp) && readFileSync(stamp, 'utf8').includes(`commit: ${s.commit}`)) {
    console.log(`${name} corpus already at ${s.commit}`)
    return
  }

  const tmp = mkdtempSync(join(tmpdir(), `tabnas-${name}-`))
  try {
    const src = join(tmp, 'src')
    mkdirSync(src)
    console.log(`fetching ${s.url} @ ${s.commit} ...`)
    git(['init', '-q'], src)
    git(['remote', 'add', 'origin', s.url], src)
    git(['fetch', '-q', '--depth', '1', 'origin', s.commit], src)
    git(['checkout', '-q', 'FETCH_HEAD'], src)

    const got = git(['rev-parse', 'HEAD'], src)
    if (got !== s.commit) {
      throw new Error(`expected ${s.commit}, got ${got}`)
    }

    rmSync(dest, { recursive: true, force: true })
    mkdirSync(dest, { recursive: true })
    for (const [from, to] of s.copy) {
      cpSync(join(src, from), join(dest, to), { recursive: true })
    }
    for (const l of s.license) {
      if (existsSync(join(src, l))) { copyFileSync(join(src, l), join(dest, 'LICENSE')); break }
    }
    writeFileSync(stamp,
      `upstream: ${s.url}\n` +
      `commit: ${s.commit}\n` +
      `paths: ${s.copy.map(([a, b]) => `${a}/ -> ${b}/`).join(', ')}\n` +
      `license: ${s.note}\n` +
      `NOTE: this directory is FETCHED, not committed. See scripts/fetch-corpus.mjs\n`)
    console.log(`${name} corpus at ${dest} (${countXml(dest)} xml files)`)
  } finally {
    rmSync(tmp, { recursive: true, force: true })
  }
}

const which = process.argv[2] ?? 'all'
const names = 'all' === which ? Object.keys(SUITES) : [which]
for (const n of names) {
  if (!SUITES[n]) {
    console.error(`unknown suite: ${n} (known: ${Object.keys(SUITES).join(', ')}, all)`)
    process.exit(2)
  }
  fetchSuite(n)
}
