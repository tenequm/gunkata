---
type: Reference
title: 2026-09-19 report - why a gunkata review missed a bug a session review found (cuttle PR 73)
description: Dated forensic report comparing /polish reviews of glim-sh/cuttle#73 run through the gunkata review kata and from a Claude Code session, with the same model, effort, skill and prompt; the kata miss traced to lead prompt anchoring, a checkout mutated mid-review and reasoning variance - not tools, MCP, skills, CLAUDE.md, model or effort - plus the wall-time breakdown and the harness and kata changes that followed.
tags: [review, kata, polish, forensics, quality, acpx, claude, ground-truth]
status: stable
stale_after: "2027-03-19T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-19T10:30:00Z" }
sources:
  - id: pr
    resource: https://github.com/glim-sh/cuttle/pull/73
    title: "feat(serve): close the idle session browser under --idle-timeout (draft, head 04cd6ac)"
  - id: runs
    resource: "gunkata run dirs on ws-pond-01 under ~/.local/state/gunkata/runs/: 20260918T231146Z423f, 20260918T234700Z4a28, 20260918T235932Z705c, 20260919T015424Z9b75, 20260919T022043Z1b56 (record.json, gunkata.log, jobs/*/executor.jsonl, artifacts/*/report.md, Claude transcripts under jobs/*/home/.claude/projects)"
    title: the kata runs
  - id: session
    resource: "Claude Code session 93876eaf-9cc0-4da8-aa6e-6f2c27efa696 on ws-pond-01, subagent transcripts agent-adb24c3865a33f874 and agent-a32d08f151551ca2a with their review sub-agents, under ~/.claude/projects/-home-tenequm-pj-gunkata/93876eaf-.../subagents/; also indexed in pond"
    title: the session-path runs
  - id: forensic
    resource: "Forensic comparison subagent a24cf7c09d05adc0e in the same session, 2026-09-19; scratch extracts and scripts (lat.py, think.py, perm.txt) in that session's scratchpad/forensic/"
    title: transcript-level comparison of run A and run B
  - id: harness
    resource: "Harness investigation subagent a46d538d7967a72d0 in session 93876eaf-..., 2026-09-19: live haiku probes toggling one env var at a time; version survey of acpx and claude-agent-acp"
    title: Monitor gating, subagent waits, adapter versions
  - id: judge
    resource: "Blind judge subagent ac8c0138476a3399f, 2026-09-19; anonymized reports and mapping in that session's scratchpad/judge/"
    title: K1-K3 and recurring-finding scoring of 12 reports
  - id: timing
    resource: "Timing forensics subagent a6f62f4994b6fc44a, 2026-09-19; scripts tl.py, metrics.py, phases.py, lat.py, sig.py in that session's scratchpad/tf/"
    title: per-phase timelines of the 12 runs
  - id: skill
    resource: https://github.com/tenequm/skills/tree/main/skills/polish
    title: the polish skill, Phase 3 hand-off rules (snapshot SHA 1cf72df in the run dirs)
---

# What was compared

The same draft PR[^pr] was reviewed repeatedly with `/polish` in review mode, first through
the gunkata `review` kata (bare executor: acpx 0.17.0 -> claude-agent-acp 0.76.0 bundling
Claude Code 2.1.257, engine-owned HOME, only the gh-cli and polish skills, no MCP)[^runs]
and then from a Claude Code session subagent (host Claude Code 2.1.277, full host
environment: global CLAUDE.md, dozens of skills, glim/notion/pond MCP and claude.ai
connectors).[^session]

Three findings serve as the grading key, each verified by hand against the code at
`04cd6ac`:

- **K1 pool-mode fail-open** - `pool.go:351` treats a missing idle deadline as due; a
  timer that fires while `getOrLaunch` relaunches a crashed Chrome (`removeProcess`
  deletes the deadline at `:881`, pool mode arms no timer at launch `:587`) reaps the
  fresh browser before its client connects.
- **K2 profile/login loss** - in session mode with a non-durable profile, an idle reap
  goes `idleReap` -> `captureAndTerminate` -> `terminate` -> `safeRemoveTree`, and the
  profile is kept on a failed capture only while shutting down (`supervise.go:144`), so a
  failed final capture deletes the only copy of the logins.
