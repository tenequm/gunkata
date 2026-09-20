---
type: Reference
title: 2026-09-19 report - why a gunkata review missed a bug a session review found (cuttle PR 73)
description: Dated forensic report comparing /polish reviews of glim-sh/cuttle#73 run through the gunkata review kata and from a Claude Code session, with the same model, effort, skill and prompt; the kata miss traced to lead prompt anchoring, a checkout mutated mid-review and reasoning variance - not tools, MCP, skills, CLAUDE.md, model or effort - plus the wall-time breakdown, the harness and kata changes that followed, a 12-run blind-scored measurement, and every later setup scored against it: the review-2 DAG and its 2b (clearance audits) and 2c (hand-off fixes) variants, and the single-agent kata with a concision rule, which leads on key items at the lowest cost.
tags: [review, kata, polish, forensics, quality, acpx, claude, ground-truth]
status: stable
stale_after: "2027-03-19T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-19T15:53:34Z" }
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
  - id: k1
    resource: "K1-miss forensics subagent a7df257383b02ac26 in session 93876eaf-..., 2026-09-19; dumps in that session's scratchpad/forensic/"
    title: why two kata runs cleared K1
  - id: kata2
    resource: katas/review-2.kata.yml
    title: the review-2 kata (repo path; as of commit 9ded112)
  - id: runs2
    resource: "review-2 run dirs on ws-pond-01 under ~/.local/state/gunkata/runs/: 20260919T101131Z2028 (run 1), 20260919T104323Za16e (run 2) (record.json, gunkata.log job durations and per-turn cost, artifacts/*)"
    title: the review-2 runs
  - id: judge2
    resource: "Blind judge 2 in session 93876eaf-..., 2026-09-19; rubric.md (judge 1's key, recurring set and core-7 reused verbatim, plus calibration notes) and scores.md in that session's scratchpad/judge2/; X1 = run 2, X3 = run 1, X2/X4 = byte-identical copies of R06/R09"
    title: blind scoring of the two review-2 reports with judge calibration
  - id: r2k1
    resource: "review-2 K1 forensics in session 93876eaf-..., 2026-09-19; scratchpad/r2forensic/k1.md with scripts in k1work/, from the executor-HOME Claude Code transcripts of both runs and the 0217 and session hits"
    title: why both review-2 runs missed K1
  - id: r2time
    resource: "review-2 recall and time forensics in session 93876eaf-..., 2026-09-19; scratchpad/r2forensic/recall-time.md with cc.py, tl.py, per-job timelines tl-*.txt and 0217 extracts k0217-*.md"
    title: review-2 recall losses, variance and wall time
  - id: kata3
    resource: "katas/review-2b.kata.yml and katas/review-2c.kata.yml (repo paths, as of commit 3ac4917); the concision variant is katas/review.kata.yml plus an append_system_prompt Response Shape section, kept in session 93876eaf-...'s scratchpad/terse/review-terse.kata.yml and not committed"
    title: the review-2b, review-2c and terse katas
  - id: runs3
    resource: "run dirs on ws-pond-01 under ~/.local/state/gunkata/runs/: review-2b 20260919T111827Z9cfe and 20260919T111827Z78a9, terse 20260919T125013Z5f50 and 20260919T125015Zd2bb, review-2c 20260919T134941Zc038 (record.json, gunkata.log job durations and per-turn cost, artifacts/*)"
    title: the review-2b, terse and review-2c runs
  - id: judge3
    resource: "Blind judge 3 in session 93876eaf-..., 2026-09-19; scratchpad/judge3/scores.md, scored against judge2/rubric.md verbatim including its calibration notes, with a mid-scoring revision to recurring item l applied to all four reports it affects. Labels Y1/Y2 = the review-2b runs, Y3/Y4 = the terse runs, Y5 = the review-2c run; the mapping was withheld from the judge"
    title: blind scoring of the review-2b, terse and review-2c reports
  - id: lensab
    resource: "Lens A/B in session 93876eaf-..., 2026-09-19; scratchpad/lensab/notes.md - the cleanliness lens on byte-identical prepare artifacts, claude-sonnet-5 (3 runs) against claude-opus-5 (2 runs), with every finding verified against the checkout"
    title: the sonnet/opus cleanliness-lens A/B
  - id: lensx
    resource: "Cross-harness lens runs on ws-pond-01, 2026-09-19: 20260919T142004Zdfaf and 20260919T143630Zcb9c (codex gpt-5.6-sol at reasoning_effort medium and high), 20260919T141905Zf955 (agy gemini-3.7-flash-medium, parked on an empty message.md) and 20260919T142306Zbaa7 (the agy re-run), all on the same prepare artifacts and prompt"
    title: the codex and agy cleanliness-lens runs
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

