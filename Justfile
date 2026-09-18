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

# golangci-lint takes a global lock in the system temp dir and exits with
# "parallel golangci-lint is running" rather than queueing, so `fmt` and `run`
# are paired inside one recipe instead of being concurrent siblings of a gate;
# with fixes on they would also write the same files.

# Verify formatting, lint config and lint, whole repo (one golangci-lint at a time)
[group('quality')]
fmt-lint: fmt-check lint-config lint

# Format and lint the staged Go packages, applying fixes
[group('quality')]
fmt-lint-staged:
    #!/usr/bin/env bash
    set -euo pipefail
    mapfile -t files < <(just staged-go)
    if [[ "${#files[@]}" -eq 0 ]]; then
        echo "fmt-lint-staged: nothing staged"
        exit 0
    fi
    # `golangci-lint run` cannot take a bare file list: a list spanning two
    # directories is rejected outright, and one file of a multi-file package
    # reports phantom `undefined:` typecheck errors. Lint the packages instead.
    mapfile -t pkgs < <(printf '%s\n' "${files[@]}" | xargs -n1 dirname | sort -u)
    echo "fmt-lint-staged: ${pkgs[*]}"
    # `golangci-lint run --fix` is package-scoped, so it can rewrite a file that
    # shares a package with a staged one but is not staged itself. lefthook's
    # stage_fixed re-stages only the set it was handed, so such a fix would sit
    # unstaged while the commit went through without it. Compare the unstaged
    # set across the fixers and refuse to pass silently.
    before="$(git diff --name-only | sort -u)"
    cd packages/gunkata
    golangci-lint fmt "${files[@]}"
    golangci-lint run --fix "${pkgs[@]}"
    cd - >/dev/null
    staged="$(printf 'packages/gunkata/%s\n' "${files[@]}" | sort -u)"
    stray="$(comm -13 <(printf '%s\n' "$before") <(git diff --name-only | sort -u) \
        | comm -23 - <(printf '%s\n' "$staged"))"
    if [[ -n "$stray" ]]; then
        echo "fmt-lint-staged: the fixer rewrote files that are not staged:" >&2
        printf '%s\n' "$stray" | sed 's/^/  /' >&2
        echo "fmt-lint-staged: review and 'git add' them, then commit again" >&2
        exit 1
    fi

# Run vulnerability check
[group('quality')]
[working-directory('packages/gunkata')]
vuln:
    govulncheck ./...

# Scan the working tree for secrets
[group('quality')]
secrets:
    gitleaks dir . --redact=100 --no-banner

# Scan the staged diff for secrets
[group('quality')]
secrets-staged:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ -z "$(git diff --cached --name-only --diff-filter=ACMR)" ]]; then
        echo "secrets-staged: nothing staged"
        exit 0
    fi
    gitleaks git --pre-commit --staged --redact=100 --no-banner

# Lint the GitHub Actions workflows (shellchecks every run: block)
[group('quality')]
actions:
    actionlint .github/workflows/*.yml

# Evaluate the flake and build its packages
[group('quality')]
flake:
    nix flake check

# Staged Go files, relative to the module dir the Go tools must run in
[private]
staged-go:
    @git diff --cached --name-only --diff-filter=ACMR | grep '^packages/gunkata/.*\.go$' | sed 's|^packages/gunkata/||' || true

# Testing

# Run all tests with race detection
[group('test')]
[working-directory('packages/gunkata')]
test *args="./...":
    gotestsum --format testname -- -race {{ args }}

# Run the tests of the staged Go packages (go test takes packages, not files)
[group('test')]
test-staged:
    #!/usr/bin/env bash
    set -euo pipefail
    mapfile -t files < <(just staged-go)
    if [[ "${#files[@]}" -eq 0 ]]; then
        echo "test-staged: nothing staged"
        exit 0
    fi
    mapfile -t pkgs < <(printf '%s\n' "${files[@]}" | xargs -n1 dirname | sort -u | sed 's|^|./|')
    just test "${pkgs[@]}"

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

# Fail if go.mod or go.sum is untidy, without rewriting them
[group('deps')]
[working-directory('packages/gunkata')]
tidy-check:
    go mod tidy -diff
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

# Gates

# Both verbs run their gates concurrently. `check` is staged-only and applies
# fixes; `check-ci` is whole-repo and mutates nothing, so the pre-push hook and
# CI run the identical command and cannot drift.

# Report any gate tool missing from the shell
[group('ci')]
tools:
    #!/usr/bin/env bash
    set -euo pipefail
    missing=()
    for tool in actionlint gitleaks go golangci-lint gotestsum govulncheck nix python3; do
        command -v "$tool" >/dev/null || missing+=("$tool")
    done
    if [[ "${#missing[@]}" -gt 0 ]]; then
        echo "tools: not on PATH: ${missing[*]}" >&2
        # A gate run from a shell whose dev env predates a flake.nix change
        # fails with a bare exit 127; this says what to do about it.
        echo "tools: dev shell stale or absent - run 'direnv reload', or prefix with 'nix develop -c'" >&2
        exit 1
    fi

# Fast staged-only gate, applies fixes (pre-commit)
[group('ci')]
[parallel]
check: tools fmt-lint-staged test-staged tidy secrets-staged kb-index
    @echo "check: passed"

# Full verify-only gate, mutates nothing (pre-push and CI)
[group('ci')]
[parallel]
check-ci: tools fmt-lint test tidy-check secrets vuln actions flake kb-check
    @echo "check-ci: passed"

# Clean build artifacts
[group('ci')]
[working-directory('packages/gunkata')]
clean:
    go clean
    rm -f {{ binary }} coverage.out
