---
type: Reference
title: pond PR 289 as a review ground-truth case
description: A hand /polish review of tenequm/pond#289 (lance 12 upgrade) with 8 findings and 10 validated non-findings, pinned to head 6a83a53, held as the grading key for a future replay of the gunkata review flow.
tags: [review, corpus-candidate, ground-truth, pond]
status: stable
generated: { by: claude-code/fable-5, at: "2026-09-17T22:20:00Z" }
sources:
  - id: pr
    resource: https://github.com/tenequm/pond/pull/289
    title: "feat: upgrade Lance to 12.0.0 (+265/-249, 7 files)"
  - id: polish
    resource: "Operator's interactive /polish run over PR 289, fix mode, pasted into the gunkata session of 2026-09-17; no other durable copy exists - this concept is the record"
    title: the hand review this concept transcribes
  - id: pins
    resource: "GitHub API read of pulls/289, 2026-09-17: head 6a83a53439d5b4b0463d8ee7d897605d69dc2ba8, base f9f4c8aea7679b3a936ee05618b3990345a90c01, commits a08e9a3c + 6a83a534"
    title: the exact-commit pins
---

# Purpose

When the gunkata review graph is ready for a ground-truth replay (the
review-v1 pattern: run the flow, score recovered findings against a hand
review with known outcomes), this PR is the banked case. The replay MUST run
against the pinned head commit, never the branch's current head - a later
rebase or the fixes this very review recommends would grade the flow against
a tree in which the findings no longer exist.

# Pins

- PR: [tenequm/pond#289](https://github.com/tenequm/pond/pull/289), "feat:
  upgrade Lance to 12.0.0", +265/-249 across 7 files, open at capture.[^pr]
- Head: `6a83a53439d5b4b0463d8ee7d897605d69dc2ba8` (commits `a08e9a3c` feat,
  `6a83a534` polish).[^pins]
- Base: `f9f4c8aea7679b3a936ee05618b3990345a90c01`. Main moved to `deb580e`
  (CI/nix revamp) while the hand review ran; the PR's green checks are from
  the pre-revamp pipeline.
- Review context: fix mode, worktree checkout, phase-1 gates green
  (`cargo clippy -D warnings` exit 0, `cargo fmt --check` clean).[^polish]

# The grading key: 8 findings

A replay is scored on recovering these. File:line are in the PR-head tree;
"out of diff" findings require reading beyond the hunks - the class review-v1
measured generic lenses missing.[^polish]

Correctness (both from lance 12 moving `LanceFileVersion`'s default 2.1 to
2.2, breaking the two creation sites the V2_1 pin does not cover):

1. `packages/pond/src/substrate.rs:5906` (out of diff) -
   `Dataset::write(reader, uri, None)` in
   `scan_verify_rejects_zeroed_column_add_data_file` now writes format 2.2;
   the test asserts physical per-fragment data files in a format production
   never writes, and still passes, so nothing announces the drift. Fix:
   `Some(sessions::write_params_for_create())`.
2. `packages/pond/benches/fmindex_probe.rs:168` (out of diff) -
   `WriteParams::default()`, same 2.1 to 2.2 drift; index size/latency
   numbers after the merge are not comparable with ones before, silently.

Cleanliness:

3. `packages/pond/src/substrate.rs:3883` - new `wrap_paginated` doc claims
   "read-through, so a cached path is one the origin holds too"; the module's
   own test at `substrate.rs:4090-4097` proves the opposite (cache serves
   INDEX after `inner.delete`). Conclusion right on the comment's other
   ground; the one clause is false.
4. `packages/pond/src/sessions.rs:6600-6601` (out of diff) - the V2_1 pin's
   in-code rationale says 2.2 is "marked unstable in Lance docs"; at lance 12
   that file marks only 2.3 unstable and 2.2 is the stable default. The
   comment the upgrade most needed to refresh; the PR refreshed two lesser
   ones (`sql.rs:717`, `substrate.rs:3595`) and missed it.
5. `packages/pond/src/substrate.rs:1304` (out of diff) - "Lance 10 has two
   such families, ZoneMap and BloomFilter"; lance 12 has three (fmindex).
   Inventory and version label stale; the conclusion still holds.
6. PR description - claims writing 2.2 sets the sticky mixed-versions flag
   (bit 8) so lance 11 readers refuse the store; lance 12.0.0 only reserved
   the flag (`lance-table-12 feature_flags.rs:56,66,163-166`), nothing sets
   it. Wrong claim in what becomes the permanent squash-merge record.

Design:

7. `packages/pond/src/main.rs:1` - the new `recursion_limit` comment names
   "Lance 12", which churns per upgrade; the four sibling comments state the
   mechanism version-free.

Efficiency:

8. No bench-gate row for the upgrade, which `AGENTS.md:90` mandates for
   exactly this change class; `bench-gate-baseline.jsonl` newest row is
   pre-upgrade (`abc49b9`), `results.md` has no lance-12 section. Timely
   because the old-side binary still exists at
   `target/pond-pre-upgrade-lance11` and the irreversibility window
   (`AGENTS.md:91`) had not closed at review time.

# The negative key: 10 validated non-findings

A replay is also scored on NOT reporting these; each was raised and dropped
with evidence. Short forms - the full traces are in the polish
report:[^polish]

1. Duplicate object_store (0.13.2 + 0.14.2) in the lock - upstream lance 12
   dependency, never crosses pond's boundary.
2. IVF_RQ `num_bits` default change - pond has no IVF_RQ.
3. NGRAM rebuild requirement - no NGRAM on production paths.
4. lance-namespace response-object change - pond calls none of the four
   affected methods.
5. Vendored `FtsQueryUDTF` - still needed, upstream byte-identical 11 to 12.
6. ZoneMap / upstream #7434 re-verification - satisfied by a live assert in
   the passing suite (`tests/integration/search.rs:330-339`).
7. `MESSAGES_VECTOR_INDEX` naming an IVF_SQ index - persisted identifier,
   documented legacy name, rename is a migration.
8. Bench reusing the production FTS index name - deliberate, documented.
9. No wrapper-bypass hazard on the store trait - identical 20 methods in
   object_store 0.13.2 and 0.14.2.
10. Recursion limits vs the pending toolchain rebase - channel unchanged.

# What makes this case valuable

Five of the eight findings are out of diff or outside the tree entirely (PR
body, missing bench row), the correctness pair requires knowing an upstream
default changed between versions, and the negative key is long - a flow that
fans out shallow lens reads will score near zero here. It is a much harder
case than cuttle#62 and sits in the same difficulty class as review-v1's
PR-5737 replay.

[^pr]: the pull request
[^polish]: the hand review this concept transcribes
[^pins]: the exact-commit pins
