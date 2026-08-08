/* Copyright (c) 2021-2026 Richard Rodger and other contributors, MIT License */

// Shared helpers for the third-party conformance corpora.
//
// The corpora are NOT committed to this repo (project rule: never vendor a
// third-party test corpus). They are fetched at a pinned commit by
// `scripts/fetch-feedparser.sh` and `scripts/fetch-feedvalidator.sh`, into
// gitignored directories.
//
// A conformance suite that quietly does not run is worse than no suite at
// all, because the green tick is a lie. So `requireCorpus` THROWS when the
// corpus is absent — it never skips.

import { readdirSync, readFileSync, existsSync } from 'node:fs'
import { join } from 'node:path'

// dist-test/ -> ts/ -> repo root
export const REPO = join(__dirname, '..', '..')

export function requireCorpus(dir: string, script: string): string {
  const full = join(REPO, 'test', dir)
  if (!existsSync(full)) {
    throw new Error(
      `\n\nCONFORMANCE CORPUS MISSING: ${full}\n` +
      `This suite cannot run without it, and must never be skipped.\n` +
      `Fetch it (pinned commit, idempotent):\n\n` +
      `    ./scripts/${script}\n\n` +
      `See scripts/${script} for the upstream URL and pinned SHA.\n`,
    )
  }
  return full
}

export function walkXml(dir: string, out: string[] = []): string[] {
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name)
    if (e.isDirectory()) walkXml(p, out)
    else if (e.name.endsWith('.xml')) out.push(p)
  }
  return out.sort()
}

export function read(p: string): string {
  return readFileSync(p, 'utf8')
}

export function rel(p: string): string {
  return p.startsWith(REPO) ? p.slice(REPO.length + 1) : p
}

// The document element's local name, read from the SOURCE TEXT — deliberately
// independent of our own parser, so the classification cannot be biased by
// the thing under test.
export function rootLocalName(src: string): string {
  const s = src
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/<\?[\s\S]*?\?>/g, '')
    .replace(/<!DOCTYPE[\s\S]*?>/gi, '')
  const m = s.match(/<\s*([A-Za-z_][-A-Za-z0-9_:.]*)/)
  if (!m) return ''
  const n = m[1]
  const i = n.lastIndexOf(':')
  return 0 <= i ? n.slice(i + 1) : n
}

// The set of document elements @tabnas/feed claims: RSS 0.90-2.0 and
// Atom 0.3/1.0. Anything else (KML, OpenSearch, OPML, RSS 1.1, APP) is
// outside the README's claim.
export function isFeedRoot(src: string): boolean {
  const n = rootLocalName(src)
  return 'feed' === n || 'rss' === n || 'RDF' === n
}

export function fmtFails(fails: string[], limit = 40): string {
  const head = fails.slice(0, limit).map((f) => '  ' + f).join('\n')
  const more = limit < fails.length ? `\n  ... and ${fails.length - limit} more` : ''
  return head + more
}
