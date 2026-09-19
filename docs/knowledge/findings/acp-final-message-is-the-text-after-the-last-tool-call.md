---
type: Finding
title: An ACP agent's final message is its message-chunk text after the last tool call
description: In the ACP stream a turn's text arrives as agent_message_chunk updates interleaved with tool calls - short preambles before each tool call, the answer after the last one - so the final message is the chunk text accumulated since the last tool_call or tool_call_update, not all of the turn's text and not one messageId.
tags: [acpx, acp, executor, stream, message]
status: stable
stale_after: "2027-03-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-19T10:30:00Z" }
sources:
  - id: review
    resource: "gunkata review-kata run 20260919T090446Z0217 on ws-pond-01, jobs/review/executor.jsonl: claude-opus-5 via acpx 0.17.0, 1145 lines"
    title: Review-kata executor stream
  - id: live
    resource: "Live gunkata runs 20260919T102303Z11c1 and 20260919T102332Z1524 on ws-pond-01, 2026-09-19: claude-haiku-4-5 via acpx 0.17.0, a prompt asking for text, one ls tool call, then a two-line answer"
    title: Live final-message runs
  - id: engine
    resource: packages/gunkata/internal/engine/executor.go (eventStream.update, appendMessage)
    title: gunkata's final-message capture
---

# Finding

A Claude executor's turn streams its text as `session/update` notifications with
`sessionUpdate: agent_message_chunk`, each carrying `content: {type: text, text: ...}`,
a few characters to a few words per chunk.[^live] Text is interleaved with tool calls: in
a review run, nine separate text runs appeared - eight short preambles of 1 to 19 chunks,
each followed by a tool call, and last, 236 chunks after the final `tool_call_update`
that were the whole report.[^review]

So "the agent's answer" is not all of the turn's message text. It is the text since the
last `tool_call` or `tool_call_update`. Live, a prompt that asked for "Starting now." before
an `ls` produced that sentence as its own chunk before the tool call; the text after the
tool's `completed` update was exactly the requested two lines.[^live]

Details that break naive parsers:

- **`messageId` does not separate thought from answer.** `agent_thought_chunk` updates
  carry the same `messageId` as the message chunks that follow them (one API message); the
  update type does. Grouping by `messageId` alone would pull thinking into the answer.
- **`content` changes shape by update type**: an object on message chunks, an array of
  content blocks on `tool_call_update`. A decoder must not type it as one struct.
- **No trailing newline**: the final chunk ends where the model stopped, so the
  accumulated text usually lacks one.
- `usage_update` lines arrive between message chunks and do not end the message.

gunkata resets its buffer on every `tool_call` and `tool_call_update`, appends each
`text`-typed message chunk, and writes the result to `artifacts/<job>/message.md` when the
executor ends.[^engine] A turn whose last event is a tool call leaves it empty.

[^review]: Review-kata executor stream
[^live]: Live final-message runs
[^engine]: gunkata's final-message capture
