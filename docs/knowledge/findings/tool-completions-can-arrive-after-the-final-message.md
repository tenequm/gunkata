---
type: Finding
title: A tool completion can arrive after the agent's final message
description: agy batches its tool_call_update completions at the end of the turn, so the rule that any tool update resets the captured message threw away whole reviews and parked the job with an empty message.md; the engine now resets only on a tool start or a non-terminal update.
tags: [acpx, acp, agy, executor, stream, message, engine]
status: stable
stale_after: "2027-03-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-19T15:53:34Z" }
sources:
  - id: parked
    resource: "gunkata run 20260919T141905Zf955 on ws-pond-01, 2026-09-19: agy gemini-3.7-flash-medium on the review-2 cleanliness prompt, parked after 125 s with failure \"output message.md is missing or empty\" and a 0-byte artifacts/cleanliness/message.md, its gunkata.log showing the full tool sequence"
    title: The parked agy lens run
  - id: rerun
    resource: "gunkata run 20260919T142306Zbaa7 on ws-pond-01, 2026-09-19: the identical kata re-run, succeeded in 69 s with a 7252-byte message.md"
    title: The agy re-run that captured its message
  - id: fix
    resource: "gunkata commit 3ac4917 \"fix(engine): keep the final message when tool completions arrive after it\", packages/gunkata/internal/engine/executor.go (eventStream.update, maybeEnd) and engine_test.go"
    title: The engine fix
---

# Finding

gunkata captures a job's output by accumulating `agent_message_chunk` text and resetting the
buffer on every tool event, because
[the final message is the text after the last tool call](/findings/acp-final-message-is-the-text-after-the-last-tool-call.md).
That rule assumes a tool's terminal `tool_call_update` arrives before the text that follows
it. **agy breaks the assumption: it batches its tool completions at the end of the turn.**

An agy lens job wrote a complete review and then parked, with the engine reporting "output
message.md is missing or empty" over a 0-byte artifact after 125 s of real work.[^parked]
The `completed` updates for tools that had already run landed after the last message chunk,
each one resetting the buffer, so nothing survived to be written. The failure is silent in
the sense that matters: the job ran, the model answered, and the engine saw an empty turn.
An identical re-run minutes later, still before the fix, captured its 7252-byte message
normally,[^rerun] so the loss depends on where the batch lands in the stream and cannot be
ruled out by one green run.

The fix separates the two cases.[^fix] `maybeEnd` now reports whether the update it logged
reached a terminal status (`completed` or `failed`), and `update` resets the buffer only when
it did not:

```go
case updateToolUpdate:
	if !e.maybeEnd(u, e.track(u)) {
		e.message.Reset()
	}
```

A `tool_call` (a tool starting) still resets unconditionally: text before a tool starts is a
preamble. A non-terminal `tool_call_update` still resets, for the same reason. Only a
completion is now treated as possibly retrospective. The engine's stub-acpx test covers the
ordering, per the rule that engine tests never involve a model.

Why it was not caught earlier: the claude and codex streams never showed it. Both interleave
their completions before the text that follows, so the original rule held for two harness
families and broke on the third. Expect the same shape from any future adapter: the ACP
stream does not promise that a tool's terminal update precedes the text written after that
tool ran. See also
[the agy ACP server's other bare-start quirks](/findings/agy-acp-server-replaces-its-token-link-and-stays-bare.md).

[^parked]: The parked agy lens run
[^rerun]: The agy re-run that captured its message
[^fix]: The engine fix
