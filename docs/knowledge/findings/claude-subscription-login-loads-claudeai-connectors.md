---
type: Finding
title: A Claude subscription login loads the account's claude.ai connectors into a bare executor
description: A Claude executor authenticated with the subscription OAuth login auto-loads every claude.ai connector on the account (Gmail, Slack, Drive, Calendar, glim...); ENABLE_CLAUDEAI_MCP_SERVERS=false stops it, and an allowedMcpServers list keyed by serverUrl is the selective alternative.
tags: [executor, claude, mcp, connectors, bare-start, acpx]
status: stable
stale_after: "2026-12-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-18T12:00:00Z" }
sources:
  - id: live
    resource: "Live gunkata/acpx runs on this host, 2026-09-18: acpx 0.17.0, claude-agent-acp 0.76.0 bundling Claude Code 2.1.257, executor HOME with only .claude/.credentials.json linked"
    title: Live runs that listed the executor's MCP servers with and without the switch
  - id: executor
    resource: /packages/gunkata/internal/engine/executor.go
    title: The per-harness executor environment
---

# Finding

The executor contract says a Claude executor starts bare and inherits only credentials. The
credential itself breaks that: with the subscription OAuth login linked in, Claude Code fetches
the account's claude.ai connectors and loads them as MCP servers - Gmail, Slack, Google Drive,
Google Calendar, glim and whatever else the account has connected - into an executor whose
HOME declares none of them.[^live] Nothing on disk in the executor HOME names them, so reading
the HOME does not reveal the leak; only listing the servers the running agent sees does.

# The switch

`ENABLE_CLAUDEAI_MCP_SERVERS=false` in the executor environment stops it. With it set, the only
MCP servers the agent sees are the ones the step declared and passed on acpx stdin.[^live] The
engine sets it for the Claude harness.[^executor]

# The selective alternative

When a step genuinely needs one connector, leave the switch on and put `allowedMcpServers` in
the executor HOME's `.claude/settings.json`. Two conditions, both verified:[^live]

- acpx only reads user-scope settings when `ACPX_CLAUDE_INCLUDE_USER_SETTINGS=1` is set (see
  [acpx loads Claude with project and local settings only](/findings/acpx-claude-skips-user-settings-by-default.md)).
- A connector must be named by `serverUrl`, not by its display name: allowlist names accept
  only `[A-Za-z0-9_-]`, and connector names such as `claude.ai Gmail` do not fit that set.
