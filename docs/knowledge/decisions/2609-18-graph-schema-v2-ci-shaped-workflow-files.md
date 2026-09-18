---
type: Decision
title: "Graph schema v2: CI-shaped workflow files"
description: Rework the graph YAML toward CI-workflow shape with gunkata's own semantics - agent profiles, a workflow job map, plural namespaced outputs with post-step checks, invocation params - plus the CI lessons worth borrowing and the anti-lessons that stay out.
tags: [schema, workflow, ci-lessons]
status: draft
generated: { by: claude-code/fable-5, at: "2026-09-18T18:24:48Z" }
sources:
  - id: session
    resource: "Schema design discussion, gunkata graph rework, 2026-09-18; no public record"
    title: Workflow schema design session
  - id: woodpecker
    resource: https://woodpecker-ci.org/docs/usage/workflow-syntax
    title: Woodpecker CI workflow syntax
---

# Decision

The graph YAML moves toward CI-workflow shape - gunkata's own semantics under familiar
vocabulary, explicitly NOT an adoption of the Woodpecker or GitHub Actions schemas (the
only part of those schemas that maps onto gunkata is "named steps + depends_on", which
`nodes:`/`needs:` already are).[^session]

1. **`agents:` named profiles** (harness, model, timeout, skills/mcps/env) replace the
   single `defaults:` block; a job references a profile via `agent:`. A profile is the
   executor contract factored: every addition still explicitly declared, once.
2. **`workflow:` map of jobs** replaces the `nodes:` list; `needs:` DAG edges carry over.
   yaml.v3 rejects duplicate map keys, so the map form is safe.
3. **`outputs: []`** (plural, namespaced per node under `artifacts/<node>/`) replaces the
   single `artifact`; **`post-steps`** (a list of argv commands) replaces `check` - they
   are the done-bit. Declared outputs get an implicit non-empty presence check;
   post-steps add the semantic check. Namespacing is the one structural must-fix: a
   flat artifacts dir collides the moment a matrix job's expansions write the same
   filename.
4. **`params:`** - invocation-time values (`gunkata run g.yaml -p pr=<url>`), required by
   default, `default:` makes one optional. File `inputs:` stays for actual files.

Pipelines are self-sufficient and repo-agnostic - never one-repo focused. For a review
pipeline the only input is a PR link; prompts tell the agent to discover the repo's own
verification, and no toolchain name appears in the graph. The graph carries shape, the
prompts carry judgment, params carry the target.[^session]

Workspace provisioning is handled by the engine primitive decided in
[workspace provisioning is an engine primitive](/decisions/workspace-provisioning-is-an-engine-primitive.md),
which supersedes checkout wiring inside graphs.

# Borrowed CI lessons, and the anti-lessons that stay out

Borrowed:[^woodpecker]

- **Classified retry**: `retry: {max, when: [quota, timeout, ...]}` - lock 4 in the file;
  `when:` names failure classes the transcript classifier emits, never expressions. No
  `retry:` means park on failure (the default stays).
- **`gunkata lint`** (load-time validation as a command) and **resume-from-verified**
  (`--resume` skips nodes whose outputs already verify) - engine features that never
  appear in the file.
- **Matrix fan-out**: one job body times a parameter grid; `{{matrix:focus}}` (colon
  syntax, one placeholder grammar) expands in the same single pass as the rest;
  depending on a matrix job name fans in all expansions.
- Deferred until a real graph demands them: `allow_failure`, `max_parallel`, `fail_fast`.

Anti-lessons (the scars every YAML CI system earned):

- No templating, `extends:`, includes, or expression language in the file - when graphs
  get repetitive, generate them with a real program. The file stays dumb; the engine
  gets smarter.
- No string outputs between jobs (files won), no stages (the DAG won), exactly one
  placeholder-expansion pass at one documented moment.
- Placeholders expand to PATHS only, never file contents - no syntax exists for splicing
  content into a prompt or command. Agent-facing commands are argv, no shell; shell is
  allowed only in deterministic engine-run steps, with values reaching the script only
  through `env:`.
- Secrets never appear in graph files; executor auth inherits from the environment per
  the executor contract.

# Status and landing order

The engine runtime already matches the design (DAG gating on verified evidence, bare
engine-owned HOMEs, per-node agent/model, acpx families, process-group teardown).
Missing: the schema v2 rework itself (mechanical), params (small), the workspace
subsystem (the big new build), matrix (deferrable - unroll lenses by hand), and a
`claude` family authLinks entry (probe the minimal credential set first; never the whole
`~/.claude`).

