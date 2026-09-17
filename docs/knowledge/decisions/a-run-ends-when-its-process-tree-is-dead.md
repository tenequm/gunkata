---
type: Decision
title: A run ends when its process tree is dead
description: Teardown is part of the run contract; the engine owns the full process tree via process groups.
tags: [locks, runtime, teardown]
status: stable
generated: { by: claude-code/opus-5, at: "2026-09-17T02:02:39Z" }
sources:
  - id: prior
    resource: "Operator experience running the prior orchestration engine, 2026-09-02 to 2026-09-17; no public record"
    title: Two weeks operating the prior engine
  - id: locks
    resource: /AGENTS.md
    title: gunkata agent instructions, "The locks" (lock 5)
---

# Decision

A run is over when every process it started is dead, not when its top-level process exits.
The engine starts each executor in its own process group and owns that group, so teardown
reaches the whole tree.[^locks]

# Why

An agent executor is not one process. It spawns a language runtime, which spawns tool
processes, which spawn servers, and any of those can outlive the parent. In the prior engine
this produced two costs.[^prior]

The first is resource leak. Orphaned servers hold ports and memory, which then collides with
lock 3's per-run allocation - a run that was isolated on paper is not isolated in practice
because a dead run's process is still holding what the new run asked for. The leak compounds
across a day of runs until the machine is the failure.

The second is worse: an orphan keeps writing. A tool process that survives its run keeps
producing output into paths the engine believes are settled, so an artifact can change after
it was verified. That breaks lock 1 at the root - verification is only meaningful if the
verified thing stops moving.

Killing by PID does not solve this, because the PID the engine holds is the parent's and the
children have reparented. Process groups are the mechanism that makes teardown total rather
than best effort.

# Consequences

- Every executor is started in a new process group; the engine records the group, not just
  the PID.
- Teardown is a contractual step of the run, executed on success, on failure and on
  cancellation alike.
- A run that cannot confirm its tree is dead does not report as ended.
- Artifact verification happens after teardown, so nothing can rewrite a verified artifact.
