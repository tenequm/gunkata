---
type: Decision
title: Evidence is files and exit codes
description: The engine gates on artifacts on disk and transcripts synced into its own dedicated pond, never on a stream, a status field, or an agent's say-so.
tags: [locks, verification, transcripts, pond]
status: stable
generated: { by: claude-code/opus-5, at: "2026-09-17T02:02:39Z" }
sources:
  - id: prior
    resource: "Operator experience running the prior orchestration engine, 2026-09-02 to 2026-09-17; no public record"
    title: Two weeks operating the prior engine
  - id: locks
    resource: /AGENTS.md
    title: gunkata agent instructions, "The locks" (lock 2)
---

# Decision

Executors run in engine-owned HOME directories at predictable per-run paths. At task
boundary the engine syncs that run's transcripts into gunkata's own dedicated local pond -
its own `pond serve` over its own storage directory, holding run transcripts only, kept
separate from any personal pond - and then gates on two things: the artifact check, which
is the done-bit, and failure classification informed by the transcript. The engine never
trusts a stream, a status field, or an agent's say-so.[^locks]

Live-write into pond during a run is the upgrade path for mid-run supervision. It is
explicitly not a v1 dependency: v1 syncs at task boundary and nothing in the gate may
require a live feed.

# Why

Three separate failures in the prior engine all reduce to trusting something that was not
evidence.[^prior]

A stream is not evidence. Structured progress events arrive while an agent is confidently
doing the wrong thing, and they stop arriving for reasons that have nothing to do with the
work. Gating on them means the gate has no idea what happened.

A status field is not evidence either, because the agent or the adapter writes it. It
records an intention, and the intention is exactly the thing under question.

Transcripts are evidence, but only after the fact and only for classification - they say
what the executor did and said, which is what lock 4 needs to tell a quota failure from a
real defect. They do not say whether the work is correct. That is the artifact check's job,
and keeping the two roles apart is what stops transcript reading from drifting back into
trusting narrative.

A dedicated pond rather than the operator's personal one keeps the two corpora from
contaminating each other: run transcripts are machine data the engine queries and prunes,
personal session history is not, and an engine that can prune is an engine that must not be
pointed at anything irreplaceable.

# Consequences

- The engine owns HOME for each executor and knows every path it will write.
- Transcript sync is part of the task boundary, not a background best effort.
- The gate is artifact check first; transcripts inform classification only.
- pond storage for runs is separate infrastructure with its own storage directory.
