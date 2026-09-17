---
type: Decision
title: Graphs declare input names; runs bind the files
description: A graph lists the input names it needs and {{input:<name>}} references them; the concrete file arrives only at invocation via --input name=path, so a graph file never carries a host path.
tags: [engine, graph-format, portability]
status: stable
generated: { by: claude-code/fable-5, at: "2026-09-17T21:05:00Z" }
sources:
  - id: starter-wart
    resource: git history of examples/starter (hard-coded agy path, fixed in commit ad2ac31)
    title: the starter examples' portability defect
  - id: impl
    resource: packages/gunkata/internal/graph/graph.go
    title: inputs validation and {{input:...}} expansion
---

# Decision

External files enter a run in two separated halves: the graph declares a list
of input *names* (`inputs:`), and the invocation binds each name to a concrete
file (`gunkata run --input name=path`). Prompts and checks reference inputs
only through `{{input:<name>}}`, which expands to the engine-owned copy at
`<runDir>/inputs/<name>`.

Why this shape and not paths in the YAML: the first version of the starter
examples hard-coded an absolute agent path and was thereby unrunnable on any
other host - a graph that embeds a source path repeats that defect for every
input.[^starter-wart] Declaring names keeps the graph file host-independent
and reviewable, while the binding travels with the invocation, which is the
thing that is host-specific anyway.

Consequences carried by the implementation:[^impl]

- Binding is validated before any node starts: every declared name bound
  exactly once, no undeclared bindings, every source an existing non-empty
  regular file. A binding failure is a run *error*, never a parked node.
- Sources are copied byte-for-byte into `<runDir>/inputs/` (never symlinked),
  so the run directory stays a self-contained evidence record even if the
  source later changes (lock 2).
- `artifacts/` remains exclusively node-produced evidence; inputs live in a
  sibling directory so the grader's assumptions about artifacts are untouched.
- `record.json` records the name-to-source-path map for provenance.

[^starter-wart]: the starter examples' portability defect
[^impl]: inputs validation and {{input:...}} expansion
