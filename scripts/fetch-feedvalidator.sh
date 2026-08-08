#!/usr/bin/env bash
# Fetch the rubys/feedvalidator test corpus at a PINNED commit.
#
# This is the corpus behind the W3C Feed Validation Service — the most widely
# cited third-party conformance suite for RSS/Atom. The WHOLE testcases/ tree
# is fetched: no selection, no filtering.
#
#   upstream: https://github.com/rubys/feedvalidator
#   commit:   2a8050b950594464b3923af249623b614774c138   (pinned; see fetch-corpus.mjs)
#   into:     test/feedvalidator/  (gitignored — the corpus is NEVER committed)
#
# Idempotent. Thin wrapper over scripts/fetch-corpus.mjs so the same logic
# serves `npm pretest` on every platform.
set -euo pipefail
exec node "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/fetch-corpus.mjs" feedvalidator
