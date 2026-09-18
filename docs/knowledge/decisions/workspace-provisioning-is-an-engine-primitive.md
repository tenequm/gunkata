---
type: Decision
title: Workspace provisioning is an engine primitive
description: The graph declares a workspace source from a closed vocabulary and the engine provisions it - resolved once per run, materialized as a credential-free local clone per node - because deterministic, exit-code-verifiable mechanics may live in the engine while judgment never does.
tags: [schema, workspace, isolation, executor-contract]
status: draft
generated: { by: claude-code/fable-5, at: "2026-09-18T14:38:09Z" }
sources:
  - id: session
    resource: "Schema design discussion, gunkata graph rework, 2026-09-18; no public record"
    title: Workflow schema design session
  - id: woodpecker
    resource: https://woodpecker-ci.org/docs/usage/workflow-syntax
    title: Woodpecker CI workflow syntax (clone, skip_clone)
  - id: gitclone
    resource: https://git-scm.com/docs/git-clone
    title: git-clone(1), local clone hardlink behavior
  - id: contract
    resource: /AGENTS.md
    title: gunkata agent instructions, "Executor contract" and "The locks"
---

# Decision

A graph may declare a `workspace:` at run level or per node, from a closed vocabulary of
three forms: `pr: <url>` (clone, fetch the pull head, switch to it), `repo: <url>` +
`ref: <sha|branch>` (plain checkout), or `from: <node>` (adopt a command node's directory
output - the escape hatch for anything the vocabulary cannot say). The engine provisions it;
no checkout node and no artifact wiring are needed. When a run-level workspace is declared,
every node receives it unless the node says `workspace: false`; a graph with no `workspace:`
block behaves as before - a bare, empty engine-owned HOME.[^session]

The line this decision sits on, stated as the general law because the next "should this be
built in?" question needs a test rather than a precedent: **the engine may own deterministic
mechanics whose success is verifiable by exit codes; it may never own judgment** - prompts,
roles, grading, or inference about the repo such as guessing a base branch. Workspace
provisioning is the first application. The engine verifies its own mechanics (clone
succeeded, SHA matches) and never inspects, interprets, or describes the checkout to the
agent; what happens inside the workspace remains entirely the prompt's business.[^session]

# Why

Every YAML CI system converged on the same answer: clone is built in and overridable
(Woodpecker and Drone's implicit clone step with `skip_clone`, GitLab's automatic checkout),
because one hundred percent of pipelines needed it and hand-rolled checkout was a top source
of flaky pipeline YAML.[^woodpecker] gunkata's executors run agents against repositories;
the same universality applies, and building it in does not breach the no-opinions design
intent because provisioning is mechanics, not judgment.

**Resolved once, pinned for the run.** The engine resolves the declared source to a SHA at
run start; every node - gates, lenses, retries - checks out that SHA. This closes a real
race: a push to the PR mid-run would otherwise have lenses reviewing different code than the
gate tested. The resolved SHA, source URL, and provisioning exit codes are recorded in the
run dir as `workspace.lock`, which is lock 2 evidence and what makes resume
coherent.[^contract]

**One mirror per run, one local clone per node - explicitly not `git worktree`.** The
engine clones the remote once into the run dir, then materializes each node's copy with a
local `git clone` from that mirror. Local clones hardlink objects, so N nodes cost N
checkouts of working files, not N network clones, and each node's repository is fully
independent - commits, branches, even `git gc` cannot cross nodes.[^gitclone] `git worktree`
is rejected because worktrees share refs and repository metadata between checkouts, which is
exactly the shared mutable state lock 3 bans.[^contract]

**The auth boundary falls out of the mirror design.** Only the engine-side mirror clone
touches the remote, using the operator's ambient git credentials; every node-side clone is
local and needs no credentials at all. Private-repo access therefore never enters an
executor HOME, and the bare-executor contract holds for private repos by construction. A
private-repo failure is fixed at the engine's environment, never by injecting a token into a
node's HOME.[^contract]

**Teardown adds no surface.** Mirror and per-node clones live inside the run dir, so they
die with the run under lock 5 and leave no cross-run residue under lock 3.[^contract]

# Consequences

- The review-pipeline graph shrinks by a checkout node and all its artifact wiring; a graph
  plus `-p pr=<url>` runs against any repository with zero setup nodes.
- `workspace.lock` joins artifacts and transcripts as run evidence; resume and retry key off
  its pinned SHA.
- Executor HOMEs remain credential-free for repo access; only subscription auth is inherited,
  as the executor contract already requires.
- Explicitly deferred until a real graph demands them (the lock 7 growth rule): cross-run
  mirror caching, non-git workspace sources, and any vocabulary beyond the three forms.
- This is the engine's first built-in behavior; anything proposed as the second must pass
  the mechanics-not-judgment test above.
