---
type: Reference
title: Executor models field guide
description: What each executor family advertises over ACP, what has actually been measured about those models, and the traps that make a wrong choice look like an infrastructure fault - the answer to "which model goes on this node".
tags: [executors, models, acpx, agy, codex, pi, routing]
status: stable
stale_after: "2026-12-17T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-17T23:31:48Z" }
sources:
  - id: graph
    resource: ../../../graphs/review-cuttle-62.yaml
    title: The cuttle#62 review graph, whose header records the routing and its reasons
  - id: exec
    resource: ../../../packages/gunkata/internal/engine/executor.go
    title: authLinks - the per-family credential inheritance a bare executor home gets
  - id: run
    resource: "gunkata run 20260917T230923Z209c on this host, 2026-09-17: the cuttle#62 review graph with three agy lenses, three codex verifies and three pi shadow twins"
    title: The run that produced the ACP-advertisement mismatch and the shadow timeouts
  - id: probe
    resource: "Bare-home acpx probes on this host, 2026-09-17, acpx 0.17.0: codex with only ~/.codex/auth.json linked, pi with only ~/.pi/agent/models.json linked"
    title: Per-family bare-home startup probes
  - id: agycli
    resource: "`agy models` on this host, 2026-09-17, against acpx 0.17.0"
    title: The CLI's own model listing
  - id: piconf
    resource: "Read of ~/.pi/agent/models.json on this host, 2026-09-17 (provider metadata only, no key material)"
    title: The pi litellm provider and its three qwen ids
  - id: floor
    resource: "~/pj/build-workflow/docs/knowledge/findings/executor-model-capability-floor.md (separate repository), from operator builds of 2026-09-10"
    title: An executor model below the brief's complexity fails as a clean exit
  - id: lanes
    resource: "~/pj/build-workflow/docs/knowledge/references/review-model-lanes-2609-11.md (separate repository), survey of 2026-09-11"
    title: The 2609-11 model-lane survey - qwen throughput and the gemini burst limiter
  - id: pilane
    resource: "~/pj/build-workflow/docs/knowledge/findings/pi-lane-config-rides-the-model-id.md (separate repository)"
    title: On the pi lane thinking level rides the model id suffix
  - id: pisilence
    resource: "~/pj/build-workflow/docs/knowledge/findings/pi-agents-die-quiet-under-bernstein-heartbeats.md (separate repository), measured on pi 0.85.1"
    title: pi writes nothing to a redirected stdout until the turn ends
  - id: advert
    resource: "~/pj/build-workflow/docs/knowledge/findings/acpx-model-flag-needs-advertised-models.md (separate repository), measured against acpx 0.15.1"
    title: acpx applies --model only to a model the agent advertised
  - id: hung
    resource: "~/pj/build-workflow/docs/knowledge/findings/a-hung-model-lane-fails-as-a-dead-agent.md (separate repository)"
    title: A spent or hung lane surfaces as an agent that produced nothing
  - id: filter
    resource: "~/pj/build-workflow/docs/knowledge/findings/codex-content-filter-truncates-executor-turns.md (separate repository)"
    title: Codex turns can be truncated by its content filter and still exit 0
  - id: codexeffort
    resource: "~/pj/build-workflow/docs/knowledge/findings/codex-effort-is-per-invocation.md (separate repository), measured against codex-cli 0.154.0"
    title: codex has no effort flag; the value is a config override, and a typo downgrades it silently
  - id: acpxfaq
    resource: "~/.claude/skills/acpx-faq/SKILL.md, the agy section (host skill, not in this repository)"
    title: The agy lane's default model and its measured standing against Opus
---

# What a node actually selects

A node carries `agent` and `model` and nothing else about the model.[^graph] `agent`
is either an acpx built-in mode (`acpx:pi`, `acpx:codex`) or a path to an ACP server
command, which in practice is the agy server. The engine starts every executor in a
bare home and links exactly one family's credentials into it: agy gets the
antigravity-acp settings, token and server directory; pi gets `~/.pi/agent/models.json`
alone; codex gets `~/.codex/auth.json` alone.[^exec] Nothing else - no skills, no agent
instruction files, no MCP config, and no model or effort configuration file - crosses
that boundary. Whatever the model id does not encode, the node cannot ask for.

