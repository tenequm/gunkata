---
type: Finding
title: The agy ACP server replaces its token link with a file, and otherwise starts bare
description: Antigravity's ACP server keeps its whole profile under $HOME/.gemini, starts bare in an engine-owned HOME with three links, loads skills only from the Gemini home and the cwd itself (no ancestor walk), loaded no workspace rules at all - but its silent token refresh replaces the linked acp_token.json with a regular file, so the engine must delete it after the job.
tags: [executor, agy, antigravity, bare-start, isolation, credentials, skills]
status: stable
stale_after: "2027-03-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-18T23:55:00Z" }
sources:
  - id: live
    resource: "Live gunkata runs on this host, 2026-09-18: acpx 0.17.0, agy 1.2.5 ACP server (agy_acp_server.par), model gemini-3.7-flash-low, run dirs under ~/.local/state/gunkata/runs, with canary rule files and skills planted in the cwd and its ancestors by pre-steps"
    title: Live probe and canary runs, their trajectory DBs and server logs
  - id: src
    resource: "The ACP server's own Python source, extracted from agy_acp_server.par 1.2.5 (paths.py, oauth/credential_store.py, oauth/credential_manager.py, server.py)"
    title: agy ACP server source
  - id: acpx
    resource: https://github.com/openclaw/acpx/blob/main/agents/Antigravity.md
    title: acpx main's built-in antigravity agent (unreleased after 0.17.0, commit ad7006b)
---

# Finding

**Launch.** acpx 0.17.0 has no Antigravity agent - its built-in `gemini` is the Gemini CLI,
a different product. gunkata runs `acpx --agent agy-acp-server exec`, a host wrapper that
execs `$HOME/.local/lib/antigravity-acp/agy_acp_server.par --uid=`. acpx main adds a
built-in `antigravity` agent. It launches `agy_acp_server.par` from PATH, needs
`ANTIGRAVITY_HARNESS_PATH` set to `localharness_external`, and cancels fixed-choice user
questions even under `--approve-all`.[^acpx]

**Profile.** Everything lives under the Gemini home: `$GEMINI_HOME`, or `$HOME/.gemini` when
that is unset.[^src] With HOME moved to the job, three links are enough:
`.gemini/antigravity-acp/settings.json` (selects `oauth-personal`),
`.gemini/antigravity-acp/acp_token.json`, and `.local/lib/antigravity-acp` (the server plus
`localharness_external`). Off macOS the token is always the plain file, never a keychain.[^src]

**The token link does not survive.** The server's access token expires after about an hour.
On startup it refreshes silently and saves the result with a temp file plus `os.replace`
in the same directory.[^src] That replaces the symlink with a regular file inside the job
HOME. Every live run refreshed, because the real home's token was already stale.[^live]
Google keeps the refresh token as it is, so the real file still works and the engine throws
the copy away: it removes each linked credential, and any `<name>.*` temp sibling, after
the executor exits. Nothing is written back to the real home.

**Skills.** The server loads skills from `<gemini_home>/config/skills` and
`<gemini_home>/antigravity-cli/skills`, and from `<cwd>/.gemini/skills` and
`<cwd>/.agents/skills`.[^src] A canary skill in the cwd's `.agents/skills` loaded. The same
canary in the parent directory did not, and neither did the real
`/home/<user>/.agents/skills`, although both are ancestors of the run dir. There is no
ancestor walk.[^live]

**Rules.** `GEMINI.md`, `AGENTS.md` and `.agents|.agent|.gemini/rules/*.md` were planted in
the cwd, its parent and its grandparent. Some had `trigger: always_on` frontmatter and some
did not, and some runs had a git repo in the cwd. None of their text reached the
conversation trajectory, and none of it reached the model's own report.[^live] Plain rule
files logged `Invalid rule trigger: CORTEX_MEMORY_TRIGGER_UNSPECIFIED`. The real
`/home/<user>/AGENTS.md` did not load either. No planted file loaded, so this build could
not be shown to load workspace rules at all. The engine does not rely on that: the cwd is
the engine-owned work dir either way.

**MCP.** Servers passed on acpx stdin load as streamable-HTTP servers. The server merges
them with `<gemini_home>/config/mcp_config.json`, which a bare HOME does not have.[^live]

**Config options.** The only options are `model` and `mode` (`default`, `auto_edit`,
`yolo`). Effort is part of the model id, e.g. `gemini-3.7-flash-low`.[^live]

**Leftovers.** The server writes its glog logs and a CA bundle to `$TMPDIR` (the job's
`home/tmp`). The INFO log holds the raw tool traffic but no token material.[^live]
