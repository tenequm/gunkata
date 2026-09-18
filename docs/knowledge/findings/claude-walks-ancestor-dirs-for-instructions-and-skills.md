---
type: Finding
title: A Claude executor inherits CLAUDE.md files and skills from its cwd's ancestors
description: Claude Code walks the ancestors of its cwd for CLAUDE.md and .claude/skills; the skill walk stops at $HOME or a git root, so a work dir nested inside the executor's own HOME is bounded, and CLAUDE_CODE_DISABLE_CLAUDE_MDS=1 stops the unbounded CLAUDE.md walk.
tags: [executor, claude, bare-start, isolation, skills, claude-md]
status: stable
stale_after: "2026-12-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-18T12:00:00Z" }
sources:
  - id: live
    resource: "Live gunkata/acpx runs on this host, 2026-09-18: acpx 0.17.0, claude-agent-acp 0.76.0 bundling Claude Code 2.1.257, run dirs under ~/.local/state/gunkata/runs"
    title: Live runs whose executors reported the loaded instruction files and skills
  - id: bundle
    resource: "Claude Code 2.1.257 bundle, getProjectDirsUpToHome; probes with --setting-sources project,local on 2.1.257 and 2.1.277"
    title: The skill walk's stop conditions
  - id: runsroot
    resource: /packages/gunkata/cmd/gunkata/main.go
    title: The default runs root under the XDG state dir
---

# Finding

Overriding `HOME` does not make a Claude executor bare. Claude Code also walks every ancestor
of its working directory looking for `CLAUDE.md` and `.claude/skills`. gunkata's runs root
defaults to `~/.local/state/gunkata/runs`,[^runsroot] which sits inside the operator's real
`$HOME`, so the walk reaches `/home/<user>`. Executors there loaded `/home/<user>/CLAUDE.md`,
`~/.claude/CLAUDE.md` and every skill in `~/.claude/skills` - the operator's whole agent
environment, in a run meant to carry none of it.[^live]

# What stops what

- `CLAUDE_CODE_DISABLE_CLAUDE_MDS=1` in the executor env stops the `CLAUDE.md` files.[^live]
- No switch stops ancestor skill discovery (`skillOverrides` is per name only; `--safe-mode`
  also drops the kata's own skills and MCP; `--bare` breaks subscription auth). But the skill
  walk stops at `os.homedir()` - `$HOME` itself is not scanned - at a git root, or at `/`.[^bundle]
  gunkata had the executor's HOME beside its cwd (`jobs/<job>/home` and `jobs/<job>/work`), so
  the walk ran past it into the real home.

# Resolution

The job's working directory is now `jobs/<job>/home/work`, inside the executor's HOME, so the
skill walk stops at that HOME wherever the run dir sits - the runs root stays in the XDG state
dir. A live run from `~/.local/state` confirmed only the declared skill loaded.[^live] The
`CLAUDE.md` walk is not HOME-bounded, so `CLAUDE_CODE_DISABLE_CLAUDE_MDS=1` stays set.

A related trap this leak masked is recorded in
[acpx loads Claude with project and local settings only](/findings/acpx-claude-skips-user-settings-by-default.md).
