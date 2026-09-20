---
type: Finding
title: A parked fan-out item throws away the whole round
description: In the first live runtime fan-out run, one of nine parallel probes hit its timeout and parked, which parked the head, left the report unrun and returned nothing from eight successful investigations; a round is sampled work, so an instance that fails is data the next round should be told about, not a broken dependency.
tags: [engine, fan-out, kata, park, quota, review]
status: stable
stale_after: "2027-03-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-19T19:45:00Z" }
sources:
  - id: run
    resource: "gunkata run 20260919T174005Z92da on ws-pond-01, 2026-09-19: katas/review-3.kata.yml against https://github.com/glim-sh/cuttle/pull/73, outcome parked after 6520 s, record.json fan_out {\"escalate\": {\"rounds\": 1, \"items\": [9], \"capped\": false}}, escalate failure \"item 5: executor exited 3\", report pending"
    title: The first live fan-out run
  - id: sleeps
    resource: "gunkata.log of run 20260919T174005Z92da, job probe/round-1/item-5: 11 tool titles containing sleep, including \"sleep 560\", \"./run-900.sh && sleep 450\" and \"sleep 500\", against a prompt that forbids waiting with sleep"
    title: The probe that slept through its budget
  - id: pr
    resource: "gunkata PR #5 feat(engine): runtime fan-out, commits e0da53d and 5ecb657, packages/gunkata/internal/engine/fanout.go"
    title: The fan-out implementation
---

# Finding

Runtime fan-out worked on its first live run: `escalate/round-1` wrote nine briefs into its
items directory (six skeptics, three dives), and the engine scheduled nine parallel `probe`
instances from them, each an ordinary job with its own directory, HOME and output check.[^run]

Eight finished, between 463 s and 1866 s. The ninth chose to settle its brief with a live
browser reproduction and waited on it with blind `sleep 560`, `sleep 450` and `sleep 500`
polls - about 2167 s of explicit sleeping, roughly 40% of its hour - and hit the 3600 s
timeout.[^sleeps] Parking one instance parked the head, which ended the loop, so `report`
never ran and the run produced no review. Nine investigations were paid for, eight answered,
and none of them reached a deliverable.

The engine treated the parked instance as a parked dependency, which is right for a declared
job and wrong for a round. A round is sampled work: the instances are independent
investigations, and one that returns nothing is a fact about that question, not a broken
edge. The next head can be told "item 5 timed out, nothing came back" and carry on. Park is
still the right answer when the head itself parks, or when a declared job does.

Two costs compound it. A parked executor emits no `turn end`, so the run's most expensive job
contributed $0 to the log: the run metered $86.00 while the parked hour is worth another $57
to $60 at the measured Opus rate, close to a whole quota window for nothing. And the caps
that bound the loop - `max_rounds: 3`, `max_items: 10` - bound rounds and width, not spend:
three full rounds cannot fit in one window, and the engine has no way to say how many
executor-minutes a run may take.

The mitigations that do not need a spec change are a shorter `timeout_seconds` on a template
profile, so a runaway probe costs ten minutes rather than an hour, and a narrower brief. The
anti-sleep instruction in the prompt did not bind; the timeout is what bound, an hour too
late.