# Catalogs, as advertised over ACP

The ACP `session/new` advertisement is the authority acpx enforces: `--model` is
applied only to an id the agent advertised, and a bogus id kills the session before the
first turn.[^advert] A CLI's own model listing is a different authority and can be
wider.

**agy (Antigravity ACP server).** Advertises `gemini-3.8-flash-{high,medium,low}`,
`gemini-3.7-flash-{high,medium,low}`, `gemini-3.6-flash-{high,medium,low}`,
`gemini-pro-agent` and `gemini-3.1-pro-low`. It does NOT advertise
`gemini-3.1-pro-high`, although `agy models` lists it (alongside
`claude-sonnet-4-6`, `claude-opus-4-6-thinking` and `gpt-oss-120b-medium`, none of
which the ACP list carries either).[^run][^agycli] Effort rides the id suffix; there is
no separate knob.[^acpxfaq]

**codex (`acpx:codex`).** From a bare home with only `~/.codex/auth.json` linked it
advertises `gpt-6-astra`, `gpt-5.6-sol`, `gpt-5.6-terra`, `gpt-5.6-luna` and `gpt-5.5`,
and runs real turns on `gpt-5.6-sol`.[^probe] Codex ids carry no effort suffix, and the
only carrier for codex effort is a config override that neither the node schema nor the
bare home can supply[^codexeffort][^exec] - a codex node runs at the family default.

**pi (`acpx:pi`).** Honors `--model`; it advertises its models in `session/new`, and a
bogus id fails loudly, listing the three ids it has.[^probe] Those come from the single
linked `models.json`: `litellm/qwen3.8-flash-next`, `litellm/qwen3.8-27b-nvfp4`,
`litellm/qwen3.6-35b-a3b-nvfp4`.[^piconf] Thinking level rides the id as `<id>:<level>`;
any separate effort key is silently dropped.[^pilane]

# Measured observations, each with its scope

- **A too-weak executor model does not error.** On one frozen multi-step build brief
  (about 6k characters, with an allowlist, a validation command and a report contract),
  `gpt-5.6-luna` failed every attempt while `gpt-5.6-terra` and Claude Sonnet 5
  delivered: it wandered off the brief, wrote its report outside the declared path, and
  exited 0 with the work uncommitted. A direct one-line probe with the same model in
  the same layout committed fine - the difference was the size of the instruction.
  Prefer the strongest affordable model on executor roles.[^floor]
- **gemini-3.7-flash-medium is strong on rubric-driven bulk work.** Measured on par
  with Opus for that shape of task, and far faster; `gemini-3.8-flash-high` measured
  *worse* on the same task.[^acpxfaq] Scope: one rubric-driven bulk-reading task, not a
  general ranking - the newer, higher-effort id is not automatically the better one.
- **gemini-3.8-flash-high is the current pick for reasoning-heavy lenses here.** After
  a run that put `gemini-3.7-flash-low` on every role and produced one finding, the two
  reasoning-heavy lenses moved to `gemini-3.8-flash-high`, chosen because the ACP lane
  does not advertise `gemini-3.1-pro-high`.[^graph][^run] Scope: a routing choice on the
  cuttle#62 review graph, not a measured comparison - it sits in open tension with the
  bulk-work measurement above, which is why both are recorded.
- **The qwen lane reasons slowly despite its throughput.** Survey throughput was
  `qwen3.8-flash-next` 209 tok/s, `qwen3.8-27b-nvfp4` 78 tok/s,
  `qwen3.6-35b-a3b-nvfp4` 54 tok/s, with no metered spend and no rate limit
  observed.[^lanes] On a full review-lens brief, though, `qwen3.8-flash-next` finished
  1 of 3 twins inside 600s and parked the other two; the timeouts moved to
  900s.[^run][^graph] Tokens per second is not time to a usable answer.
- **The qwen lane is not local on this host.** The 2609-11 survey called it local-first
  with prompts that never leave the machine; the `models.json` this host links into
  executor homes points its litellm provider at a remote HTTPS gateway.[^piconf] Where
  they disagree, this host's config wins: treat the qwen lane as free, not as
  private.
