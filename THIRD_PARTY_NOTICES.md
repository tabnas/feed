# Third-Party Notices

This project incorporates material from the projects listed below. The
original copyright notices and license texts are preserved as required.

> **Nothing in this section is vendored.** Both corpora below are FETCHED at
> a pinned commit by `scripts/fetch-corpus.mjs` (via
> `scripts/fetch-feedparser.sh` / `scripts/fetch-feedvalidator.sh`) into
> gitignored directories. This repository redistributes no third-party test
> corpus; only the fetch script and the pinned SHAs are tracked.

## rubys/feedvalidator

The test corpus behind the W3C Feed Validation Service. Fetched in full
(`testcases/`) to `test/feedvalidator/`, pinned at commit
`2a8050b950594464b3923af249623b614774c138`. The upstream licence file is
fetched alongside it to `test/feedvalidator/LICENSE`.

- Project: https://github.com/rubys/feedvalidator
- Copyright (c) Sam Ruby and Mark Pilgrim

## kurtmckee/feedparser

The upstream `tests/wellformed/` and `tests/illformed/` trees are fetched
to `test/feedparser/`, pinned at commit
`a22c5521cbb109871f1a2318948581901bd47e26`. The upstream `LICENSE` is
fetched alongside them to `test/feedparser/LICENSE`.

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
