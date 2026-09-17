---
type: Decision
title: The quality gate is nix, just and lefthook
description: Three tools own three non-overlapping jobs - provisioning, task running and index-aware hooks - because each candidate for collapsing them fails on a specific, verified capability gap.
tags: [tooling, quality-gate, nix, just, lefthook, moon]
status: stable
generated: { by: claude-code/opus-5, at: "2026-09-17T19:44:32Z" }
sources:
  - id: nix-manual
    resource: https://nix.dev/manual/nix/stable/store/building
    title: Nix manual, Building - the cleared environment and the sandbox
  - id: lefthook-src
    resource: "github.com/evilmartians/lefthook at v2.1.14 (1e23553): internal/config/available_hooks.go, internal/run/controller/job.go, internal/config/command.go"
    title: lefthook 2.1.14 source
  - id: moon-src
    resource: "moonrepo/moon 2.5.4: crates/vcs/src/git/tree.rs, crates/task-runner/src/command_builder.rs, crates/config/src/task_options_config.rs"
    title: moon 2.5.4 source and CLI, exercised in a throwaway workspace
  - id: pond
    resource: "~/pj/pond: .moon/workspace.yml, .moon/hooks/pre-commit, .github/hooks/pre-commit"
    title: The live moon reference implementation on this host
  - id: upstream-practice
    resource: "github.com/NixOS/nix (maintainers/format.sh, maintainers/flake-module.nix); github.com/evilmartians/lefthook (.lefthook.yml, Makefile)"
    title: How the two upstreams run their own gates
---

# Decision

The gate is built from three tools, and each owns one job the others cannot do:

- **nix** pins which tools exist. It provisions the dev shell and provides the one construction
  that is verify-only by kernel enforcement rather than by discipline.
- **just** decides what runs. It carries the two gate verbs, `[parallel]` dependency groups,
  bash with `pipefail`, recipe parameters, dependency dedup and `--list` discovery.
- **lefthook** owns `core.hooksPath` and contributes `stage_fixed`, which nothing else
  implements.

Collapsing the three was evaluated against primary docs and running binaries in
2026-09. Each candidate failed on a specific capability, not on taste.

# Why each collapse fails

**nix cannot replace just**, because a nix build cannot see the git index. Inside a check
derivation the environment is cleared, `HOME` is `/homeless-shelter` and the source is a
read-only store copy;[^nix-manual] measured there, `git diff --cached` returns
`error: unknown option 'cached'` and a write into the source is `Permission denied`. A flake
copies the *worktree* contents of *tracked* files, so a check cannot see staged blobs even for
tracked files, let alone write fixes back. Flake apps (`nix run`) are unsandboxed and can do
all of it, but then the parallelism is hand-written `&`/`wait` inside a bash string in
`flake.nix` with no `[parallel]`, no working-directory attribute and no arguments. The two
nix-native hook frameworks both wrap Python `pre-commit`, whose maintainers refuse to touch the
staging area as policy, so a formatter that fixes a file aborts the commit.

**lefthook cannot replace just**, because of one predicate:[^lefthook-src]

```go
func HookUsesStagedFiles(hook string) bool { return hook == "pre-commit" }
```

It gates `stage_fixed`, the empty-staged-set skip, unstaged-hunk hiding, and glob-gating of
jobs with no file template. In a custom group, `stage_fixed: true` is a silent no-op - verified
on 2.1.14, the fixed file stayed unstaged with no warning. So the fixing verb cannot be a named
task; it must literally be the `pre-commit` hook. Beyond that, lefthook hardcodes `sh -c`, has
no recipe parameters or defaults, no task listing, and no dependency graph, so expressing two
modes means duplicating the definitions.

**moon cannot replace both, yet.** moon is the strongest candidate and does more than expected:
`--affected --status staged` really does scope to index-side changes,[^moon-src] `vcs.hooks`
plus `sync: true` self-wires hooks with no install step, task `extends` expresses two modes from
one definition, and content-hash caching measured 1.24s cold against 0.24s warm - the one
capability neither just nor lefthook has at all. What disqualifies it here is that it has no
`stage_fixed`: the re-stage becomes a hand-written `git add` reading `MOON_AFFECTED_FILES`,
which is joined by the OS path delimiter, inside a `script:` task where `$@` is silently wrong
rather than empty because moon invokes `<shell> -c "<script>" <args...>` and the first affected
file lands in `$0`. Around that sit a `~/.proto/shims` PATH prepend ahead of the nix dev shell,
a task fingerprint carrying no OS or arch unless a toolchain plugin adds it, and CI caching that
upstream documents as non-functional without a self-hosted remote cache server.

The one live reference on this host argues the same way. `~/pj/pond` uses moon, but its
`.moon/hooks/pre-commit` is a generated stub whose only line calls a hand-written
`.github/hooks/pre-commit`, and that script's staged scoping is gitleaks' own `--staged` flag
plus `git diff --cached`.[^pond] moon installs the pointer; it does not drive the gate.

Both upstreams run the same shape we do. NixOS/nix declares its hooks once and keeps the fixer
as a shell script; evilmartians/lefthook delegates every job body to `make`, and its own CI
never invokes lefthook.[^upstream-practice]

# What would reopen this

Adopt moon when the gate crosses about a minute, when a second module or language lands, or
when a remote cache server exists to point at. Its caching is the real prize and the rest of the
cost is bearable once the gate is slow enough for caching to matter. Nothing would reopen the
nix or lefthook questions short of an upstream gaining index-aware fixing.
