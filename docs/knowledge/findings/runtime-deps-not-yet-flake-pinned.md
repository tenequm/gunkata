---
type: Finding
title: acpx is not flake-pinned; executor CLIs never will be
description: pond is packaged in the flake from its release binaries, acpx is host-provided pending a pnpm packaging route, and the executor CLIs are host-provided by necessity.
tags: [nix, toolchain, open-question]
status: stable
stale_after: "2026-12-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-17T02:02:39Z" }
sources:
  - id: flake
    resource: /flake.nix
    title: gunkata dev shell and pond package
  - id: acpx-pkg
    resource: https://github.com/openclaw/acpx/blob/main/package.json
    title: acpx package manifest (pnpm workspace, six runtime dependencies)
  - id: probe
    resource: "Packaging probe on this host, 2026-09-17: nixpkgs pin b1b8759, pond v0.17.3 release assets, acpx v0.16.0 manifest"
    title: Day 0 packaging probe
---

# Finding

Of the runtime dependencies gunkata needs, only one is reproducibly pinned today.

**pond is pinned.** The flake packages it from the upstream release tarballs for
`x86_64-linux`, `aarch64-linux` and `aarch64-darwin`, patchelf'd on Linux, with shell
completions installed. It builds and runs; `pond --version` in the dev shell reports
0.17.3.[^flake]

**acpx is host-provided.** It is an npm package built with a pnpm workspace, and its six
runtime dependencies are not bundled into the published `dist/`.[^acpx-pkg] That rules out the
two cheap routes - `buildNpmPackage` needs an npm lockfile, and a bare node wrapper around
the published tarball would fail on the first unresolved import. The remaining clean route is
`pnpm.fetchDeps`, which needs a dependency hash obtained by a deliberate build failure and a
build of the TypeScript bundler. That was outside the day 0 timebox, so the dev shell checks
for `acpx` on PATH and prints its version instead, warning below 0.15.0.[^probe]

**The executor CLIs are host-provided permanently.** This is not a gap to close. An
executor's ACP server is installed by the executor's own vendor - Antigravity installs its
own, and the others ship theirs with their own installers and auth flows. Fetching them in a
flake would fight the vendor's update path and could not carry the subscription auth that
lock's executor contract says is the sole inheritance. The dev shell only checks presence.

# Open question

Is `pnpm.fetchDeps` worth spending on for acpx, given that acpx is locked as the core driver
and an unpinned core driver means a run's behaviour can change under the engine without the
flake moving? Two things would change the answer: acpx publishing a self-contained bundle or
prebuilt binaries, or gunkata reaching the point where a driver version skew has actually
caused a failure. Until one of those happens, the version check in the dev shell is the
mitigation, and this concept is the record that it is a mitigation and not a design.

# Adjacent note

The nixpkgs pin resolves `go_1_27` to 1.27.1 and `golangci-lint` to 2.13.2, which satisfies
the version floor the Go conventions require (the linter must be built with a Go at least as
new as the project's). A pin that moves golangci-lint below 2.13 while Go stays at 1.27 would
break the lint gate outright, so the two are bumped together or not at all.
