set shell := ["bash", "-euo", "pipefail", "-c"]

binary := "gunkata"

# The Go module lives in packages/gunkata; every Go recipe declares it as its
# working directory. The knowledge-index recipes stay at the repo root.

[private]
default:
    @just --list --unsorted

# Code Quality

# Format all Go code
[group('quality')]
[working-directory('packages/gunkata')]
fmt:
    golangci-lint fmt ./...

# Check formatting without modifying (CI-safe)
[group('quality')]
[working-directory('packages/gunkata')]
fmt-check:
    golangci-lint fmt --diff ./...

# Run linter
[group('quality')]
[working-directory('packages/gunkata')]
lint:
    golangci-lint run ./...

# Run linter with auto-fix
[group('quality')]
[working-directory('packages/gunkata')]
lint-fix:
    golangci-lint run --fix ./...

# Verify the lint config against the installed binary
[group('quality')]
[working-directory('packages/gunkata')]
lint-config:
    golangci-lint config verify

# Run vulnerability check
[group('quality')]
[working-directory('packages/gunkata')]
vuln:
    govulncheck ./...

# Testing

# Run all tests with race detection
[group('test')]
[working-directory('packages/gunkata')]
test *args="./...":
    gotestsum --format testname -- -race {{ args }}

# Run tests with coverage
[group('test')]
[working-directory('packages/gunkata')]
test-cov:
    gotestsum --format testname -- -race -coverprofile=coverage.out -covermode=atomic ./...
    go tool cover -func=coverage.out

# Open coverage report in browser
[group('test')]
[working-directory('packages/gunkata')]
coverage: test-cov
    go tool cover -html=coverage.out

# Watch tests during development
[group('test')]
[working-directory('packages/gunkata')]
test-watch:
    gotestsum --watch --watch-clear --format testname

# Run benchmarks
[group('test')]
[working-directory('packages/gunkata')]
bench:
    go test -bench=. -benchmem ./...

# Run the starter corpus: both variants end to end, then graded (needs acpx)
[group('test')]
[working-directory('packages/gunkata')]
corpus: build
    #!/usr/bin/env bash
    set -euo pipefail
    pass_dir="$(./{{ binary }} run ../../examples/starter/pass.yaml)"
    echo "pass run: $pass_dir"
    ./{{ binary }} grade --variant pass "$pass_dir"
    fail_dir="$(./{{ binary }} run ../../examples/starter/fail.yaml)" && code=0 || code=$?
    echo "fail run: $fail_dir"
    if [[ "$code" -ne 2 ]]; then
        echo "fail variant exited $code, want 2 (parked)" >&2
        exit 1
    fi
    ./{{ binary }} grade --variant fail "$fail_dir"
    echo "corpus ok: pass succeeded, fail parked, both graded clean"

# Build

# Build the binary
[group('build')]
[working-directory('packages/gunkata')]
build:
    go build -o {{ binary }} ./cmd/{{ binary }}

# Build optimized release binary
[group('build')]
[working-directory('packages/gunkata')]
build-release:
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o {{ binary }} ./cmd/{{ binary }}

# Dependencies

# Tidy and verify modules
[group('deps')]
[working-directory('packages/gunkata')]
tidy:
    go mod tidy
    go mod verify

# Run code generators
[group('deps')]
[working-directory('packages/gunkata')]
generate:
    go generate ./...

# Knowledge Base

# Regenerate the docs/knowledge index
[group('docs')]
kb-index:
    python3 scripts/kb_index.py

# Fail if the docs/knowledge index is stale
[group('docs')]
kb-check:
    python3 scripts/kb_index.py --check

# CI

# Full gate (format check + lint config + lint + test + knowledge index)
[group('ci')]
check: fmt-check lint-config lint test kb-check
    @echo "All checks passed"

# Clean build artifacts
[group('ci')]
[working-directory('packages/gunkata')]
clean:
    go clean
    rm -f {{ binary }} coverage.out
