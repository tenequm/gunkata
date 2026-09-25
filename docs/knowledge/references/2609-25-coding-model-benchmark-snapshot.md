---
type: Reference
title: 2026-09-25 snapshot - coding-agent model benchmarks, cost and time to resolution
description: Dated snapshot of Terminal-Bench 4.0/3.0/2.1 standings and the Artificial Analysis Coding Agent and Intelligence indexes, with per-task cost and wall time, for choosing which model and harness runs an executor node - GPT-6 Sol (max) in Codex is the cost-speed-capability sweet spot, Opus 5.5 the capability ceiling.
tags: [executors, models, routing, benchmarks, terminal-bench, artificial-analysis, cost]
status: stable
stale_after: "2026-11-25T00:00:00Z"
generated: { by: claude-code/opus-5-5, at: "2026-09-25T13:25:00Z" }
sources:
  - id: tb4
    resource: https://www.tbench.ai/leaderboard/terminal-bench/4.0
    title: Official Terminal-Bench 4.0 leaderboard (27 entries, board data updated 2026-09-21; parsed from the page's embedded JSON)
  - id: tbfamily
    resource: https://www.tbench.ai/benchmarks
    title: The Terminal-Bench family - 4.0 (2026-08-28) is the newest version
  - id: continuous
    resource: https://www.tbench.ai/news/continuous-benchmarks
    title: Continuous Benchmarks - leaderboards are upgraded to new dataset versions by reusing, regrading or rerunning trials
  - id: tb3
    resource: https://tokencost.app/blog/terminal-bench-3-cost-per-solved-task
    title: Terminal-Bench 3.0 results from the underlying run data (DeepSeek V4.1 Flash from codingfleet.com/blog/terminal-bench-3-leaderboard-2026)
  - id: tb21
    resource: https://benchlm.ai/benchmarks/valsterminalbench21
    title: Terminal-Bench 2.1 as run by Vals in one standardized harness (61 models, 2026-09-22)
  - id: opus55
    resource: https://anthropic.com/claude-opus-5-5
    title: Claude Opus 5.5 launch post - Terminal-Bench 4.0 66.4%, run by Anthropic itself
  - id: aacoding
    resource: https://artificialanalysis.ai/agents/coding
    title: Artificial Analysis Coding Agent Index (DeepSWE v1.1 + Terminal-Bench 4 + SWE-Atlas-QnA, each model in its native harness; parsed from the page's embedded JSON, materialized 2026-09-25)
  - id: aaintel
    resource: https://artificialanalysis.ai/leaderboards/models
    title: Artificial Analysis Intelligence Index v4.3 leaderboard (269 models, cost per index task)
  - id: aasol
    resource: https://artificialanalysis.ai/articles/gpt-6-sol-and-luna-push-the-cost-efficiency-frontier
    title: Artificial Analysis on GPT-6 Sol and Luna (2026-09-22)
  - id: supercode
    resource: https://supercode.sh/en/blog/guides/gpt-6-sol-luna-codex
    title: GPT-6 Sol and Luna vs GPT-5.6 and Opus 5.5 - prices, FrontierCode comparison
  - id: vanja
    resource: https://vanja.io/gpt-6-sol-luna
    title: GPT-6 Sol and Luna vs Astra - Codex credit ratios, messages per window, Astra safety monitoring
  - id: gemini
    resource: https://deepmind.google/models/model-cards/gemini-3-8-flash
    title: Gemini 3.8 Flash model card
  - id: ocgo
    resource: https://opencode.ai/docs/go
    title: OpenCode Go model list and per-model monthly limits
---

# Scope

A point-in-time answer to "which model and harness goes on an executor node", taken
2026-09-25 while routing a multi-lane plan across Claude Max, ChatGPT Pro (Codex), a
Grok subscription and OpenCode Go. Numbers are vendor or third-party benchmarks, not
measurements on our work; the measured routing evidence lives in
[the executor models field guide](/references/executor-models-field-guide.md), which
this snapshot complements. Re-check after any model release - the frontier moved three
times in the week this was taken.

# Terminal-Bench versions

4.0 (2026-08-28) is the newest; siblings (Terminal-Bench-Science 0.1, Challenges,
Harbor-Index) are separate benchmarks, not successors.[^tbfamily] 4.0 is a maintenance
release of 3.0 (66 of its 74 tasks, 20 revised), and leaderboards are migrated forward by
regrading or rerunning trials, so 3.0 is effectively frozen at mid-August.[^continuous]
2.1 is still run by third parties but saturated - the top ten sit within 9 points in the
78-87% band, and vendor self-reports exceed 90% - so it no longer separates frontier
models.[^tb21] Only 4.0 discriminates.

# Combined top 10 (by Terminal-Bench 4.0)

Time and cost columns are from the official 4.0 run data (66 tasks x 5 trials): average
trial wall time including failures, total time divided by successes, and cost divided
by successes.[^tb4] Where two efforts tie on score, the faster config is shown.

| Model (best 4.0 config) | TB 4.0 | TB 3.0 | TB 2.1 (Vals) | Avg trial | Time per solve | Cost per solve |
|---|---|---|---|---|---|---|
| Claude Opus 5.5 (Claude Code) | 66.4% (self-run, not on board)[^opus55] | - | - | n/a | n/a | n/a |
| GPT-6 Astra (max, Codex) | #1 58.2% | - | #1 87.3% | 47m | 80m | $17 |
| Fable 5.1 (xhigh, Claude Code) | #2 57.9% | - | #3 85.0% | 53m | 92m | $26 |
| Opus 5 (xhigh, Claude Code) | #3 53.9% | #1 42.7% | #4 84.6% | 75m | 139m | $34 |
| Fable 5 (max, Claude Code) | #4 44.5% | #3 34.1% | #7 80.5% | 70m | 157m | $49 |
| GLM-5.3 (max, Claude Code) | #5 41.8% | #5 28.3% | #17 71.5% | 97m | 232m | $20 |
| Grok 4.7 (xhigh, Grok Build) | #6 37.6% | - | - | 95m | 253m | $30 |
| GPT-5.6 Sol (max, Codex) | #7 37.3% | #2 34.6% | #2 85.8% | 40m | 107m | $21 |
| Opus 4.8 (max, Claude Code) | #8 23.6% | #7 21.1% | #16 71.9% | 86m | 364m | $83 |
| GPT-5.6 Terra (max, Codex) | #9 21.5% | #8 20.8% | #11 77.5% | 42m | 195m | $24 |
| Grok 4.6 (high, Grok Build) | #10 20.3% | #6 26.5% | #9 78.3% | 39m | 193m | $54 |

3.0 and 2.1 sources: [^tb3] [^tb21]. GPT-6 Sol and Luna (released 2026-09-22) are not on
the official 4.0 board; Artificial Analysis measures them at roughly 43% and 13% in its
own harness, which is not comparable to the table.[^aasol] GPT-6 Astra scores flat from
high to max (57.9-58.2%), so `high` buys the same result for about 30% less.[^tb4]

# Artificial Analysis Coding Agent Index

Each model in its native harness; per-task means, cost at API list price.[^aacoding]

| Harness - model | Index | Cost/task | Time/task |
|---|---|---|---|
| Claude Code - Opus 5.5 (max) | 66.0 | $13.04 | 64m |
| Claude Code - Fable 5.1 (max) | 62.2 | $12.39 | 35m |
| Codex - GPT-6 Astra (max) | 61.6 | $7.47 | 29m |
| Claude Code - Opus 5 (max) | 59.7 | $10.79 | 42m |
| Codex - GPT-6 Sol (max) | 56.7 | $2.99 | 22m |
| Grok Build - Grok 4.7 (xhigh) | 56.3 | $8.82 | 39m |
| Codex - GPT-5.6 Sol (max) | 54.6 | $6.35 | 21m |
| Muse Code - Muse Spark 1.3 (max) | 54.3 | $3.98 | 18m |
| OpenCode - GLM-5.3 (max) | 53.6 | $4.24 | 48m |
| Kimi Code CLI - Kimi K3 | 51.9 | $5.05 | 61m |
| Codex - DeepSeek V4 Pro (max) | 43.1 | $0.24 | 40m |
| Antigravity - Gemini 3.8 Flash (high) | 41.9 | $2.47 | 12m |
| Codex - GPT-6 Luna (max) | 41.1 | $0.18 | 21m |

The general Intelligence Index agrees on order (Opus 5.5 58, Fable 5.1 and Astra 53,
Sol 48, Grok 4.7 46) and shows effort is a strong lever on Claude: Opus 5.5 at `high`
scores 54 at $1.82 per task against 58 at $5.98 at `max`.[^aaintel]

# Reading it for routing

- **Sweet spot: GPT-6 Sol (max) in Codex** - cheapest and among the fastest above an
  index of 50, about 9 points under Opus 5.5 at under a quarter of the cost and a third
  of the time.[^aacoding] Suited to tightly specified, test-gated work.
- **Step up: GPT-6 Astra in Codex** - within 4.4 points of Opus 5.5 at 57% of the cost
  and 45% of the time.[^aacoding] Costs 5x Sol per token in Codex credits, and runs under
  an asynchronous safety monitor that can end a headless Codex CLI task outright.[^vanja]
- **Ceiling: Opus 5.5** - leads every board it appears on; on FrontierCode (mergeable
  patches) it beats Sol by 3-10 points at matched effort.[^supercode] Use `high` rather
  than `max` where the last few points do not matter.[^aaintel]
- **Cheap tier for checkable slices:** GPT-6 Luna and DeepSeek V4 Pro at $0.18-0.24 per
  task, index 41-43 - only where a command or schema proves the result.[^aacoding]
- **Dominated:** Grok 4.7 (Sol's index at 3x the cost and nearly 2x the time), GLM-5.3
  and Kimi K3 (lower than Sol, dearer and 48-61 minutes per task).[^aacoding] Gemini 3.8
  Flash is fast (288 tok/s) and strong on DeepSWE (73.7%) but weak at long terminal work
  (19.1% on Terminal-Bench 4.0).[^gemini]
- **OpenCode Go** caps every strong model at $15 per month (GLM-5.3, Kimi K3, Qwen3.8
  Max); its best Terminal-Bench model is Grok 4.7.[^ocgo] Not a lane for sustained work.

[^tb4]: Official Terminal-Bench 4.0 leaderboard
[^tbfamily]: The Terminal-Bench family
[^continuous]: Continuous Benchmarks
[^tb3]: Terminal-Bench 3.0 results
[^tb21]: Terminal-Bench 2.1 (Vals)
[^opus55]: Claude Opus 5.5 launch post
[^aacoding]: Artificial Analysis Coding Agent Index
[^aaintel]: Artificial Analysis Intelligence Index
[^aasol]: Artificial Analysis on GPT-6 Sol and Luna
[^supercode]: GPT-6 Sol and Luna vs Opus 5.5
[^vanja]: GPT-6 Sol and Luna vs Astra
[^gemini]: Gemini 3.8 Flash model card
[^ocgo]: OpenCode Go
