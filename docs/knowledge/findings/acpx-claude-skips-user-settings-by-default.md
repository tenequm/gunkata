---
type: Finding
title: acpx loads Claude with project and local settings only
description: acpx launches Claude Code with --setting-sources=project,local, so skills and settings placed in the executor HOME's .claude (user scope) are ignored unless ACPX_CLAUDE_INCLUDE_USER_SETTINGS=1 is set.
tags: [executor, claude, acpx, skills, settings]
status: stable
stale_after: "2026-12-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-18T12:00:00Z" }
sources:
  - id: live
    resource: "Live gunkata/acpx runs on this host, 2026-09-18: acpx 0.17.0, claude-agent-acp 0.76.0 bundling Claude Code 2.1.257"
    title: Live runs comparing skill loading with and without the variable
---

# Finding

acpx starts Claude Code with `--setting-sources=project,local`. User scope - the executor
HOME's `.claude/` - is therefore not read: skills copied into `$HOME/.claude/skills` and a
`$HOME/.claude/settings.json` have no effect. Setting `ACPX_CLAUDE_INCLUDE_USER_SETTINGS=1` in
the executor environment adds user scope back.[^live]

# Why an earlier smoke run looked fine

An earlier smoke run appeared to load skills copied into the executor HOME without the
variable. It did not: the skills it saw came from the operator's real `~/.claude/skills`
through Claude Code's ancestor walk of the cwd, recorded in
[A Claude executor inherits CLAUDE.md files and skills from its cwd's ancestors](/findings/claude-walks-ancestor-dirs-for-instructions-and-skills.md).
With the leak removed, the copied skills vanish until the variable is set.[^live]

The same variable gates the `allowedMcpServers` route in
[A Claude subscription login loads the account's claude.ai connectors](/findings/claude-subscription-login-loads-claudeai-connectors.md).
