---
type: Decision
title: Cost is observability, never control flow
description: Dollar amounts are recorded and reported but no bound in the engine may read one.
tags: [locks, cost, control-flow]
status: stable
generated: { by: claude-code/opus-5, at: "2026-09-17T02:02:39Z" }
sources:
  - id: prior
    resource: "Operator experience running the prior orchestration engine, 2026-09-02 to 2026-09-17; no public record"
    title: Two weeks operating the prior engine
  - id: locks
    resource: /AGENTS.md
    title: gunkata agent instructions, "The locks" (lock 6)
  - id: reqs
    resource: /docs/2609-17-build-pipeline-requirements.md
    title: Build pipeline requirements, B1 (wall time is the primary metric)
---

# Decision

Cost is recorded, aggregated and shown. No bound, ceiling, gate, retry decision, scheduling
decision or termination condition in the engine may read a dollar amount.[^locks]

# Why

The models come from subscriptions, and the metric the pipeline optimizes is wall time from
zero to completion with quality held; cost is explicitly secondary.[^reqs] A dollar-reading
bound therefore optimizes the wrong variable by construction.

It also fails in a specific and nasty way. A cost ceiling truncates the run that needed the
most work - the hard task, the large diff, the one where quality mattered most - and it does
so at an arbitrary point, producing a half-finished artifact rather than a clean failure.
The engine then has to decide what a truncated-by-budget task means, and there is no good
answer: it is neither a success nor a classifiable failure under lock 4.

The prior project reached the same rule and kept exactly one deliberate exception, which was
non-extensible by explicit agreement.[^prior] gunkata carries the rule with no exception at
all. The cheapest moment to refuse a cost-reading bound is before the first one exists,
because every later one arrives with a good local argument.

# Consequences

- Cost appears in reports, ledgers and the transcript record, and nowhere in a conditional.
- A run that is expensive is a fact to look at, not an event the engine acts on.
- Wall time and quality bounds are the only bounds; if a run must be stopped, it is stopped
  on time or on evidence.
