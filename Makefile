# Build, test and publish the TypeScript (ts/), Go (go/) and Rust (rs/)
# implementations. ts/ is canonical; go/ and rs/ track it.
#
# Local build/test resolve the unpublished @tabnas siblings via the
# repo-set go.work + node_modules symlinks (admin/scripts/link.sh).

.PHONY: all build test clean build-ts build-go build-rs test-ts test-go \
        test-rs clean-ts clean-go clean-rs publish-ts publish-go version-rs \
        tags-go reset fetch prose prose-counts

all: build test

build: build-ts build-go build-rs

test: fetch test-ts test-go test-rs

# Fetch the third-party conformance corpora (rubys/feedvalidator and
# kurtmckee/feedparser) at their pinned commits. They are NEVER committed —
# see scripts/fetch-corpus.mjs and .gitignore. Idempotent: a corpus already
# at the pinned SHA is left alone.
#
# `make test` runs this FIRST, so the conformance suites can never silently
# not-run. ts/ also has it as an npm `pretest` (so a bare `npm test` in ts/
# is safe), and the Go suite auto-fetches on demand.
fetch:
	node scripts/fetch-corpus.mjs all

clean: clean-ts clean-go clean-rs

# --- TypeScript (package in ts/) ---
build-ts:
	cd ts && npm run build

test-ts:
	cd ts && npm test

clean-ts:
	rm -rf ts/dist ts/dist-test

# Publish the TypeScript package at its current package.json version.
publish-ts: test-ts
	cd ts && npm publish --access public

# --- Go (module in go/) ---
build-go:
	cd go && go build ./...

# -count=1 disables the Go test cache. The shared fixtures in test/spec/*.tsv,
# the corpus in test/feedparser-wellformed/ and the fetched conformance corpora
# all live ABOVE the Go module root, so Go does not track them as test inputs:
# editing a fixture would otherwise replay a cached "ok ... (cached)" without
# running the new rows.
test-go: fetch
	cd go && go test -count=1 -v ./...

clean-go:
	cd go && go clean

# Publish the Go module: make publish-go V=x.y.z
# Injects V into the Go `VERSION` const, commits, tags go/vX.Y.Z, and
# (when gh is available) creates a GitHub release. Note this does NOT touch
# ts/package.json: TestVersionMatchesPackageJSON will fail unless both are
# moved together (the release orchestrator rewrites both).
publish-go: test-go
	@test -n "$(V)" || (echo "Usage: make publish-go V=x.y.z" && exit 1)
	sed -i.bak 's/^const VERSION = ".*"/const VERSION = "$(V)"/' go/feed.go
	rm -f go/feed.go.bak
	git add go/feed.go
	git commit -m "go: v$(V)"
	git tag go/v$(V)
	git push origin main go/v$(V)
	@command -v gh >/dev/null 2>&1 && gh release create go/v$(V) --title "go/v$(V)" --notes "Go module release v$(V)" || true

# List published Go module tags, newest first.
tags-go:
	git tag -l 'go/v*' --sort=-version:refname

# --- Rust (crate in rs/) ---
#
# The crate takes the engine, the jsonic base grammar, the xml grammar,
# the fixture runner and the debug plugin as SIBLING CHECKOUTS by path;
# none is published. ci/rust/run.sh is the full gate and checks for them
# first. These targets are the fast inner loop.
build-rs:
	cd rs && cargo build --all-targets

# `--all-targets` does NOT include doctests -- cargo documents the
# selector as "Test all targets (does not include doctests)" -- so the
# README examples need their own run or a stale one ships.
test-rs:
	cd rs && cargo test --all-targets && cargo test --doc
	cd rs && cargo clippy --all-targets --all-features -- -D warnings

clean-rs:
	cd rs && cargo clean

# Set the Rust crate version: make version-rs V=x.y.z
#
# Bumps BOTH Rust version sites, plus the crate's own entry in
# rs/Cargo.lock, which ci/rust/run.sh compares against rs/Cargo.toml
# before it runs anything. It does NOT touch ts/package.json or
# go/feed.go: rs/tests/version_test.rs fails until all four agree, which
# is the point.
version-rs:
	@test -n "$(V)" || (echo "Usage: make version-rs V=x.y.z" && exit 1)
	sed -i.bak 's/^version = ".*"/version = "$(V)"/' rs/Cargo.toml
	sed -i.bak 's/^pub const VERSION: &str = ".*";/pub const VERSION: \&str = "$(V)";/' rs/src/lib.rs
	rm -f rs/Cargo.toml.bak rs/src/lib.rs.bak
	cd rs && cargo metadata --format-version 1 --offline >/dev/null

reset:
	cd ts && npm run reset
	cd go && go clean -cache && go build ./... && go test -v ./...
	cd rs && cargo clean && cargo test --all-targets && cargo test --doc

# The prose gate (see docs/STYLE-GUIDE.md). Vale over the reader-facing
# pages, at the levels set in .vale.ini, on the same file list
# ts/test/docs.test.js reads. Requires `vale` on PATH and one
# `vale sync`. Warnings are advisory, errors fail.
prose:
	vale --minAlertLevel=error $$(node ts/scripts/gated-docs.cjs)
	node ts/scripts/vale-counts.cjs

# Re-measure what .vale.ini and the style guide record, after
# a change to the pages or to the rules moves the numbers.
prose-counts:
	node ts/scripts/vale-counts.cjs --write
