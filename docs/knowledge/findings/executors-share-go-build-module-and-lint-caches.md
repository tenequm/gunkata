---
type: Finding
title: A bare HOME rebuilds Go's build, module and lint caches every job, and all three are safe to share
description: With a per-job HOME every executor that checks a Go repo refills GOCACHE, GOMODCACHE and GOLANGCI_LINT_CACHE from scratch; all three are built for concurrent use by processes on one machine, so the engine points them at shared dirs beside the npm cache, cutting a cuttle build-and-lint from 16.4 s to 4.6 s.
tags: [executor, go, golangci-lint, performance]
status: stable
stale_after: "2027-03-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-19T10:20:00Z" }
sources:
  - id: gocache
    resource: go 1.27.1 src/cmd/go/internal/cache/cache.go (Cache doc comment, lockedfile trim)
    title: Go build cache - safe for multiple processes on one machine
  - id: modfetch
    resource: go 1.27.1 src/cmd/go/internal/modfetch/cache.go and fetch.go (lockVersion, ModCacheRW)
    title: Go module cache - per-version lock files, read-only extraction
  - id: gcilcache
    resource: https://github.com/golangci/golangci-lint/blob/v2.13.2/internal/go/cache/readme.md
    title: golangci-lint 2.13 cache - cmd/go's cache package synced to go1.26.4, GOCACHE renamed GOLANGCI_LINT_CACHE
  - id: gcillock
    resource: https://github.com/golangci/golangci-lint/blob/v2.13.2/pkg/commands/run.go
    title: golangci-lint parallel-runner lock at os.TempDir()/golangci-lint.lock
  - id: forensics
    resource: "gunkata review-kata timing forensics on ws-pond-01, 2026-09-19: executor homes holding a fresh 441 MB go-build, 64 MB module and a golangci-lint cache; just check ~80 s cold vs 35 s warm"
    title: Review-kata timing forensics
  - id: live
    resource: "Live gunkata runs on ws-pond-01, 2026-09-19: claude-haiku-4-5 via acpx 0.17.0, go 1.27.1, golangci-lint 2.13.2; shallow clone of glim-sh/cuttle, go build ./... && golangci-lint run ./... in packages/cuttle, run twice against one empty shared cache root"
    title: Cold and warm shared-cache runs
---

# Finding

The engine gives each executor its own HOME and XDG dirs, so Go's defaults - `GOCACHE`
under `XDG_CACHE_HOME`, `GOMODCACHE` under `HOME/go`, golangci-lint's cache under
`XDG_CACHE_HOME` - put a fresh build cache, module cache and lint cache in every job.
Review-kata forensics found 441 MB of go-build, 64 MB of modules and a lint cache per
executor, and `just check` on the reviewed repo at ~80 s instead of 35 s warm.[^forensics]

All three caches are designed for concurrent use by processes on one machine:

- **GOCACHE** - cmd/go's cache states it is safe for multiple processes on a single
  machine; entries are content-addressed and trimming is serialized by a locked
  `trim.txt`.[^gocache]
- **GOMODCACHE** - each module version's download and extraction runs under a lock file
  in the cache (`lockVersion`), so concurrent `go` commands never extract the same version
  twice. Extracted trees are read-only unless `-modcacherw`.[^modfetch]
- **GOLANGCI_LINT_CACHE** - golangci-lint vendors cmd/go's cache package (synced to
  go1.26.4) with only the env var renamed, so it inherits the same guarantees.[^gcilcache]
  Its parallel-runner lock is a separate file, `os.TempDir()/golangci-lint.lock`, which
  resolves to the executor's own `HOME/tmp` through `TMPDIR` - and its private `/tmp` when
  the mount namespace applies - so jobs never contend on it.[^gcillock]

The engine therefore sets all three next to the shared npm cache, as one table of
variable to dir under `$XDG_CACHE_HOME/gunkata/` (`go-build`, `go-mod`, `golangci-lint`),
created before each executor starts. Pre- and post-steps are untouched: they run with the
engine's own environment and already use the host's caches. The read-only module files
are no cleanup hazard: they live outside the run dir, and the engine only ever removes
credential copies inside a job home.

Live, for `go build ./... && golangci-lint run ./...` on a shallow cuttle clone: 16.4 s
against empty shared caches, 4.6 s on the second job, with the job home at 9 MB. The
shared root then held 242 MB go-build, 64 MB go-mod, 17 MB golangci-lint.[^live]

The same trade as the [npm cache](/findings/acpx-runs-adapters-through-npm-exec.md): build
output and module source, never config, so executors stay bare; shared writable state,
but no new capability for a process already running as the engine's uid.

[^gocache]: Go build cache - safe for multiple processes on one machine
[^modfetch]: Go module cache - per-version lock files, read-only extraction
[^gcilcache]: golangci-lint 2.13 cache - cmd/go's cache package synced to go1.26.4, GOCACHE renamed GOLANGCI_LINT_CACHE
[^gcillock]: golangci-lint parallel-runner lock at os.TempDir()/golangci-lint.lock
[^forensics]: Review-kata timing forensics
[^live]: Cold and warm shared-cache runs
