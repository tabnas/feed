.PHONY: all build test clean reset build-ts build-go build-rs test-ts \
        test-go test-rs clean-ts clean-go clean-rs

all: build test

build: build-ts build-go build-rs

test: test-ts test-go test-rs

clean: clean-ts clean-go clean-rs

# TypeScript
build-ts:
	npm run build

test-ts:
	npm test

clean-ts:
	rm -rf dist dist-test

# Go
build-go:
	cd ../go && go build ./...

test-go:
	cd ../go && go test ./...

clean-go:
	cd ../go && go clean -cache

# Rust
build-rs:
	cd ../rs && cargo build --all-targets

test-rs:
	cd ../rs && cargo test --all-targets && cargo test --doc
	cd ../rs && cargo clippy --all-targets --all-features -- -D warnings

clean-rs:
	cd ../rs && cargo clean

reset:
	npm run reset
	cd ../go && go clean -cache
	cd ../go && go build ./...
	cd ../go && go test -v ./...
	cd ../rs && cargo clean && cargo test --all-targets && cargo test --doc