- **Cross-family verification.** The verify nodes deliberately run a different model
  family (codex `gpt-5.6-sol`) from every lens, so no verifier rules on its own output;
  their prompts are blinded.[^graph] Scope: a design rule applied here, carried over
  from self-preference being reported above 50% in the literature, not measured in this
  repository.

# Traps

- **Capability floor reads as an infrastructure fault.** Sandbox-rejection lines, a
  sparse worktree and a missing commit all point at the harness; a repeated "exited 0,
  nothing delivered" cycle is a capability signal first. Swap the model and change
  nothing else before investigating the environment - it is one run.[^floor]
- **The CLI list is not the ACP list.** Sizing a node from `agy models` yields ids that
  fail at `session/new`; a parked-before-first-turn node is the signature.[^run][^advert]
  Check the advertisement, not the CLI.
- **The gemini flash constraint is a 5-hour burst limiter, not quota exhaustion.** It
  produced a 429 storm under high concurrency, so concurrency is a dial to ramp, never
  a place to start high.[^lanes]
- **pi is silent mid-run.** It writes nothing to a redirected stdout until the turn
  ends, so an empty `executor.log` halfway through a pi node is normal, not a
  hang.[^pisilence]
- **A spent or refused lane looks like a dead agent.** Some CLIs hang on a non-fatal
  refusal (quota, billing) and produce zero tokens and a zero-byte log, which every
  layer above reports as something else. A one-word one-shot probe of the lane surfaces
  the refusal immediately, and belongs before any run whose failure would be expensive
  to misread.[^hung]
- **`Failed to spawn agent command: npx pi-acp` can mean a bad `--cwd`.** The error
  names the agent command; the cause can be a `--cwd` that does not exist.[^run]
- **Codex turns can be truncated by a content filter and still exit 0.** Benign
  vocabulary (race, sweep, exploit, attack) in a prompt or a filename has ended turns
  mid-task with a normal-looking completion. Prefer neutral phrasing in prompts where it
  costs nothing, and keep the artifact check as the thing that decides.[^filter]
- **Timeouts are per work shape, not per graph.** 600s fits a flash lens; a slow lane or
  a whole-repository sweep needs 900s or more.[^graph][^run]

# Routing heuristic

| Node does this | Put this on it | Why |
|---|---|---|
| Reasoning-heavy read of a diff or a hunk | agy `gemini-3.8-flash-high` | Newest generation at its highest advertised effort; `gemini-3.1-pro-high` is not advertised over ACP[^graph][^run] |
| Rubric-driven bulk reading, breadth over depth | agy `gemini-3.7-flash-medium` | Measured on par with Opus on that shape and far faster[^acpxfaq][^graph] |
| A merge or a mechanical reconcile, not a judgement | agy `gemini-3.7-flash-medium` | Cheap shape; depth buys nothing[^graph] |
| Verify or judge another node's output | codex `gpt-5.6-sol` | Different family from every lens, so nothing rules on its own output[^graph] |
| Multi-step executor work against a long brief | The strongest model the budget allows | Below the floor the failure is a clean exit 0, not an error[^floor] |
| Free-lane shadow, data only, nothing depends on it | pi `litellm/qwen3.8-flash-next`, timeout 900s+ | No metered spend, but slow to a usable answer and not private[^run][^piconf] |

[^graph]: the cuttle#62 review graph and its routing header
[^exec]: authLinks, the per-family credential inheritance
[^run]: gunkata run 20260917T230923Z209c
[^probe]: bare-home acpx probes, 2026-09-17
[^agycli]: `agy models` on this host
[^piconf]: the pi litellm provider and its three qwen ids
[^floor]: the executor-model capability floor
[^lanes]: the 2609-11 model-lane survey
[^pilane]: pi thinking level rides the model id
[^pisilence]: pi writes nothing until the turn ends
[^advert]: acpx applies --model only to an advertised model
[^hung]: a hung lane surfaces as a dead agent
[^filter]: the codex content filter truncates turns
[^codexeffort]: codex effort is a config override, not a flag
[^acpxfaq]: the acpx-faq agy section
