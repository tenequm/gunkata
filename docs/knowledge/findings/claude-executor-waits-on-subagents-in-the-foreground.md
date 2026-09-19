---
type: Finding
title: A Claude executor waits on its subagents reliably only in the foreground
description: In a single-prompt acpx run, background subagents' results can be lost when the lead ends its turn, and Monitor is absent because CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC turns off the feature flags that enable it; foreground Agent calls in one message, or TaskOutput with block, are the reliable waits.
tags: [executor, claude, acpx, subagents, monitor]
status: stable
stale_after: "2026-12-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-19T06:00:00Z" }
sources:
  - id: forensics
    resource: "Review run 20260919T022043Z1b56 on this host: lead transcript under jobs/review/home/.claude/projects"
    title: Lead that launched four background agents and waited with sleep 120/180/240
  - id: live
    resource: "Live acpx probes on this host, 2026-09-19: acpx 0.17.0, claude-agent-acp 0.76.0 (Claude Code 2.1.257) and 0.79.0 (2.1.274) via --agent, claude-haiku-4-5, bare HOME"
    title: Probes toggling one variable at a time and timing background subagents
  - id: binary
    resource: "Claude Code 2.1.257 binary bundled by @anthropic-ai/claude-agent-sdk-linux-x64 0.3.257"
    title: Monitor tool definition and GrowthBook gate
  - id: adapter
    resource: https://github.com/agentclientprotocol/claude-agent-acp/blob/main/src/acp-agent.ts
    title: claude-agent-acp Turn.deferredSettle - holding a turn for its background subagents
---

# Monitor is off because of the nonessential-traffic switch

Monitor's `isEnabled` is the GrowthBook flag `tengu_amber_sentinel`, default false.[^binary]
`CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` - and `DISABLE_TELEMETRY=1` likewise - turns
GrowthBook off, so the flag keeps its default and Monitor never reaches the deferred-tool
list. With every other bare-start variable kept and only that one removed, a bare HOME gets
Monitor (plus DesignSync and PushNotification) on the first run; with it set, neither
adapter 0.76.0 nor 0.79.0 offers Monitor. The Claude Code version, the adapter and
non-interactive mode are not the cause.[^live]

Monitor would not fix waiting anyway: it streams events as notifications, which land at the
same turn boundaries as task notifications - it is not a blocking wait.

# Ending the turn does not reliably wait for background subagents

claude-agent-acp holds `session/prompt` open while subagents the turn spawned are live and
settles it at the followup's result.[^adapter] One background subagent, or three with
staggered finishes, are delivered inside the same acpx run. Four finishing within about two
seconds of each other are not: the first followup's result settles the turn and acpx exits
with results still owed - 3 of 3 runs on 0.76.0 and 2 of 3 on 0.79.0 lost at least one
agent.[^live] The adapter documents this as an accepted residual.[^adapter] The lead in the
review run had also not ended its turn: it slept blind for up to four minutes while its
agents' completion notices queued behind the sleep.[^forensics] This corrects the
"ending the turn ends the job" reading in
[the cuttle PR 73 review forensics](/references/2609-19-cuttle-pr73-review-forensics.md):
the adapter does hold the turn, just not reliably.

# What does wait

Four foreground Agent calls in one message run in parallel and return when the last
finishes; four background agents followed by TaskOutput with `block: true` on each id also
returned all four. Both work on 0.76.0 with the bare-start environment unchanged.[^live]
Since the engine has no built-in prompts, a kata that fans out subagents says so in its
prompt.

[^forensics]: Review run 20260919T022043Z1b56 lead transcript
[^live]: Live acpx probes, 2026-09-19
[^binary]: Claude Code 2.1.257 binary
[^adapter]: claude-agent-acp acp-agent.ts
