---
type: Decision
title: Zero process-global state
description: Every run gets its own ports, temp dirs and config paths; no defaults are shared between runs.
tags: [locks, isolation, runtime]
status: stable
generated: { by: claude-code/opus-5, at: "2026-09-17T02:02:39Z" }
sources:
  - id: prior
    resource: "Operator experience running the prior orchestration engine, 2026-09-02 to 2026-09-17; no public record"
    title: Two weeks operating the prior engine
  - id: locks
    resource: /AGENTS.md
    title: gunkata agent instructions, "The locks" (lock 3)
---

# Decision

Every run allocates its own ports, its own temporary directories and its own configuration
paths. No default location, no well-known port, and no shared cache is used by more than
one run.[^locks]

# Why

Shared defaults are the reason two runs that are individually correct are jointly wrong. A
well-known port means the second run either fails to bind or, worse, attaches to the first
run's server and reads its data. A default config path means run B picks up whatever run A
left behind, and the resulting behaviour is neither run's intent. A shared cache means a
result computed under one run's configuration is served to another.[^prior]

The damage is disproportionate to the cause because it is not reproducible. A run that fails
only when another run happens to overlap it looks like flakiness in the model, in the
network, or in the executor, and costs days of investigation in the wrong place. Isolation
is cheap; diagnosing a cross-run collision is not.

This is also what makes the corpus meaningful. A case that passes alone and fails under
concurrency has proven nothing, and lock 7's single case is only trustworthy if the engine
cannot contaminate it from a neighbouring run.

# Consequences

- Port allocation is dynamic and per run; no constant is a port number.
- Temp and config paths are derived from a run identifier, never from a default.
- The engine's own pond instance is addressed per run, not by a well-known endpoint.
- Two runs of the same graph must be able to execute concurrently with no coordination.
