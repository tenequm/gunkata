---
type: Finding
title: A gate must run inside the pinned dev shell
description: A hook inherits the invoking shell, so an environment cached before a flake change fails the gate with a bare exit 127; the pre-push hook therefore wraps itself in nix develop.
tags: [quality-gate, nix, direnv, lefthook, hooks]
status: stable
stale_after: "2026-12-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-17T19:44:32Z" }
sources:
  - id: incident
    resource: "gunkata, 2026-09-17: `git push` of the one-gate merge, blocked by the pre-push hook"
    title: The push that exposed it
  - id: config
    resource: /lefthook.yml
    title: The pre-push job, wrapped in nix develop
  - id: tools
    resource: /Justfile
    title: The tools recipe
---

# Finding

A git hook runs as a child of whatever shell invoked git, so it inherits that shell's `PATH`
and nothing more. In a repo whose toolchain comes from a nix dev shell via direnv, that makes
the gate's result a function of when the invoking shell was last loaded.

This was not theoretical. Merging the two-verb gate to main added `actionlint` and `gitleaks`
to `flake.nix`, and the push was blocked by the pre-push hook:[^incident]

```
error: recipe `actions` failed on line 97 with exit code 127
exit status 127
```

`direnv status` showed the loaded environment watching `flake.nix` at `01:59:30Z` - before the
commit that added the tool. The gate was correct, the tool was genuinely absent, and the
diagnosis took a detour because exit 127 says only "command not found".

# The fix, in two parts

The pre-push job wraps itself: `nix develop -c just check-ci`.[^config] The gate then cannot be
decided by the caller's shell state, and it becomes byte-identical to the CI step, which is a
stronger form of the no-drift property than sharing a verb. Measured cost of the wrapper on this
host is about 2.3s, against a gate of roughly 3.5s. Verified while adopting it: neither
`nix develop` nor `nix flake check` writes `flake.lock` when the lock is current and committed,
so the wrapper does not mutate the tree it is gating.

Pre-commit is deliberately *not* wrapped. It runs in about 0.15s when nothing Go is staged, and
2.3s of dev-shell startup on every commit is the wrong trade for the interactive path. Instead
both gates depend on a `tools` recipe that names what is missing and what to do:[^tools]

```
tools: not on PATH: actionlint
tools: dev shell stale or absent - run 'direnv reload', or prefix with 'nix develop -c'
```

# The limit worth knowing

`tools` checks presence, not identity. During the incident `gitleaks` was on `PATH` from
`~/.nix-profile` rather than from the flake, so a presence check would have passed it. A gate
that must pin versions has to compare versions; this one only converts an obscure 127 into an
actionable sentence, and the `nix develop` wrapper is what actually guarantees the toolchain on
the path that matters most.
