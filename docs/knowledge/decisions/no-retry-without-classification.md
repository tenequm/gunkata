---
type: Decision
title: No retry without classification
description: A failure is classified from transcript evidence before any retry; unclassified failures park the task.
tags: [locks, failure-handling, retries]
status: stable
generated: { by: claude-code/opus-5, at: "2026-09-17T02:02:39Z" }
sources:
  - id: prior
    resource: "Operator experience running the prior orchestration engine, 2026-09-02 to 2026-09-17; no public record"
    title: Two weeks operating the prior engine
  - id: locks
    resource: /AGENTS.md
    title: gunkata agent instructions, "The locks" (lock 4)
---

# Decision

Before a failed task may be retried, the failure is classified from transcript evidence into
one of quota, timeout, refusal, or real defect. A failure that cannot be classified parks
the task; it is not retried and it does not release dependents.[^locks]

# Why

Blind retry is how an engine turns one failure into an expensive one. The four classes want
opposite responses, and a single retry policy is wrong for at least three of them.[^prior]

A quota failure wants waiting, or a different lane. Retrying immediately burns the remaining
budget against a wall and converts a delay into an outage.

A timeout wants a larger budget or a smaller task. Retrying unchanged reproduces the timeout
exactly, at full cost, as many times as the retry count allows.

A refusal wants a changed prompt or a human. Retrying identical input to a model that
declined it is the purest form of wasted wall time, and wall time is the metric the pipeline
optimizes.

A real defect wants a human, or a repaired input. Retrying hides it: run three succeeds, the
record shows success, and the defect ships.

Parking rather than retrying on an unclassified failure is deliberate. An engine that
guesses the class is an engine that silently picks one of the four wrong responses, and the
cost of a parked task that a human looks at is far lower than the cost of a wrong automatic
response that nobody sees.

# Consequences

- Retry is downstream of classification; there is no path from failure to retry that skips
  it.
- Classification reads transcripts, which is why lock 2 requires them synced at task
  boundary.
- Parked is a first-class terminal state, distinct from failed, and it is visible.
- The starter case's fail variant asserts parking, not retrying.
