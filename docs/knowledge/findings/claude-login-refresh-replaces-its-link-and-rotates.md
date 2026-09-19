---
type: Finding
title: A Claude login refresh replaces the linked credentials file and rotates the refresh token
description: Claude Code writes .credentials.json by temp file and rename, so an executor's OAuth refresh replaces its link with a file holding a new single-use refresh token and spends the host's; its refresh lock lives in the executor's own .claude, so jobs and the host never serialize - the engine must hand a newer login back to the host.
tags: [executor, claude, credentials, oauth, isolation]
status: stable
stale_after: "2027-03-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-19T00:30:00Z" }
sources:
  - id: bin
    resource: "Bundled JS in the Claude Code 2.1.257 linux-x64 binary that acpx 0.17.0 runs through claude-agent-acp 0.76.0 (~/.npm/_npx/*/node_modules/@anthropic-ai/claude-code-linux-x64/claude), read statically with rg -a and dd; no refresh was run"
    title: Claude Code 2.1.257 bundled source
  - id: issue
    resource: https://github.com/anthropics/claude-code/issues/54443
    title: "claude-code #54443: concurrent sessions forced to /login, refresh-token fingerprints rotating per refresh, invalid_grant on reuse"
  - id: host
    resource: "Non-secret fields (expiresAt, file mtime) of the host's ~/.claude/.credentials.json, 2026-09-19"
    title: Host login metadata
---

# Finding

**The write replaces the link.** The plaintext login store writes through a helper that
writes `<path>.tmp.<8 hex>` beside the target and renames it over `<path>`. It opens with
`O_NOFOLLOW` and never resolves the path first.[^bin] Under an engine-owned HOME, `<path>`
is the job's `.claude/.credentials.json` symlink. The rename puts a regular file in the
link's place, and the host's file keeps the old contents. Reads use `readFile`, which does
follow the link.[^bin]

**The refresh token rotates.** The refresh request stores the response's `refresh_token`.
It falls back to the posted token only when the response has none. The save is a
compare-and-set against the refresh token it posted. Another path revokes a returned
refresh token that differs from the posted one.[^bin] A 24-hour trace in the field showed a
new refresh-token fingerprint after every refresh. Reusing a spent token returns
`invalid_grant`.[^issue] On `invalid_grant`, Claude Code rewrites the file with
`refreshToken: ""`, `accessToken: ""` and `expiresAt: 0`. The same write also replaces
the link.[^bin]

**When it refreshes.** Claude Code refreshes within 5 minutes of `expiresAt`, and access
tokens last 8 hours.[^bin][^host] Any job that starts near the host's expiry, or runs
through it, refreshes. The refresh spends the refresh token the host still holds. Before
the fix, dropping the job's file after the run threw away the only live token, and the
host was logged out.

**No shared lock.** The refresh lock is `<config dir>/.oauth_refresh.lock`, plus a legacy
`<realpath(config dir)>.lock`. Both are keyed by the executor's own `$HOME/.claude`, which
is a real directory in the job HOME.[^bin] Parallel jobs and the host never take the same
lock. Two refreshes that land within the same few seconds can still race, and the loser
gets `invalid_grant`. Before it refreshes, and after a failed refresh, Claude Code
re-reads the file through the link. If the file holds a different access token, it adopts
that token ("race resolved") instead of posting.[^bin] So a token that reaches the host
file quickly also reaches sibling jobs.

**A server flag closes the link entirely.** When the server-served flag `tengu_hover_rest`
is on (`CLAUDE_CODE_HOVER_REST`), the v5 store opens the file with `O_NOFOLLOW` and treats
a symlink as a failed read. In that mode it never refreshes or writes through the link.
Executors run with `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` and an empty `.claude.json`,
so they stay on the plaintext path above.[^bin]

**What the engine does.** While a claude executor runs, `watchClaudeLogin` polls the job's
link once a second, and checks once more when the executor exits. When the link has
become a regular file, the engine compares the two `claudeAiOauth.expiresAt` values. If
the file's is later than the host's, the engine writes it over the host's file (after
resolving symlinks) with a temp file and rename. Then it restores the link with a staged
symlink and a rename. A dead-token clear has `expiresAt` 0, so it never overwrites the
host's file. `CLAUDE_CONFIG_DIR` pointed at the real `~/.claude` would keep writes on the
host's file, but it would end the bare start. A static `CLAUDE_CODE_OAUTH_TOKEN` cannot
refresh, so it would die mid-job at the 8-hour expiry.

[^bin]: Claude Code 2.1.257 bundled source
[^issue]: claude-code #54443
[^host]: Host login metadata
