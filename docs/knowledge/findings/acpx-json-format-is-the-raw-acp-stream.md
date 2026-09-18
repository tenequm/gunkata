---
type: Finding
title: acpx --format json emits the raw ACP JSON-RPC stream
description: With --format json acpx writes the ACP JSON-RPC traffic to stdout one object per line - session/update notifications for tool calls, usage and thought chunks, the client requests, and their responses, with stopReason and token usage on the prompt response - and --json-strict keeps non-JSON off it.
tags: [acpx, acp, executor, observability, json]
status: stable
stale_after: "2026-12-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-18T12:00:00Z" }
sources:
  - id: live
    resource: "Live acpx runs on this host, 2026-09-18: acpx 0.17.0 with claude-agent-acp 0.76.0 bundling Claude Code 2.1.257, stdout captured with --format json --json-strict"
    title: Captured stdout of live runs
---

# Finding

`acpx --format json` does not summarize: stdout is the raw ACP JSON-RPC stream, one JSON object
per line.[^live] Three kinds of line appear:

- **Notifications** - `session/update`, carrying among others:
  - `tool_call` and `tool_call_update`, with `toolCallId`, `title`, `kind` and `status`;
  - `usage_update`, which includes the running cost;
  - `agent_thought_chunk`, one per token - by far the highest-volume line type.
- **Requests** - `initialize`, `session/new`, `session/prompt` and
  `session/request_permission`.
- **Responses** to those requests. The `session/prompt` response carries `stopReason` and the
  turn's token usage.

`--json-strict` keeps non-JSON output (banners, diagnostics) off stdout, so every line parses.

# Consequence

Tool activity, permission requests, cost and the stop reason of a step can be read from the
captured stream line by line without scraping human-readable output. Anything consuming it
should filter `agent_thought_chunk` early, since per-token lines dominate the volume.
