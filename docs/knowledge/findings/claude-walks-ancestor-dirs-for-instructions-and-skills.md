---
type: Finding
title: A Claude executor inherits CLAUDE.md files and skills from its cwd's ancestors
description: Claude Code walks the ancestors of its cwd for CLAUDE.md and .claude/skills, so run dirs inside the real $HOME leak the operator's instructions and skills into every executor; CLAUDE_CODE_DISABLE_CLAUDE_MDS=1 stops the md files, but nothing found stops the skills short of moving the cwd out of $HOME.
tags: [executor, claude, bare-start, isolation, skills, claude-md]
status: stable
stale_after: "2026-12-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-18T12:00:00Z" }
sources:
  - id: live
    resource: "Live gunkata/acpx runs on this host, 2026-09-18: acpx 0.17.0, claude-agent-acp 0.76.0 bundling Claude Code 2.1.257, run dirs under ~/.local/state/gunkata/runs"
    title: Live runs whose executors reported the loaded instruction files and skills
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
- No environment switch was found that stops ancestor skill discovery. The only fix is
  placement: the executor's cwd must live outside the real `$HOME`.

# Open

Where executor cwds should live instead is not decided as of 2026-09-18. Until it is, any run
whose run dir is under the real `$HOME` should be assumed to carry the operator's skills.

A related trap this leak masked is recorded in
[acpx loads Claude with project and local settings only](/findings/acpx-claude-skips-user-settings-by-default.md).
