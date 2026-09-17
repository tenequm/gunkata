---
type: Decision
title: Completion is verified, never claimed
description: A task is done when its named evidence artifact exists and passes its check; dependents release only after verification.
tags: [locks, scheduling, verification]
status: stable
generated: { by: claude-code/opus-5, at: "2026-09-17T02:02:39Z" }
sources:
  - id: prior
    resource: "Operator experience running the prior orchestration engine, 2026-09-02 to 2026-09-17; no public record"
    title: Two weeks operating the prior engine
  - id: locks
    resource: /AGENTS.md
    title: gunkata agent instructions, "The locks" (lock 1)
---

# Decision

A task in the DAG is complete when the evidence artifact the task named exists and passes
the check the task named. Nothing else completes a task. Dependents become runnable only
after that verification has run and passed.[^locks]

# Why

The prior engine treated a finished executor as a finished task. An agent that ran to
termination was recorded as successful, and its dependents were released on that record.
This is wrong in the most expensive direction: an agent that did nothing useful terminates
exactly like an agent that did the work, so the graph proceeds on a lie and the failure
surfaces several nodes later, where it is far harder to attribute.[^prior]

The corollary is that every task must name its artifact and its check up front. A task
without a named artifact cannot be verified, so it cannot be scheduled - that is a feature,
because it forces the graph author to say what the step is actually for. "The step ran" is
not an outcome.

# Consequences

- Every node declares an artifact path and a check before the graph is accepted.
- Release of dependents is a separate act from termination of the executor.
- A task whose executor succeeded but whose check failed is a failed task, without
  exception and without an override.
