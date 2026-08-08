# Third-Party Notices

This project incorporates material from the projects listed below. The
original copyright notices and license texts are preserved as required.

Two further corpora are **fetched, not distributed**: `test/feedvalidator/`
(rubys/feedvalidator) and `test/feedparser/` (the full kurtmckee/feedparser
tree). `scripts/fetch-corpus.mjs` clones each at a pinned commit into a
gitignored directory and copies the upstream `LICENSE` alongside it. Neither
is committed here or shipped in the npm package.

## kurtmckee/feedparser

A focused subset of well-formed feed samples (`atom10`, `atom`, `rss`,
`rdf`) from `tests/wellformed/` of the upstream `kurtmckee/feedparser`
project is vendored at `test/feedparser-wellformed/`. The upstream
`LICENSE` is preserved in place at
`test/feedparser-wellformed/LICENSE`.

- Project: https://github.com/kurtmckee/feedparser
- License: BSD 2-Clause "Simplified" License
- Copyright (C) 2010-2025 Kurt McKee
- Copyright (C) 2002-2008 Mark Pilgrim

Full upstream license text:

```
Copyright (C) 2010-2025 Kurt McKee <contactme@kurtmckee.org>
Copyright (C) 2002-2008 Mark Pilgrim
All rights reserved.

Redistribution and use in source and binary forms, with or without modification,
are permitted provided that the following conditions are met:

*   Redistributions of source code must retain the above copyright notice,
    this list of conditions and the following disclaimer.
*   Redistributions in binary form must reproduce the above copyright notice,
    this list of conditions and the following disclaimer in the documentation
    and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS 'AS IS'
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE
LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.
```