# Why two kata runs cleared K1

Investigated on the two misses (20260919T090454Z1d23, 20260919T090634Z504d).[^k1]

- In both, the side-effect agent cleared K1 itself after reading all of `pool.go`
  (`removeProcess`, `cancelIdleLocked`, the pool-mode launch path). Its clearance rested on
  an incomplete case list: "every mover of the refcount/deadline ... re-establishes a future
  deadline" (writers only) and "`cancelIdleLocked` callers either delete the process or
  re-arm" (treating delete as final, though `getOrLaunch` deletes then relaunches under the
  same key), plus "the reap waits on the lock, so it is serialized" - the waiter's premise was
  captured before the lock holder changed the state.
- polish Phase 4 validates only positive findings, so an agent's "this holds" passed into
  "Dropped after validation" unaudited; no other agent happened to report K1.
- Hits asked what the guard does when the map entry is absent and walked
  `getOrLaunch:474 -> removeProcess -> cancelIdleLocked` concretely. In kata runs K1 came
  mostly from the design agent (4/7); in session runs from the side-effect agent (5/5).
- Generic fixes proposed, none naming K1: clearances must cite every write and delete site
  of the state the guard reads; an absent-map-entry rule; a queued-waiter rule; Phase 4
  audits clearances or labels them "cleared by agent, not validated"; a replaced guard
  needs a counterexample search against the base fix's commit message and tests; case lists
  in prompts are floors; a repro-test attempt before dropping a race.

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

# review-2: polish as gunkata jobs

