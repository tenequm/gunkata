---
type: Finding
title: acpx runs the claude and codex adapters through npm exec, so a bare HOME re-installs them every job
description: A Nix-packaged acpx 0.17 finds no bundled claude-agent-acp or codex-acp and runs them via npm exec --yes against its pinned range, never via PATH; with a per-job HOME that is a ~400-900 MB install per job, and one shared npm_config_cache turns it into a sub-second no-op reify.
tags: [executor, acpx, npm, claude, codex, performance]
status: stable
stale_after: "2026-12-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-19T00:10:00Z" }
sources:
  - id: registry
    resource: https://github.com/openclaw/acpx/blob/main/src/agent-registry.ts
    title: acpx 0.17.0 agent-registry.ts (resolveBuiltInAgentLaunch, package ranges)
  - id: libnpmexec
    resource: https://github.com/npm/cli/blob/latest/workspaces/libnpmexec/lib/index.js
    title: npm 11.17.0 libnpmexec (missingFromTree, _npx hash dir, concurrency.lock)
  - id: live
    resource: "Live gunkata runs on this host, 2026-09-19: acpx 0.17.0 (nix), node 24.19.0, npm 11.17.0, claude-haiku-4-5 and gpt-5.6-luna"
    title: Job home sizes and npm exec timings with and without the shared cache
---

# Finding

acpx launches a built-in adapter from its own `node_modules` tree if the package is there,
else through `node npm-cli.js exec --yes --package=<pkg>@<range> -- <bin>`. The adapter
binary on PATH is consulted only by `acpx` inspection, never by the launch.[^registry] The
Nix acpx package ships no adapters, so every claude and codex executor goes through npm
exec, and with the engine-owned HOME npm filled `home/.npm` from scratch each job: ~925 MB
per claude job home before the fix.[^live]

npm exec keys its install dir as `_npx/<sha512(spec)[:16]>` under the cache. For a range
spec in that dir it always revalidates the packument (one conditional GET) and runs a
reify, but the reify is a no-op (`reify moves {}`) when the installed version is still
current - nothing is downloaded or extracted. Loads and installs of one `_npx` dir are
serialized by a `concurrency.lock` in it, and the `_cacache` store is content-addressed, so
parallel jobs may share one cache.[^libnpmexec]

The engine therefore points `npm_config_cache` at `$XDG_CACHE_HOME/gunkata/npm`. Measured
for the exact acpx invocation: claude-agent-acp 5.0 s cold vs 0.7 s shared, codex-acp 5.4 s
vs 0.7-1.2 s; job homes dropped to 3-7 MB.[^live] acpx's pinned ranges stay in force - the
spec, and so the `_npx` dir, changes when acpx bumps a range - and a newer release inside a
range is picked up by the revalidation.

The cache holds package code and npm's own logs, no harness config, so executors stay
bare. It is shared writable state across jobs, but not a new capability: executors run as
the engine's uid on an unsandboxed filesystem and could reach it anyway.

[^registry]: acpx 0.17.0 agent-registry.ts (resolveBuiltInAgentLaunch, package ranges)
[^libnpmexec]: npm 11.17.0 libnpmexec (missingFromTree, _npx hash dir, concurrency.lock)
[^live]: Job home sizes and npm exec timings with and without the shared cache
