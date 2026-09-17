set shell := ["bash", "-euo", "pipefail", "-c"]

binary := "gunkata"

[private]
default:
    @just --list --unsorted

# Code Quality

# Format all Go code
[group('quality')]
fmt:
    golangci-lint fmt ./...

# Check formatting without modifying (CI-safe)
[group('quality')]
fmt-check:
    golangci-lint fmt --diff ./...

# Run linter
[group('quality')]
lint:
    golangci-lint run ./...

# Run linter with auto-fix
[group('quality')]
lint-fix:
    golangci-lint run --fix ./...

# Verify the lint config against the installed binary
[group('quality')]
lint-config:
    golangci-lint config verify

# Run vulnerability check
[group('quality')]
vuln:
    govulncheck ./...

# Testing

# Run all tests with race detection
[group('test')]
test *args="./...":
    gotestsum --format testname -- -race {{ args }}

# Run tests with coverage
[group('test')]
test-cov:
    gotestsum --format testname -- -race -coverprofile=coverage.out -covermode=atomic ./...
    go tool cover -func=coverage.out

# Open coverage report in browser
[group('test')]
coverage: test-cov
    go tool cover -html=coverage.out

# Watch tests during development
[group('test')]
test-watch:
    gotestsum --watch --watch-clear --format testname

# Run benchmarks
[group('test')]
bench:
    go test -bench=. -benchmem ./...

# Build

# Build the binary
[group('build')]
build:
    go build -o {{ binary }} ./cmd/{{ binary }}

# Build optimized release binary
[group('build')]
build-release:
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o {{ binary }} ./cmd/{{ binary }}

# Dependencies

# Tidy and verify modules
[group('deps')]
tidy:
    go mod tidy
    go mod verify

# Run code generators
[group('deps')]
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
clean:
    go clean
    rm -f {{ binary }} coverage.out