`katas/review-2.kata.yml` replaces the single lead with polish's phases as jobs:
`prepare` (Setup, Phases 1-2, base check; writes `pr.md`, `checks.md`, `diff.patch`,
`changed-files.txt`, `context.md`, `base.md`, facts only) -> four parallel lens jobs
(cleanliness, design, efficiency, side-effects; polish's checklists verbatim) -> `report`
(Phases 4-5). Each later job `cp -a`s prepare's checkout into its own working directory, and
each job's final message is its output (`message.md`).[^kata2] The two measured runs predate
that last change: run 1 (`20260919T101131Z2028`) gave the lenses prepare's checkout read-only,
run 2 (`20260919T104323Za16e`) the per-job copies; both wrote `findings.md`/`report.md`.[^runs2]

| Run | K1 | K2 | K3 | Core-7 | Recurring | Findings | Wall | Cost |
|---|---|---|---|---|---|---|---|---|
| review-2 run 1 | no (cleared) | yes (exact, main finding) | yes | 5/7 | 18/25 | 26 | 1899 s | $31.54 |
| review-2 run 2 | no (dropped) | yes (exact, numbered but filed as pre-existing follow-up; broader path as main) | yes | 5/7 | 13/25 | 18 | 1731 s | $29.25 |
| review-2 (2), vs the 12-run table | 0/2 | 2/2 | 2/2 | 71% | 62% | - | 1815 s median | - |

Per job, run 1 / run 2 (s): prepare 780/670, cleanliness 408/306, design 505/435, efficiency
443/575, side-effects 714/601, report 402/458.[^runs2] Scores are judge 2's calibrated grades,
blind to which run produced which report.[^judge2]

- **Judge calibration.** Judge 2 also scored byte-identical copies of R06 and R09 from the
  12-run table. Calibrated (strict e/f, K2 graded where the finding is numbered) it matched
  judge 1 on 6/6 key-item grades and on recurring counts within 2/25 (R09 exact, R06 16-18 vs
  16); raw, 5/6 and +2. The rows above are therefore comparable to the 12-run table at +-2 on
  recurring.[^judge2]
- **Quality: no gain, not proven worse.** Recurring 18 and 13 sit inside the single-agent
  kata range (14-19) and the session range (13-22); n=2. Same model, same checklists, same
  four samples and the same raw volume (27-28 raw lens findings, as in kata run 0217), so the
  DAG changed plumbing and added no coverage. The two runs share most misses (union
  19/25).[^r2time]
- **Cost: ~1.6x wall and ~1.7x dollars** against the single-lead kata run 0217 (1140 s,
  $17.8), with 30% more output tokens (215-226k vs 166k).[^r2time]

## Why both runs missed K1

Ranked; evidence vs inference as marked. Thinking was redacted in every transcript, so
"reasoning" means the written clearance, tool order and per-turn tokens.[^r2k1]

1. **The base fix's rationale was filtered out (evidence of loss; weight inferred, strong).**
   #93's commit body (`c667868`) describes K1 almost verbatim ("`getOrLaunch` relaunches it.
   The queued reap then finds the *new* instance"). prepare read it in both runs and wrote only
   its mechanism into `base.md` (timer-identity parameter, new test); a search of both
   `base.md` files for "Dead browser|fresh one|relaunch" finds nothing. The prepare prompt's
   "never write suspected findings, failure scenarios, worked traces" rule caught an
   author-written scenario. Every hit anywhere framed K1 as "#93 covers this, the PR's guard
   does not"; every miss treated the two guards as equivalent ("Both are defensible").
2. **No lens-specific brief (evidence of the prompt difference; weight inferred).** Hit
   prompts in the session runs and in kata 0217 were lead-written after reading the code:
   they named `getOrLaunch` as a racer against the timer, timer identity as a gate, and told
   the agent to verify the author's pool-mode claim. review-2 lenses get the generic
   checklist (payments, middleware `next()`) plus a facts-only `context.md`; the no-hints
   rule forbids exactly that steering.
3. **An author correctness claim sat under a do-not-flag heading (evidence of placement and
   echoes; causality inferred).** Both `context.md` files filed "pool mode unchanged, per the
   author" under "known-intentional patterns"; run 1's side-effects and efficiency lenses
   echo "Pool mode is untouched" in their clearances.
4. **Incomplete clearances pass unaudited (evidence).** Every miss cleared with a case list
   that omitted the relaunch-delete path: deadline-present only (run 1 side-effects,
   efficiency, design); disconnect-and-reschedule only plus a session-only "missing deadline
   is recreated" rule applied to pool mode (run 2 side-effects); a nil-instance guard
   misapplied (run 2 design #9, which noticed K1's exact predicate and cleared it). Same
   shape as the 090454/090634 misses.
5. **Report validation is one-directional (evidence).** Run 2's report re-tested positive
   findings with scratch tests, then dropped design #9 on the lens's own word as refactor
   churn ("walked the reachable paths (as the Design agent did)").
6. **Base comparison late or absent (evidence of timing).** Hits read the base's competing
   fix within ~40 s; misses read it after their main reasoning block (run 2 side-effects) or
   never (run 1 side-effects, efficiency).
7. **One read gap (evidence).** Run 2 design never opened `pool.go` 700-1000, so never saw
   `removeProcess` calling `cancelIdleLocked`.

Ruled out: budget (misses spent 430-710 s and 33-52k output tokens vs session hits 246-324 s),
code visibility (all but run 2 design had `removeProcess` and the `:474` call in their tool
results), test ability (run 1 read-only and run 2 with repro tests both missed), model and
effort (all `claude-opus-5`, high). No single lens is reliable: in kata 0217 design hit K1
while its side-effect agent called the PR's guard "a genuine improvement over the base".[^r2k1]

## Recall and time beyond K1

Ranked causes.[^r2time]

1. **Per-lens sampling variance (dominant).** The same lens prompt raised different items
   across runs (run 1 design raised h and m, run 2 design did not; run 1 efficiency raised p
   and t, run 2 raised e instead).
2. **Report merge and drop losses (verified).** Run 2 kept 18 of 27 raw (6 dropped, 3
   merged): e and v were raised by lenses and lost when folded into an umbrella "merge state"
   finding; r (mark spoofable) was dropped by validation judgment in both runs (0/2 vs 6/12
   earlier). Report drop rate varied 14% (run 1) to 22% (run 2).
3. **Unaudited lens clearances (verified).** K1, p, h and t were cleared in design #9 and in
   the lenses' "Checked" sections. p's clearance ("connect cancels the poll") contradicts
   `context.md` 3.7 ("neither connect nor disconnect is called" on the marked path), and the
   report copied the false premise into its own text.
4. **Base drift has no owner (facts verified; recall effect inferred).** `base.md` carried the
   facts for e, v and x, but no lens checklist asks for merge hazards, and in run 2 only the
   design lens opened `base.md`.
5. **Author claims framed as intentional (framing verified, effect mixed).** Run 1's
   efficiency lens still found p despite the same framing.
6. **Time: documentation workload and cold re-validation, not throughput.** Throughput is
   flat (57-82 tok/s per job), tool time negligible (1-11 s per lens). Against 0217: prepare
   +435..545 s (~60 s `just check`, 200-260 s of exploration including ~95 s of `git show` on
   each base commit, ~300 s writing and fixing 10 KB `base.md` plus 23 KB `context.md`);
   fan-out -22..+91 s, side-effects the straggler in both runs; report +134..190 s, starting
   cold and re-running reproductions the lenses had already run (329 s validating in run 2 vs
   122 s for 0217's warm lead). Inherent to the DAG: the serial hand-off, the report's cold
   start (~60-120 s), the straggler.

Ruled out: the hand-off itself losing facts (`context.md` was as pointed as 0217's lead-written
drift notes, and lenses cite its 3.x items); lens isolation from the lead's conversation
(0217's subagents also saw only a written prompt).

## Generic kata fixes proposed

None names K1 or this PR.[^r2k1][^r2time]

- **prepare:** quote base commit message bodies verbatim as recorded author text (carved out
  of the no-scenarios rule) with the tests each added; flag competing fixes where base and PR
  change the same guard differently; split "intentional choices (don't flag)" from "author
  claims about behavior (verify)"; list every read, write and delete site of state the diff
  adds or re-defines, with the concurrent actors that touch it.
- **Lenses:** a clearance must cite every write and delete site of the state a guard reads,
  the absent/zero value, and a waiter queued on a lock whose holder changes that state; a
  noticed-then-cleared divergence becomes a correctness hypothesis with the attempted
  counterexample; read the base version of every rewritten function first; every lens reads
  `base.md`, and one lens owns drift against the current base; attach reproduction evidence;
  cap "Checked" to a list.
- **Report:** audit clearances and Checked sections, not just positives, or label them
  "cleared by lens, not validated"; merge only same-mechanism, same-lines findings, keep each
  source's consequence, and publish a raw-to-final table (kept, merged-into, dropped with
  reason); re-validate only unreproduced findings.
- **Time:** move `just check` into a job parallel to the lenses that only the report needs;
  cap prepare's prose (`base.md` as conflict list plus SHA, subject, files).

These are under test as two variants: review-2b (clearance audits) and review-2c (hand-off
fixes).

## review-2b and review-2c measured

Blind judge 3 scored five more reports with judge 2's rubric verbatim, calibration notes
included, against anonymized labels Y1-Y5.[^judge3] The figures below are its calibrated
pass, so they sit on the same scale as the 12-run table at +-2 on recurring.

| Run | K1 | K2 | K3 | Core-7 | Recurring | Findings | Wall | Cost |
|---|---|---|---|---|---|---|---|---|
| review-2b run 1 | miss | miss | hit | 3/7 | 14/25 | 22 numbered + 2 follow-ups, 1 dropped | 1824 s | $32.25 |
| review-2b run 2 | partial | hit | hit | 3/7 | 12/25 | 14 numbered + 2 follow-ups, 4 dropped | 2389 s | $36.37 |
| review-2c run 1 | hit | miss | hit | 6/7 | 13/25 | 17 numbered + 2 dropped, 11 clearances | 2713 s | $38.51 |

Per job, review-2b run 1 / run 2 (s): prepare 570/733, cleanliness 429/440, design 602/425,
efficiency 647/656, side-effects 546/749, report 604/904. review-2c (s): prepare 670, checks
50, cleanliness 463, design 953, efficiency 911, side-effects 1285, report 755.[^runs3]

**review-2b = review-2 plus clearance audits.**[^kata3] Every lens gets four extra rules: a
clearance is a claim held to a finding's bar and must cite every write and every delete or
reset site as `path:line` with the search that established the list; an absent map entry,
field or record treated as meaningful state requires enumerating every path that can make it
absent; "it takes the lock, so it is safe" is not a clearance, the waiter's view after the
state changed under it must be checked; and a guard the diff removes needs a counterexample
search plus the base commit that introduced it. The report job gains a "cleared by a lens,
not validated" section.

- **It did not work.** Core-7 fell to 3/7 in both runs (review-2's own runs scored 5/7) and
  recurring to 14 and 12 of 25, the lowest of any setup measured on this PR. K1 stayed a
  miss in run 1; run 2 is a partial, and an instructive one: it names the gate
  (`pool.go:351`, an absent `idleDeadlines` entry reads as due), the crash-relaunch race and
  main's timer-identity remedy, but on the session-mode deletion path
  (`sessionIdleWaitLocked`) rather than pool mode's `removeProcess -> cancelIdleLocked` with
  no launch arm. Right gate, right race shape, right fix, wrong mode.[^judge3]
- **Neither report used the new section.** No "cleared by a lens, not validated" list appears
  in either. Adding an output slot for audited clearances did not make the report job produce
  one, so the rule bought prose in the lenses and nothing downstream.
- Both reports' own totals disagree with their own contents (run 1 says 20 against 22
  numbered findings, run 2 says 12 against 14).[^judge3]

**review-2c = review-2b plus hand-off fixes.**[^kata3] prepare now quotes each base commit's
message body verbatim in a `<commit-message>` block with the tests it added (carved out of
the no-scenarios rule as recorded author text); flags **Competing fixes** where a base commit
and the PR change the same guard differently; splits "Intentional choices (don't flag)" from
"Author claims - verify"; and emits a **State inventory** naming every read, write, delete and
reset site of the state the diff touches with the search used and the concurrent actors.
The report job owns **Base drift** as a finding category, merges only same-mechanism
same-lines findings, and must publish a raw-to-final disposition table. `just check` moved
into a `checks` job parallel to the lenses, and prepare's prose is capped to terse facts.

- **K1 came back, with the strongest evidence any report has produced**: the full
  `removeProcess -> cancelIdleLocked` deletion, `pool.go:587` arming only in session mode,
  and the absent-deadline gate, reproduced by a test that fails 3/3 at the head and passes at
  the merge base and against main's signature, which also establishes the diff as its
  origin.[^judge3]
- **K2 was lost.** The profile deletion appears only as collateral damage inside two other
  findings and once in the side-effect trace, where it is listed among the side effects the
  lens judged properly gated. The report never says a non-durable session loses its logins on
  a routine reap. Judge 3 calls it the closest a miss has come to hit* in these five.
- **It is the only report with a full raw-to-final table**, accounting for all 29 lens
  findings, plus an 11-item re-verified clearance list, and its numbered findings, stated
  total and category counts reconcile.
- **One likely false clearance, produced by the new machinery.** It clears the driver marker
  because the parameter "is dropped by `parseConnectionParams` at `http.go:256-258` so it
  never becomes a Chrome arg". That holds for the valueless spelling only; `?cuttle-driver=1`
  falls through the `default:` branch onto Chrome's argv, as another report traced. The
  categorical phrasing turns recurring item g into a false clearance, so a stricter clearance
  regime still produces confident wrong ones.
- **Time went up, not down.** Moving `just check` out cost 50 s in its own job and saved
  little (prepare 670 s against review-2b's 570 and 733), while the per-lens evidence and
  competing-fix work made the fan-out the new straggler (side-effects 1285 s). The `checks`
  job is cheap ($0.36) and its post-step asserts prepare's checkout is untouched.

## The concision test: the single-agent kata with a Response Shape rule

The unmodified `katas/review.kata.yml` plus an `append_system_prompt` carrying a Response
Shape section (shortest response that fully answers, conclusion first, expand only for
multi-step reasoning, tradeoffs, code or a correctness-changing caveat, cut restatement and
hedging), ending with a line that tells the lead to paste that same section verbatim at the
end of every agent prompt it launches. Two runs, in parallel.[^kata3][^runs3]

| Run | K1 | K2 | K3 | Core-7 | Recurring | Findings | Wall | Cost |
|---|---|---|---|---|---|---|---|---|
| terse run 1 | hit | hit | hit | 6/7 | 16/25 | 21 numbered + 3 follow-ups, 8 dropped | 992 s | $12.58 |
| terse run 2 | hit | hit* | hit | 4/7 | 14/25 | 22 numbered, 12 dropped | 1125 s | $15.83 |

- K1 2/2, K2 2/2, K3 2/2 - the only group where every run found every key item. The
  unmodified kata scored 4/5 on K1 and 4/5 on K2 over five runs; review-2, 2b and 2c together
  are 1/5 on K1.[^judge3]
- Both runs came in under the single-agent kata's 1140 s median (992 s and 1125 s) at the
  lowest cost of anything measured here, and run 1's numbered findings, stated total and
  category counts reconcile.
- One error, in run 2: it drops arm-before-re-inject as "safe here, because `idleReap` takes
  the seed lock `getOrLaunch` holds across the inject". That is on judge 1's list of minor
  errors, and the other three reports in this batch treat the same placement as a real
  defect.
- **n=2.** This is the best-performing setup measured on this PR, and that is a ranking, not
  a causal claim: the unmodified kata's own five-run group varies 4/5 on the same items, so
  two runs cannot separate the concision rule from run-to-run variance. What the two runs do
  establish is that the rule cost nothing: no key item, no recurring recall outside the
  kata's existing range, and less wall time and money than the kata without it.

## Every setup measured on this PR

| Setup | Runs | K1 | K2 | K3 | Core-7 | Recurring | Median wall | Cost |
|---|---|---|---|---|---|---|---|---|
| session subagent | 5 | 5/5 | 4/5 | 5/5 | 86% | 66% | 548 s | not metered |
| kata, single agent, adapter 0.76.0 | 5 | 4/5 | 4/5 | 5/5 | 77% | 64% | 1140 s | $17.80 (run 0217) |
| kata, single agent, adapter 0.79.0 | 2 | 1/2 | 1/2 | 2/2 | 79% | 62% | 1159 s | - |
| kata + Response Shape (terse) | 2 | 2/2 | 2/2 | 2/2 | 71% | 60% | 1059 s | $12.58, $15.83 |
| review-2 (DAG) | 2 | 0/2 | 2/2 | 2/2 | 71% | 62% | 1815 s | $31.54, $29.25 |
| review-2b (clearance audits) | 2 | 0/2 (1 partial) | 1/2 | 2/2 | 43% | 52% | 2107 s | $32.25, $36.37 |
| review-2c (hand-off fixes) | 1 | 1/1 | 0/1 | 1/1 | 86% | 52% | 2713 s | $38.51 |

- No setup has found all three key items in every run of a group except the two terse runs,
  and no setup has exceeded 66% recurring recall. The ceiling is the same everywhere; what
  moves is which items each sample happens to raise.
- The DAG variants cost 2-3x the single-lead kata in wall time and money and have not bought
  a key item back. K1 in particular is 1/5 across all three DAG variants against 12/14 across
  the four single-lead groups.
- Prompt rules aimed at a specific past miss have not transferred: clearance audits (2b)
  scored the worst core-7 of any setup, and the hand-off fixes (2c) recovered K1 while losing
  K2. Each variant moves the blind spot rather than shrinking it.
- The one structural difference that tracks K1 across all 19 runs is who writes the report:
  single-lead setups, where the reporting agent read the code itself, are 12/14; DAG setups,
  where it reads only written lens output, are 1/5. That is a correlation over four groups
  against three, not a demonstrated cause, and review-2c shows a DAG can still cross it.

## One lens, five models

Held out of the setup comparison because it measures a single job, not a review: the
cleanliness lens was re-run on byte-identical `prepare` artifacts with the prompt verbatim,
changing only the model. claude-opus-5 found 7 of the 7 verified findings twice; three
claude-sonnet-5 runs found 1 between them;[^lensab] codex gpt-5.6-sol found 1 plus one item
opus missed, the same two at `reasoning_effort` medium and at high; agy
gemini-3.7-flash-medium found none.[^lensx] Two of the three sonnet runs and the agy run
shipped a confident "no findings" over a Checked section claiming coverage their tool logs
refute, which a report job cannot distinguish from a clean bill. Details, the per-harness
traps and the machine-verifiable-Checked recommendation are in
[the lens model finding](/findings/review-lens-model-choice-and-fabricated-coverage.md); the
parked agy run in that batch exposed
[a tool completion arriving after the final message](/findings/tool-completions-can-arrive-after-the-final-message.md),
fixed in `3ac4917`.

# Proposals awaiting review

Three changes that the measurements argue for and nobody has approved yet. Each one names
what would falsify it, because each can cost more than it returns.

## Proof and disposition in the review kata (measured, rejected)

Two prompt additions to `katas/review.kata.yml`:

- Prove each correctness finding with a test that fails at the PR head and passes at the
  merge base, run it, report the outcome, delete it. A finding whose test does not fail as
  predicted is reported as unproven with what was tried, never dropped in silence. Changes
  that cannot be tested that way say so and why.
- End with a disposition table: one row per finding any agent raised, including the ones
  inside cleared or checked sections, resolved as kept, merged, dropped with a reason, or
  cleared with verified evidence. Rows must account for every agent finding.

Both come from review-2c, the only run that produced them: it proved K1 with a test failing
3/3 at the head and passing at the merge base, and was the only report whose disposition
table covered all 29 lens findings.[^judge3]

The risk is real in both directions. Writing tests costs wall time, and review-2c's report
job took 755 s. Worse, a reviewer under a proof rule can quietly downgrade a true finding it
cannot reproduce quickly, which is the same failure as a fabricated clearance with better
manners. What would falsify the change: key-item or recurring recall below the terse
baseline of 6/6 keys, core-7 6/7 and 4/7, recurring 16 and 14 at 992-1125 s,[^judge3] or
findings appearing as unproven that earlier runs reported outright.

First run, 20260919T162716Zd03a, 1445 s: both falsifiers fired.[^judge3] K2 came out the
strongest instance of any run - the failed-capture gate as the top blocking finding, proven
by a test failing at the head and passing at the merge base - and the disposition table was
the most complete of the six, with merges declared from both sides, one uncounted note
aside. But K1 was missed and actively cleared ("idleTimers/idleDeadlines are cleared
together on every reachable path", which is false on the pool-mode removal path), core-7
fell to 4/7, and the dropSeed ordering item was demoted to "reported as unproven" where
three earlier reports state it outright. Proof discipline grades findings, not clearances,
so it does not close the hole that loses K1.

The second run, 20260919T230839Zba55, 1274 s, repeated it: K1 missed again, core-7 5/7,
recurring 15 of 25, one merge item marked unproven.[^judge3] Two of two proof runs miss K1
where the terse baseline hits it two of two, at a cost of $13.56 and $18.84. The rule is
rejected and `katas/review.kata.yml` keeps its terse default unchanged. The disposition table
is the half worth salvaging - both proof runs produced the most complete accounting of raw
lens findings seen, and review-3 reconciled 71 raw findings into 42 without a proof rule at
all - so an accounting requirement without a testing requirement is the version to try if
this returns.

## Machine-verifiable Checked sections (not built)

A lens claiming coverage should be checkable against its own stream. `jobs/<job>/executor.jsonl`
records every file an executor opened and every command it ran; comparing that with
`changed-files.txt` and failing the job on a coverage claim the stream contradicts costs one
script and under a second per job, no model tokens. Two sonnet runs and two agy runs asserted
they had read every changed file while the stream shows 2 to 7 of 18 to 20 opened; this gate
catches exactly that class.[^lensab][^lensx] It proves a file was opened, not that it was read
with attention, and the kata spec has no placeholder for a job's own stream, so a post-step
reaches it by relative path until the engine grows one.

## Escalation the static DAG cannot express (built, first run parked)

The job-based review is one static pass: prepare, four lenses, one report. The polish-new
skill converges instead - after validating findings it launches a skeptic per correctness
finding to refute it, a dive per confirmed mechanism to find siblings, and a dive per
unresolved suspicion, feeding the results back until a round finds nothing new, with every
finder required to end on a non-empty "Unresolved suspicions" section.[^polishnew] None of
that was expressible in a kata, because the DAG was fixed at load time and no job could fan
out from what it found. That, rather than prompt wording, is the likeliest reason the DAG
line clears findings the single-lead runs keep: no skeptic ever attacked a clearance.

Runtime fan-out was built as PR #5: one job key, `fan-out: {items, job, max_rounds,
max_items}`, makes a job a round head whose items directory schedules N copies of a template
job, each reading its own brief through `{{item}}`, the loop ending on a round that emits
nothing or at `max_rounds`. One spec promise moved with it: the DAG is no longer fully known
at load time. `katas/review-3.kata.yml` is review-2c plus that loop.

The mechanism worked on its first live run, `20260919T174005Z92da`: `escalate/round-1` took
910 s and wrote nine briefs - six skeptics and three dives, among them
`skeptic-pool-queued-reap-kills-relaunch`, which names the exact mechanism of the key finding
this line has missed in four of five runs - and the engine ran nine `probe` instances in
parallel, keyed `probe/round-1/item-N`, each with its own job dir, HOME and output check.

The run still returned nothing. Eight probes finished in 463-1866 s; the ninth slept through
its budget and hit the 3600 s timeout, which parked the head, ended the loop and left
`report` unrun. The run metered $86.00 and truly cost about $145 once the parked hour, which
emits no `turn end`, is priced in. There is no report to judge, so the loop's effect on
recall is still unmeasured. What the run did establish is a design gap, recorded in
[a parked fan-out item throws away the whole round](/findings/a-parked-fan-out-item-throws-away-the-whole-round.md):
a round is sampled work, and one instance failing should be an outcome the next head reads,
not a dependency failure that discards the round.

Attempt 2 ran that fix plus a budget trim - a 900 s probe timeout, `max_items: 6`,
`max_rounds: 2`, and a probe prompt that names the four tools a brief may be settled with and
forbids browsers, daemons and `sleep` outright. Run `20260919T233053Z5dee` succeeded in
5141 s for $82.31 metered, against $145 for the attempt that returned nothing: two rounds of
six and five items, eleven probes, none parked, longest job the report at 1091 s. The loop
stopped at `max_rounds` with `capped: true` while round 2 was still emitting five briefs, so
it was cut off rather than converged; whatever the report scores is a floor for the shape.
The parked-instance tolerance was therefore insurance that no probe needed: the trim is what
saved the run.

Blind-judged, that report is the best of the eight scored on this PR: all three key items,
core-7 6/7, and 18 of the 25 recurring items - higher than the five-run session baseline
(16.5) and the terse kata (16 and 14). It hit K1 with the most honest evidence of any report,
stating that natural timing did not reproduce and that the window had to be widened with a
slow `running()` call, and it reconciled 71 raw lens findings into 42 numbered ones. Its one
core-7 miss is the lease re-check, which four weaker reports found. So the convergence loop
is the first job-based shape to beat a single lead with subagents, on one run, cut off at
`max_rounds` rather than converged.

# What a run costs against the subscription window

The binding limit is a rolling 5-hour window, not money. One window measured at about $152
of usage: cumulative in-window spend reached $132.50 at 16:41 while the gauge read 87% at
16:50.[^quota] Prices fitted on the 33 subagent-free jobs of the day reproduce every one of
them with zero residual: per Mtok, opus-5 $5 in, $10 cache write, $0.50 cache read, $25 out;
sonnet-5 and haiku-4.5 at two fifths and one fifth of that. All cache creation was the
one-hour kind, so published headline pricing misses opus by 165%. codex and agy runs bill
nothing against the subscription.

The constant worth remembering: an Opus executor costs $0.95 to $1.01 per wall-clock minute,
across 21 runs of every kata shape measured. A DAG does not cost more per minute; it fills
the time with more parallel streams. So one window is roughly 120 Opus executor-minutes, and
in window terms a review run is 10.4%, review-2 20.0%, review-2b 22.6%, review-2c 25.3%, an
analysis subagent 1.7%, one lens on opus 2.7% and the same lens on sonnet 0.5%.

Executors dominate but the orchestrator is not free: in the 12:40 to 17:40 window, executors
took $94.39 and the orchestrating session $38.11, which is 29% of the spend for 8% of the
output tokens. The session re-reads about 243k cached tokens per turn, so each turn costs
about 0.12% of a window before it does anything, and every large file read in the main
session raises that floor permanently.

Waste is concentrated and measurable: four runs parked at the limit on 2026-09-19 spent
$36.63 and returned nothing; a seven-way replication cost 83% of a window; a concurrent pair
of review-2b runs cost 45% in forty minutes. The standing limits this argues for live in
`/AGENTS.md` under "Quota ceiling for live runs".

Three accounting gaps stand in the way of doing this from the engine alone.[^quota] A job's
`turn end` line carries lead-only usage but a cost that includes the executor's own
subagents, so the two fields are not comparable and `record.json` carries neither; a parked
run emits no `turn end` at all, so its spend is invisible to any log-based accounting; and
executors never reach pond, because an engine-owned HOME puts their transcripts inside the
run dir. Writing per-job usage and cost, for the lead and its subagents separately, into
`record.json`, and emitting one on park and on timeout, would close the first two.

[^pr]: https://github.com/glim-sh/cuttle/pull/73
[^runs]: the kata run dirs on ws-pond-01
[^session]: the session-path transcripts, indexed in pond
[^forensic]: forensic comparison subagent a24cf7c09d05adc0e
[^skill]: polish SKILL.md Phase 3
[^harness]: harness investigation subagent a46d538d7967a72d0, 2026-09-19
[^judge]: blind judge subagent ac8c0138476a3399f; reports anonymized as R01-R12
[^timing]: timing forensics subagent a6f62f4994b6fc44a
[^k1]: K1-miss forensics subagent a7df257383b02ac26, 2026-09-19; dumps in that session's scratchpad/forensic/
[^kata2]: katas/review-2.kata.yml
[^runs2]: the review-2 run dirs on ws-pond-01
[^judge2]: blind judge 2, scratchpad/judge2/scores.md
[^r2k1]: review-2 K1 forensics, scratchpad/r2forensic/k1.md
[^r2time]: review-2 recall and time forensics, scratchpad/r2forensic/recall-time.md
[^kata3]: katas/review-2b.kata.yml, katas/review-2c.kata.yml and the uncommitted terse variant
[^runs3]: the review-2b, terse and review-2c run dirs
[^judge3]: blind judge 3, scratchpad/judge3/scores.md; reports anonymized as Y1-Y5
[^lensab]: the sonnet/opus cleanliness-lens A/B, scratchpad/lensab/notes.md
[^lensx]: the codex and agy cleanliness-lens runs
[^quota]: quota attribution subagent ab54b393202220611, 2026-09-19; method and tables in that session's scratchpad/quota/notes.md
[^polishnew]: https://github.com/tenequm/skills/blob/main/skills/polish-new/SKILL.md