- **K3 base drift** - the branch is 15 commits behind main, conflicts in 6 files, and main
  has rewritten the same reap code (#86 arm in all modes after re-inject, #93 timer
  identity guard, #91 dialog dismissal on `connect`).

# Results

| Run | Path | Model / effort | Fan-out | Issues | K1 | K2 | K3 | Wall |
|---|---|---|---|---|---|---|---|---|
| kata 231146 | kata, host skills leaked in | sonnet / default | 4 agents | 11 | yes | no | not checked | 12 min |
| kata 234700 | kata, bare | sonnet / default | none | 3 | no | no | no | 7.5 min, $2.10 |
| kata 235932 | kata, 3 passes + merge | 2 sonnet + 1 opus / default | pass-a/b only | 2 correctness | no | no | not checked | 20 min, $15.60 |
| kata 015424 | kata, bare | opus / high | none | 14 | subsumed by K3 | no | yes | 11.8 min, $4.70 |
| **kata 022043 (run A)** | kata, bare | opus / high | 4 agents | 28 | yes | **no** | yes | 19.8 min, $13.79 |
| session adb24 | session, bare prompt | opus / high | 4 agents | 20 | yes | yes | yes | 8.4 min |
| **session a32d0 (run B)** | session, kata's prompt | opus / high | 4 agents | 26 | yes | **yes** | yes | 9.1 min |

Early conclusions that did not survive checking, kept because they were plausible:

- "A Claude Code subagent cannot spawn subagents, so the session run had no fan-out" -
  false; pond shows session adb24 made 4 Agent calls at spawn depth 2.[^session]
- "The session ran at a different effort" - false; every message in all ten transcripts
  records `"effort":"high"` and model `claude-opus-5`.[^forensic]
- "Multiple identical passes raise recall" - the 3-pass run resampled the same blind
  spot, cost the most, and found neither K1 nor K2.
- "The prompt explains the gap" - only partly; with the kata's exact prompt the session
  path still found K2 and run A still did not.

# Why run A missed K2 while run B found it

Ranked by evidence strength.[^forensic]

1. **Reasoning variance in the side-effect agent (partly verified).** A's Side-Effect
   Gating agent read all of `supervise.go` at 02:25:45, including the comment at `:139`
   ("A failed capture means this browser's state exists nowhere but its profile dir ...
   Only while shutting down") and the gate at `:144`, and cited the chain `pool.go:375`
   -> `supervise.go:148` -> profile-dir removal itself - then reasoned past it: "the first
   reap always writes a snapshot", "`idleReap` does not check `p.closing`, but no
   side-effect follows from that". It treated the profile deletion as the reap's intended
   cost, not as a gated side-effect. B's agent read the same lines at 03:01:08 and reported
   it at 03:04:27. Session path: 2/2 hits; comparable kata runs: 0/1 - too few to call it
   systematic.
2. **The lead anchored its agents (inferred, moderate).** A's lead wrote 19.2k chars of
   agent prompts including five fully worked hypothesis traces (lease race, viewer race,
   stale reap vs timer identity, driver mid-command, arm-before-reinject) and a rollback
   example aimed at `processes`. All six of A's side-effect findings map one-to-one onto
   those traces and that example; the example occupied the rollback slot where K2
   belonged. B's lead (13.9k chars) posed open questions, left rollback generic, and three
   of its agent's seven findings went beyond its prompt, K2 among them. The polish skill's
   Phase 3 prescribes the hand-off as the agent's own checklist section, the diff path,
   the changed-file list, an untrusted-data notice and Phase 2/3 context - never
   suspected findings.[^skill] A's lead improvised beyond that.
3. **The lead mutated the checkout mid-review (verified; effect inferred).** A's lead ran
   `git merge --no-commit origin/main` in the tree its agents were reading at 02:25:13 and
   aborted at 02:25:42. The side-effect agent read `pool.go` with conflict markers at
   02:25:22, spent turns diagnosing it, and reviewed from a 13-file snapshot it extracted
   to `/tmp/pr73head`; the Cleanliness agent hit the same markers. B's lead used the
   non-mutating `git merge-tree`.
4. **Ruled out (verified).** Tools, MCP, skills, CLAUDE.md, model, effort. Neither run used
   any MCP, web tool, pond or skill besides polish; the only visible effect of B's global
   CLAUDE.md was an `fd`/`rg` line passed to its agents. Nothing A tried was missing: 63
   permission requests, all answered `allow-once` by acpx at 0.1-2 s. B also started with
   far more context (agents ~40k tokens vs ~8k in A) and still found it.

# Wall time: 1190 s (A) vs 548 s (B)

| Phase | A | B | A extra |
|---|---|---|---|
| Engine start, skill fetch, adapter start | 7 s | 0 | +7 s |
| Skill load, PR metadata | 11 s | 15 s | -4 s |
| Clone and checkout | 25 s (depth-50 clone, failed `gh pr checkout`, unshallow) | 8 s | +17 s |
| Validation `just check` | 60 s foreground | 37 s background | ~+23 s |
| Diff prep and base drift | 28 s | 28 s | 0 |
| Writing the four agent prompts | 100 s | 54 s | +46 s |
| Fan-out, first start to last end | 561 s | 352 s | +209 s |
| Lead idle after the last agent finished | **243 s** (`sleep 240`) | 0 | +243 s |
| Final validation | 66 s | ~16 s | +50 s |
| Writing the report | 116 s (24.7 KB `Write`) | 45 s (13.8 KB handback) | +71 s |

- The four agents ran concurrently in both runs; A's starts staggered up to 78 s as each
  tool block finished streaming.
- Agent time was model time, not tools: A's agents spent 1-3 s in tools in total; A's
  side-effect agent averaged 13.9 s per turn vs B's 9.3 s, with more thinking.
- **The idle sleep is a harness effect.** The agents were launched in the background and
  the executor had no `Monitor` tool, so the lead polled with blind `sleep 120/180/240`
  while completion notifications queued behind the sleep. Correction, verified later:
  ending the turn does not end the job - the adapter keeps the prompt open while spawned
  agents run - but agents finishing within ~2 s of each other can lose results, and
  `Monitor` is absent because `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` disables the
  feature flag that enables it; see
  [the foreground-wait finding](/findings/claude-executor-waits-on-subagents-in-the-foreground.md).[^harness]

# What changed as a result

- The review kata runs on `claude-opus-5` with `options: {effort: high}`, a single pass,
  and a prompt that checks the PR against its base branch, requires polish's four
  parallel agents, holds the lead to polish's Phase 3 hand-off (facts, not conclusions;
  no worked traces or example bugs), launches the agents in the foreground (no sleep
  polling), and forbids changing the checkout during the fan-out (`git merge-tree` and
  `git show`, never `git merge` or `git checkout`).
- Deliberately rejected: more passes, and a fifth agent aimed at data-loss paths - both
  target this PR's miss rather than its cause.
- Harness: `record.json` records the acpx, adapter and Claude Code versions per job; no
  newer acpx exists (0.17.0 pins claude-agent-acp ^0.76.0) and the newest adapter,
  0.79.0, bundles Claude Code 2.1.274; a profile can pin it with `acp_adapter`.[^harness]

# Measurement: 12 runs, same prompt, blind-scored

Five kata runs (adapter 0.76.0), two kata runs pinned to adapter 0.79.0, and five session
subagent runs, all with the final kata prompt, run in parallel batches on 2026-09-19, then
scored by a judge that saw only anonymized reports.[^judge] Core-7 is the seven most
recurrent non-key findings; the recurring set is all 25 findings raised by 4+ reports.

| Group | K1 | K2 | K3 | Core-7 recall | Recurring recall | Median wall |
|---|---|---|---|---|---|---|
| kata, adapter 0.76.0 (5) | 4/5 | 4/5 (1 exact, 3 broader always-delete path) | 5/5 | 77% | 64% | 1140 s |
| kata, adapter 0.79.0 (2) | 1/2 | 1/2 (broader path) | 2/2 | 79% | 62% | 1159 s |
| session (5) | 5/5 | 4/5 (1 exact, 3 filed as follow-ups) | 5/5 | 86% | 66% | 548 s |

- With the prompt fixed, quality is at parity within noise: K2 and K3 equal, recurring
  recall 64% vs 66%, all runs fanned out to the four agents and none polled with sleep.
- Both K1 misses are kata runs whose side-effect agent examined the right mechanism and
  cleared it in "Dropped" ("the guard holds") - the same failure shape as the original K2
  miss: seeing the path and reasoning past it.
- K2 is found in two forms: session runs usually name the exact failed-capture gate but
  downgrade it to a follow-up; kata runs usually flag the broader always-delete path as
  blocking. Only 2 of 12 name `supervise.go:144` as a main finding.
- Adapter 0.79.0 made no measurable difference.
- False positives were rare: "quote the `?` in the attach script" (sh passes an unmatched
  glob through literally) in two session reports, and one kata aside calling an endpoint
  loopback-only (it binds 0.0.0.0 in a container).

# Wall time: kata 1140 s vs session 548 s (medians)

Ranked by seconds on the critical path; the first three overlap.[^timing]

| Cause | Seconds | Status |
|---|---|---|
| Kata review agents do ~2x the work: 1.6x API calls, 2.2x thinking tokens, 1.5x longer reports | ~150 | volume verified; cause inferred - session agents inherit the host's global CLAUDE.md concision rules, the bare executor has none |
| Token generation ~27% slower through the kata (65 vs 89 est. tokens/s; 79 vs 101 where exact), solo runs included | ~150-200 | measured; the adapter maps `claude-opus-5` to `opus[1m]` (1M context) - suspected, not proven; accepted as-is |
| Kata lead validates only after the fan-out; the session lead validated as agents returned | +142 | verified; host Claude Code 2.1.277 ran the session's agents in the background despite the prompt saying foreground |
| Longer lead hand-off prompts (39k vs 26k chars) | +81 | verified |
| `just check` in the foreground on cold Go caches (a fresh 441 MB go-build cache per executor HOME) | +80 | verified |
| Larger report written with `Write`, plus a closing summary | +63 | verified |
| Clone (+18) and engine setup (+9) | +27 | verified |

Ruled out: ACP transport, permission round trips (<0.1 s), the private /tmp namespace,
and the adapter version. Fixed per-call latency is identical (median 3.5 s both paths).

[^pr]: https://github.com/glim-sh/cuttle/pull/73
[^runs]: the kata run dirs on ws-pond-01
[^session]: the session-path transcripts, indexed in pond
[^forensic]: forensic comparison subagent a24cf7c09d05adc0e
[^skill]: polish SKILL.md Phase 3
[^harness]: harness investigation subagent a46d538d7967a72d0, 2026-09-19
[^judge]: blind judge subagent ac8c0138476a3399f; reports anonymized as R01-R12
[^timing]: timing forensics subagent a6f62f4994b6fc44a
