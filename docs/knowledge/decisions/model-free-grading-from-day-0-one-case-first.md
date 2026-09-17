---
type: Decision
title: Model-free grading from day 0, one case first
description: The corpus starts with exactly one deterministic case the engine must pass before anything else is built, and grader tests never involve a model.
tags: [locks, grading, corpus, testing]
status: stable
generated: { by: claude-code/opus-5, at: "2026-09-17T02:02:39Z" }
sources:
  - id: prior
    resource: "Operator experience running the prior orchestration engine, 2026-09-02 to 2026-09-17; no public record"
    title: Two weeks operating the prior engine
  - id: locks
    resource: /AGENTS.md
    title: gunkata agent instructions, "The locks" (lock 7)
  - id: case
    resource: /docs/2609-17-starter-corpus-case.md
    title: Starter corpus case
---

# Decision

Grading is model-free from day 0. The corpus starts with exactly ONE deterministic,
unambiguous case, verifiable in under ten minutes, and the engine must pass it before
anything else is built. The corpus grows only from real cases encountered later, never
speculatively. Grader tests never involve a model.[^locks] The case is specified in
`/docs/2609-17-starter-corpus-case.md`.[^case]

# Why

Three separate lessons collapse into this one rule.

**A model in the grader means the regression signal is itself noisy.** In the prior project
every corpus failure it ever had turned out to be the model's structured verdict disagreeing
with the deterministic grader, not the engine regressing - and once that class of failure was
moved into ordinary unit tests it failed in milliseconds instead of costing a full corpus
run.[^prior] A grader that can be wrong cannot tell you the engine is wrong.

**One case first, because the engine has to be provable before it is large.** An engine
built for weeks and then measured has no known-good point to bisect back to. The single case
is the known-good point, and requiring it to pass before anything else is built means there
is never a period during which the engine is unverifiable.

**Speculative cases are a tax, not coverage.** A case invented from imagination encodes a
guess about what will break. It costs wall time on every run forever and, when it fails, it
is as likely to be the case that is wrong as the engine. A case distilled from a real failure
is paid for by a failure that already happened; it can only be earned, never predicted.

The ten-minute ceiling and the one-case rule together keep the loop tight enough that it is
actually run. A corpus nobody runs is not a regression signal.

# Consequences

- The grader is a deterministic function of files and exit codes, unit-tested on its own.
- The first milestone of the engine is passing the starter case, both variants.
- A second case is added only when a real failure is reduced to one; a pull request adding a
  speculative case is rejected on that ground alone.
- The corpus run is fast enough to be a gate, not a ceremony.
