---
type: Finding
title: Claude MCP OAuth tokens are keyed by server name plus a hash of its exact config
description: Claude Code stores MCP OAuth tokens in .credentials.json under "<name>|" + the first 16 hex of sha256 over {type,url,headers}, so a server passed on acpx stdin reuses a stored login only when name, exact URL and empty headers match - and linking the file hands the agent every stored token.
tags: [executor, claude, mcp, oauth, credentials, acpx]
status: stable
stale_after: "2026-12-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-18T12:00:00Z" }
sources:
  - id: live
    resource: "Live gunkata/acpx runs on this host, 2026-09-18: acpx 0.17.0, claude-agent-acp 0.76.0 bundling Claude Code 2.1.257, MCP servers passed on acpx stdin with .claude/.credentials.json linked"
    title: Live runs matching stdin MCP servers against stored logins
  - id: executor
    resource: /packages/gunkata/internal/engine/executor.go
    title: The Claude credential link
---

# Finding

Claude Code keeps MCP OAuth tokens in `.claude/.credentials.json` under `mcpOAuth`, keyed as:

```
"<name>|" + sha256(JSON.stringify({type, url, headers: headers || {}})).slice(0, 16)
```

A server the step passes on acpx stdin authenticates from the linked file only when all three
match the original login: the server name, the exact URL string, and empty headers. Any
difference - a trailing slash, a renamed server, an added header - computes a different key and
the server comes up unauthenticated.[^live] To reuse a login, declare the server exactly as it
was declared when the login was made.

# The exposure, accepted

The file is linked into the executor HOME whole, because it also carries the subscription
token the executor runs on.[^executor] So every executor can read every MCP OAuth token stored
there, not only the ones its step declared, plus the subscription token itself. The operator
accepted that exposure on 2026-09-18; a step that must not see the other tokens needs a
separately provisioned credentials file, not a filter on this one.
