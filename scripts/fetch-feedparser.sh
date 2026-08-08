#!/usr/bin/env bash
# Fetch the kurtmckee/feedparser test corpus at a PINNED commit.
#
#   upstream: https://github.com/kurtmckee/feedparser
#   commit:   a22c5521cbb109871f1a2318948581901bd47e26   (pinned; see fetch-corpus.mjs)
#   into:     test/feedparser/     (gitignored — the corpus is NEVER committed)
#   license:  BSD 2-Clause, fetched verbatim to test/feedparser/LICENSE
#
# Idempotent. Thin wrapper over scripts/fetch-corpus.mjs so the same logic
# serves `npm pretest` on every platform.
set -euo pipefail
exec node "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/fetch-corpus.mjs" feedparser
