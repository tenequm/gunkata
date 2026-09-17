---
type: Finding
title: lefthook guarantees less about the index than it appears
description: stage_fixed re-stages the list it was given rather than what changed, only partially staged files are hidden, the backup stash is shared across linked worktrees, and LEFTHOOK=0 turns any run into a silent pass.
tags: [lefthook, hooks, git, worktrees, quality-gate]
status: stable
stale_after: "2026-12-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-17T19:44:32Z" }
sources:
  - id: src
    resource: "github.com/evilmartians/lefthook at v2.1.14 (1e23553): internal/run/controller/job.go, internal/run/controller/guard.go, internal/git/repo.go, internal/config/command.go, internal/templates/hook.tmpl"
    title: lefthook 2.1.14 source
  - id: probes
    resource: "gunkata, 2026-09-17: live hook probes on 2.1.14, including commit 751ae4b (since reset)"
    title: Behaviour executed against the installed binary
  - id: upstream-issues
    resource: https://github.com/evilmartians/lefthook/issues/1529
    title: Concurrent commits in linked worktrees destroy each other's unstaged changes
---

# Finding

lefthook's index handling is the reason it stays in this repo's gate, and it is narrower than
its documentation suggests. Four limits are worth carrying, all verified against 2.1.14.[^src]

**`stage_fixed` re-stages the list it was given, not the files that changed.** The job's
substituted file list is what gets `git add`-ed; with no file template it falls back to the
staged set filtered by the job's `glob`, `exclude`, `root` and `file_types`. So a file the
command *created*, or fixed but which was not in that list, is not staged. For a package-scoped
Go linter this is a real gap: `golangci-lint run --fix` may fix an unstaged file in the same
package, and that fix lands in the worktree while the commit proceeds without it. The `git add`
is deferred and batched once at hook end, and since 2.1.12 a failed `git add` fails the hook.

`fmt-lint-staged` closes this by comparing the unstaged set across the fixers and failing on any
file the fixer wrote that is neither staged nor already dirty. The limit of that guard is that
`git diff` does not report untracked files, so a fixer that *creates* a file still passes it.
In practice the trigger is narrow, because it needs HEAD to already carry a fixable issue -
which this gate now prevents from landing.

**Only partially staged files are hidden.** The guard stashes a file only when it is dirty in
*both* the index and the worktree. A file with unstaged changes that was never staged is not
hidden, so a hook judges the on-disk file, not the indexed one - verified by editing the
`Justfile` without staging it and watching the hook execute the new recipe body.[^probes] The
practical reading: a hook always runs the working tree's gate definition.

**The backup is shared across linked worktrees.** The partial-stage backup patch
(`.git/info/lefthook-unstaged.patch`) and the `lefthook auto backup` stash live in the common
git dir, so two linked worktrees committing concurrently can destroy each other's unstaged
changes.[^upstream-issues] A related open issue has a failed patch re-apply fall back to a
literal `git checkout .`. Both are unfixed in 2.1.14 and there is no config knob to disable the
stash dance, so the only mitigation is operational: do not let two worktrees of one repo commit
at the same moment. This matters here because worktrees come from a shared pool.

**`LEFTHOOK=0` is a silent pass**, both for `lefthook run` and in the generated
`.git/hooks/<hook>` script, which exits 0 before doing anything. Anything inheriting that
variable goes green without running a check, which is why CI invokes `just check-ci` directly
and never reaches a gate through lefthook.

# Two configuration traps

`assert_lefthook_installed` is not a runtime guard. It is a template argument baked into the
generated hook script at `lefthook install` time, so changing it without reinstalling changes
nothing; what it produces is the abort branch in `.git/hooks/*` that fires when no lefthook
binary can be found.

`commands:` is a map and lefthook sorts it - by `priority` (where 0 sorts last), then a numeric
name prefix, then alphabetically - so a `commands` block with `piped: true` does not run in
written order. An earlier version of this repo's config was running `fmt`, `kb-index`, `lint`,
`mod-tidy` while reading as `fmt`, `lint`, `mod-tidy`, `kb-index`. `jobs:` is a list, preserves
declaration order, and has no priority field at all; this repo uses `jobs`.
