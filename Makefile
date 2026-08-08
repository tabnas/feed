# Build, test and publish both the TypeScript (ts/) and Go (go/)
# implementations. ts/ is canonical; go/ tracks it.
#
# Local build/test resolve the unpublished @tabnas siblings via the
# repo-set go.work + node_modules symlinks (admin/scripts/link.sh).

.PHONY: all build test clean build-ts build-go test-ts test-go \
        clean-ts clean-go publish-ts publish-go tags-go reset fetch

all: build test

build: build-ts build-go

test: fetch test-ts test-go

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

clean: clean-ts clean-go

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

reset:
	cd ts && npm run reset
	cd go && go clean -cache && go build ./... && go test -v ./...
