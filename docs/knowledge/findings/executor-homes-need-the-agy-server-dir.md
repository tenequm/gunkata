---
type: Finding
title: A bare executor HOME must still carry the agy server directory
description: The Antigravity ACP wrapper resolves its .par through $HOME, so an engine-owned HOME breaks executor startup until the server directory is explicitly inherited alongside the credentials.
tags: [executor, acpx, agy, home, bare-start]
status: stable
stale_after: "2026-12-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-17T19:09:00Z" }
sources:
  - id: executor
    resource: /packages/gunkata/internal/engine/executor.go
    title: linkAuth and the executor environment whitelist
  - id: live-run
    resource: "Live starter-corpus run, 2026-09-17: /home/tenequm/.local/state/gunkata/runs/20260917T182726Z908c"
    title: First live run, parked at produce with exit 127
  - id: wrapper
    resource: ~/.local/bin/agy-acp-server
    title: The NixOS wrapper that execs the .par with --uid=
---

# Finding

The executor contract says an executor inherits nothing but subscription credentials and
auth. The first live corpus run proved that rule has a second, non-obvious member: the ACP
server's own installation directory.

`produce` parked with `executor_exit: 1` and this in its `executor.log`:[^live-run]

```
[acpx] error: RUNTIME AGENT_STARTUP_FAILED ACP agent exited before initialize completed
(exit=127, signal=null): agy-acp-server: missing
<runDir>/nodes/produce/home/.local/lib/antigravity-acp/agy_acp_server.par - run agy-acp-install
```

The cause is that the wrapper passed as acpx's `--agent` resolves the `.par` it executes
through `$HOME`, not through an absolute path baked in at install time.[^wrapper] The engine
overrides `HOME` to the node's engine-owned directory, so a wrapper that was perfectly valid
on the host pointed at a path that had never existed. The credentials were symlinked in
correctly; the binary the credentials are for was not.

The fix keeps the contract intact rather than widening it: `.local/lib/antigravity-acp` is
symlinked into the node's HOME by the same code that links `settings.json` and
`acp_token.json`, so the inheritance stays explicit and enumerable in one place.[^executor]

# Why this is the contract working, not failing

Three properties of the failure are worth keeping, because they are what the locks were
bought for. The executor did not silently degrade - it failed at startup with a non-zero
exit. The engine did not trust the executor's own account of itself: `produce` parked on
evidence (exit code, then a missing artifact), `gate` never ran, and `consume` never started.
And the cause was legible from the transcript the engine had already captured, which is the
argument for owning executor HOMEs in the first place.

# Generalization

Any executor reached through a wrapper is suspect in the same way. A vendor's launcher may
resolve its runtime, its configuration or its update channel through `$HOME`, and none of
that is visible in the flags the engine passes. The rule to apply when adding an executor:
run it once in a fresh engine-owned HOME and read the startup failure, rather than reasoning
about what it ought to need. Exit 127 before `initialize` means a path the wrapper expected
under `$HOME` is absent.
