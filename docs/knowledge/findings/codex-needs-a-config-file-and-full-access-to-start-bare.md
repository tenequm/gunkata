---
type: Finding
title: A Codex executor needs its own config.toml, full-access mode and the NixOS env marker to start bare
description: Under acpx, Codex with only auth.json in its HOME still loads subscription connectors and plugins, bundled system skills, a cwd AGENTS.md, repo-walk skills and host-keyring MCP tokens, hides late MCP servers, sandboxes the network off and rebuilds PATH through NixOS zsh init; a per-job config.toml, INITIAL_AGENT_MODE=agent-full-access and inheriting __NIXOS_SET_ENVIRONMENT_DONE fix all of it.
tags: [executor, codex, bare-start, isolation, skills, mcp, nixos]
status: stable
stale_after: "2026-12-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-18T23:59:00Z" }
sources:
  - id: live
    resource: "Live gunkata runs on this host, 2026-09-18: acpx 0.17.0, codex-acp 1.12.0 bundling @openai/codex 0.154.0, model gpt-5.6-luna; runs under ~/.local/state/gunkata/runs and under a runs root inside a directory with a .git marker"
    title: Live runs whose executors reported their skills, MCP tools and instructions, called gh and a DeepWiki tool
  - id: roots
    resource: https://github.com/openai/codex/blob/rust-v0.154.0/codex-rs/ext/skills/src/host_roots.rs
    title: Codex skill roots - user, system cache, and the .agents/skills walk from cwd to the project root
  - id: markers
    resource: https://github.com/openai/codex/blob/rust-v0.154.0/codex-rs/config/src/project_root_markers.rs
    title: project_root_markers - an empty list disables project root detection
  - id: bundled
    resource: https://github.com/openai/codex/blob/rust-v0.154.0/codex-rs/config/src/skills_config.rs
    title: "[skills.bundled] enabled - the general switch for the .system skills"
  - id: storage
    resource: https://github.com/openai/codex/blob/rust-v0.154.0/codex-rs/login/src/auth/storage.rs
    title: File auth storage saves by opening auth.json with truncate, following a symlink
  - id: modes
    resource: "@agentclientprotocol/codex-acp 1.12.0 dist/index.js, class AgentMode"
    title: The adapter's modes and INITIAL_AGENT_MODE
  - id: engine
    resource: /packages/gunkata/internal/engine/executor.go
    title: codexConfig, harnessEnv and inherited
---

# Finding

A Codex executor whose HOME holds only `.codex/auth.json` is not bare. Each leak and fix
below was confirmed by a live run,[^live] with the mechanism read from source where noted.
The engine writes the config to `HOME/.codex/config.toml` for every Codex job.[^engine]

- **Subscription connectors and plugins.** `[features] apps = false`, `plugins = false`,
  `remote_plugin = false`. With them off, the plugin-install tool disappears.
- **Bundled system skills.** Codex installs imagegen, openai-docs, plugin-creator,
  skill-creator, skill-installer and others into `.codex/skills/.system`.
  `[skills.bundled] enabled = false` stops them all, and the directory is never
  created.[^bundled] It is the general switch, so per-skill `[[skills.config]]` entries are
  not needed.
- **Skills from outside HOME.** Codex loads `HOME/.agents/skills` and `$CODEX_HOME/skills`
  as user skills. It also walks from the cwd up to the nearest `.git` and loads every
  `.agents/skills` on the way.[^roots] With no `.git` above the cwd, the walk stops at the
  cwd, so the default runs root does not leak `~/.agents/skills`. A runs root inside a
  repository would leak that repo's skills. `project_root_markers = []` pins the project
  root to the cwd.[^markers] A live run under a `.git`-marked directory with
  `.agents/skills/probe-leak` listed only the declared skill.[^live] The engine copies
  declared skills to `HOME/.agents/skills`, because `$CODEX_HOME/skills` is the
  deprecated location.
- **A cwd AGENTS.md.** `project_doc_max_bytes = 0`. A codeword planted in `work/AGENTS.md`
  never reached the context. The executor saw it only when it read the file itself.
- **Host keyring.** Codex looked for MCP OAuth tokens in the session D-Bus Secret Service,
  which is the host's keyring. `mcp_oauth_credentials_store = "file"` keeps them in the
  job's HOME.
- **MCP tools missing.** Unless `mcp_optional_startup_grace_ms = 0`, a server passed on
  acpx stdin that is still starting when the catalog is built never reaches the model.
- **Network and a reviewer model.** The adapter starts in mode `agent`, which has a
  workspace-write sandbox with the network off. That mode also sends out-of-workspace
  actions to a "Guardian Review" model, and the network cut breaks the inherited GitHub
  access. `acpx --approve-all` does not change the mode. The adapter reads
  `INITIAL_AGENT_MODE`,[^modes] so the engine sets it to `agent-full-access`. A kata can
  still pick another mode with `options: {mode: ...}`.
- **PATH rebuilt on NixOS.** Codex runs commands in the login shell from passwd, not
  `$SHELL`, which is zsh on this host. NixOS `/etc/zshenv` re-runs `set-environment` unless
  `__NIXOS_SET_ENVIRONMENT_DONE` is set. That rebuilds PATH from the executor's HOME, drops
  the inherited PATH and its `gh` wrapper, and `gh` then says "gh auth login". The engine
  inherits the marker.

Memories are off by default. Built-in tools (image generation, goals, multi-agent, web)
remain: they are harness capability, not host inheritance.

# Credentials

Codex saves a refreshed login by opening `auth.json` with truncate and writing through the
path.[^storage] The linked file is therefore updated in place in the real home, and the link
stays a link. The host login stays current, so removing the link after the job loses
nothing.
