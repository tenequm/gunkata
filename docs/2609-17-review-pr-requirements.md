# Pull request review - requirements

Requirements and goals only. No design, no tool names, no model names. Where the operator
stated a requirement in his own words, his words are kept verbatim and attributed in the
Provenance section.

## Framing

A pull request review is a DAG with a predefined action graph. What gets reviewed, by how
many independent lenses, in what order, and under what gate is decided by the structure of
the graph, not by an agent's ad-hoc thinking at runtime. The quality ideal is the output of
the operator's `/polish` skill: the bar is what a good `/polish` run produces, and a review
that produces less has failed regardless of how it ran.

## Goals

### G1. One sweep, not best effort

The review must catch the issues systematically in one converging process. Repeated runs
finding fresh issues each time is the defect to eliminate, not a property to live with.

> "The core questions I'm trying to answer here now: Why every @skills/polish/SKILL.md
> polish skill run keeps finding so much issues? Why aren't all of them caught in the first
> pass of it? What does our polish skill lack to make really bulletproof and
> one-sweep-catch-all-issues one, not try-your-best-at-catching-most-issues?"

### G2. Findings are grounded in evidence, never in an agent's report

A finding is believed because of what it points at, not because a reviewing agent asserted
it. Verification reads the actual artifacts rather than trusting the summary handed up.

> "Once the agents mine the session polish runs - read yourself the actual polish runs and
> subagents running in them transcripts. Don't just trust what [model name removed]
> subagents return to you, they tend to be misinterpreting facts a lot."

### G3. Wrong findings must not corrupt the verdict

Reviewer findings are frequently wrong, and a wrong finding that reaches the evaluator
produces a wrong judgement. The review must be constructed so that an incorrect finding is
caught before it decides anything.

### G4. Review depth scales with the diff

The amount of independent review applied is a function of the size and complexity of the
change, so that a large diff does not lose anything important to a fixed-width process.

> "From my experience it is typically hard to trust what subagents find, and their findings
> may be wrong and provoking wrong judgements of evaluator, how to avoid that? And should
> we make the amount of subagents launched more dynamic based on the size of the diff to
> accommodate bigger diffs and be sure not to loose anything important?"

### G5. Zero dropped findings, across context loss

A persistent reconciliation ledger records every finding and its disposition, so that
nothing is lost to compaction or to a long run. Reviewer outputs are retained in it so a
finding can be re-verified later against what actually produced it.

> "Regarding that, I have an idea. check @skills/pre-compact/SKILL.md skill. I had came up
> with a handoff structure to preserve artifacts. Would it be a good idea to try to come up
> with something like this here, to get a full reconciliatioin ledger there and maybe as
> well have the subagent outputs to be put there too just in case reverification may be
> needed?"

### G6. Deeper investigation is driven by trouble found, not by a budget

When the review finds trouble, it is able to expand and go deeper. Depth is not capped by a
token budget, and a question that an earlier stage could have resolved is resolved there
rather than deferred to a later dive. Quality is the priority; a small fixed budget per
reviewing agent is not.

> "i don't like the outcome here. how could we try to make the skill's workflow more
> dynamic able to absorb needed deeper dive-ins by allowing it to scale and run more
> subagents when necessary?"

> "i don't want to defer to dives something that could have been resolved earlier by design
> step subagent. Why not keep feedback loop tighter?"

> "it doesn't feel too good, you sure there is not better pattern? and i don't want to stick
> to strict small token budget subagent runs, its not a priority, quality is a priority."

### G7. Lean by mechanism, not by prohibition

Quality comes from the structure of the process, not from accumulated prose rules. Adding
text is a failure mode; a requirement that can be enforced by a mechanism is not written as
an instruction.

> "Please don't flood me with text. Explain each suggestion. And my goal is not to flood it
> with more text. My goal is to review it, and to think of how we could make it leaner
> having less instructions if thats applicable here while still ensuring the quality
> standards of its outputs delivered."

### G8. Harness independence

The review must not depend on the existence of any particular tool. It must be runnable by
any harness capable of running independent workers.

> "Lets make sure that skill stays generalized and doesn't include dependency on [tool name
> removed] or other specific tools existence, so that it could be run by different agentic
> harnesses which are capable of handling subagents. Would that be hard?"

### G9. Lenses are partitioned by the shape of what they look for

How review work is split across independent lenses follows the nature of the concern.
Efficiency and other cross-cutting concerns follow call paths across the change and are not
partitioned per file; only genuinely localized checks may be.

> "'efficiency shard by files (mostly local)' - thats surely wrong. think more."

### G10. Fixes are applied without churn or loss

The step that applies fixes must not lose findings and must not thrash over the same code.
Every accepted finding ends in an applied fix or an explicit, recorded decision not to fix.

> "and check /polish-new skill, maybe it is worth to use some techniques from it so that all
> fixes would be applied and not get lost as they do usually. Tell me if we should actually
> use any techniques from it or not."

## Provenance

- Primary source: session `23c4112b-78fd-4397-be8a-3da67ff37414`, 2026-08-27 10:55:45Z to
  13:58:46Z. First statement of the review ideal, the failure forensics behind it, and the
  structural mechanics that replaced prose rules with artifact-enforced convergence loops.
  Confidence: high. G1, G2, G3, G4, G5, G6, G7, G8 and G9 are quoted from it.
- Supporting: session `9cc91b43-eaec-4d65-8e82-ebe9f581eda2`, 2026-08-31. Source of G10,
  from applying fix-churn mechanics during a live delivery.
- Supporting: session `db1e2a06-1e7e-4e80-8c36-203171aa75be`, 2026-09-01. Practical
  evaluation of multi-lens review and fix validation on live infrastructure.
- Supporting: session `14f591a7-8a97-4d59-9263-359a9a5ee986`, 2026-09-03. Refinement of
  review stage decoupling so the pipeline runs self-contained.
- Not found: transcript records for the first generation of the review skill prior to July
  2026; stored session records begin in July 2026 because of an earlier schema boundary. Any
  requirement from that period is not recoverable and is deliberately absent here.
- The DAG and predefined-action-graph framing at the top of this document is the operator's
  instruction for gunkata, carried in from the build pipeline requirements (see
  `2609-17-build-pipeline-requirements.md`, B4); it is not quoted from the review session.