Landing order, one measured batch at a time: schema v2 first (port the cuttle-62 graph,
rerun, prove no behavior change) -> workspace -> claude family when a graph wants it ->
matrix last or never. The honest v1 cut: `agents:`, `pre/post-steps`, `outputs:`,
`retry.when`.

Open design decision: gate verdict semantics. A post-step of
`grep -qxE "PASS|FAIL"` releases lenses even when the repo's tests FAIL (verdict-exists,
not gate-passed) - likely right for review, but then the report job must consume
`{{artifact:gate/verdict}}` explicitly, or the verdict is produced and never read.

# Reference example

The sketch that anchored the discussion, kept verbatim with its `# v1` / `# later`
markers (the lock 7 growth rule governs `later`). It PREDATES the workspace primitive:
the `inputs: diff.patch` binding and the clone pre-steps in `lint-and-tests` are what
`workspace:` replaces, and `{{matrix.focus}}` was later corrected to colon syntax.[^session]

```yaml
name: review-pr

max_parallel: 4              # later - concurrency cap, resource knob
fail_fast: false             # later - one lens failing lets siblings finish

inputs:
  - diff.patch
  - repo-ref                 # file containing the clone URL + sha

agents:
  executor:
    harness: agy-acp-server
    model: gemini-3.7-flash-low
    timeout_seconds: 240
  brain:
    harness: claude-code
    model: claude-opus-5
    timeout_seconds: 600
    skills: []               # explicit additions per the executor contract
    mcps: []
    env: {}

workflow:

  lint-and-tests:
    agent: executor
    pre-steps:                              # v1 - runs inside this job's workspace
      - command: [git, clone, --depth=1, "{{input:repo-ref}}", repo]
      - command: [git, -C, repo, apply, "{{input:diff.patch}}"]
    prompt: |
      Run `just check-ci` in ./repo. Write the full output to {{output:gate.log}},
      and the single word PASS or FAIL as the only line of {{output:verdict}}.
    outputs: [gate.log, verdict]            # v1 - was `artifact`, now plural
    post-steps:                             # v1 - the done-bit; exit 0 releases dependents
      - command: [grep, -qxE, "PASS|FAIL", "{{output:verdict}}"]
    retry:                                  # v1 - lock 4 in the file: classified only
      max: 2
      when: [quota, timeout]

  lens:
    agent: brain
    needs: [lint-and-tests]
    matrix:                                 # later - expands to lens-correctness, lens-api, ...
      focus: [correctness, api-shape, error-handling]
    prompt: |
      Review {{input:diff.patch}} for {{matrix.focus}} defects only.
      Gate result: {{artifact:lint-and-tests/verdict}} (a path - read it).
      Write findings as JSON to {{output:findings.json}}, [] if none.
    outputs: [findings.json]
    post-steps:
      - command: [jq, -e, "type == \"array\"", "{{output:findings.json}}"]
    retry: {max: 1, when: [quota, timeout, refusal]}
    allow_failure: true                     # later - advisory: a dead lens doesn't block merge-report

  merge-report:
    agent: brain
    needs: [lens]                           # a matrix name means "all its expansions"
    prompt: |
      Merge every findings file under {{artifacts:lens}} into one report
      at {{output:report.md}}, deduplicated, ordered by severity.
    outputs: [report.md]
    post-steps:
      - command: [test, -s, "{{output:report.md}}"]
```

What each borrowed lesson looks like in the file:

- Classified retry is the only conditional construct anywhere; no `retry:` means park.
- Matrix replaces 3 copy-pasted lens jobs with one body; depending on the matrix job
  name fans in all expansions.
- `allow_failure` makes a lens advisory: merge-report releases even if one lens parks,
  and merges whatever findings files exist.
- Every placeholder expands to a path; all commands are argv, no shell.
- Resume and lint don't appear in the file at all - that's the point.

Renames from today's schema: `artifact` -> `outputs` (plural), `{{artifact}}` ->
`{{output:name}}` for own outputs while `{{artifact:node/file}}` refs others, `check` ->
folded into `post-steps`, `defaults` -> subsumed by `agents:` profiles. Everything else
(`needs`, `inputs`, load-time validation of every reference, KnownFields) carries over
unchanged.

[^session]: Workflow schema design session
[^woodpecker]: Woodpecker CI workflow syntax
