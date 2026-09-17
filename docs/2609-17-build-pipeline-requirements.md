# Build pipeline - requirements

Requirements and goals only. No design, no tool names, no model names. Where the operator
stated a requirement in his own words, his words are kept verbatim and attributed in the
Provenance section. Quotes that named a tool or a model carry `[tool name removed]` and
`[model name removed]` in place of the name.

## Framing

The build pipeline is the staged path from an idea, or from a plan someone already has, to
an implementation that has been verified. The stages are separately invocable, the
artifacts they produce are on disk in a known layout, and each stage gates the next.

## Goals

### B1. Wall time is the primary metric; cost is secondary

The optimization target is wall time from zero to completion, with quality of the end
result held. Cost is not a constraint on the pipeline.

> "Look. Cost is not a concern for us. We get all those models from subscription. The most
> crucial and valuable metric for us is wall time from 0 to completion. Can you deeply think
> through the situation we had here. Don't gloss over, don't just purely trust what
> subagents may have told you as they may have their own bias. Use common sense and your own
> senior engineer professional judgement. What could be the best possible combination using
> any of the aforementioned models available for us to get that wall time to completion
> lower with ensuring we get a high quality result in the end?"

### B2. The process is redesigned from the ground, not tuned

The pipeline is a ground-up redesign that combines the best features of each available
model and provider, rather than an incremental adjustment to an existing sequential
workflow.

> "can you think of creative ways how we could rethin our process from the ground to achieve
> quality-rich results faster by combining best features from each model/provider we could
> run through [tool name removed]?"

### B3. A review workflow of the same shape, with benchmarking lanes and a scoring gate

The pipeline carries a review stage built like the operator's existing review workflows.
Benchmarking lanes are kept deliberately, to find out empirically whether a technique
improves outcomes, and an automated scorer gates progress.

> "If we would have tried to combine that into a workflow similar to our polish or
> polish-new skills, how would it look like high level? For 3 - i'd like to keep that, just
> for our own becnhmarking purposes too, curious to see if that actually improves any
> outcomes. For 6 - it promises a significant cut in time, right? If so, how could we
> get/setup that scorer right so that it would do its job well?"

### B4. DAG orchestration with no vendor or harness lock-in, and our own reviewers

Orchestration is a DAG, not a sequence, and not a feature of one harness. The reviewers are
ours, running our workflow; a vendor's own reviewer as the cross-family judge is lock-in and
is refused.

> "No [tool name removed] dynamic workflows pls. I want something not locked on one harness
> feature. aren't there any popular, modern, dag solution for such case on github? ... 'The
> vendors' own reviewers as the cross-family judges.' lock on vendor again. i'd prefere
> having our own reviewers ran through [tool name removed] with our own workflow like
> polish."

### B5. Take design ideas, do not fork or rewrite

Prior art is read for its design and architecture and adapted. It is not forked and not
rewritten. A project being young is irrelevant; the design patterns and the engineering
quality are what matter.

> "I was thinking more about analyzing [tool name removed] and picking the
> design/architecture ideas from it and adapting for our usecase, while i don't want to
> fork/rewrite it, i'd rather try to see if there is anything to grab inspiration from ...
> and the fact that those projects are young are not important for me at all. i care only
> about the design patterns and good engineering."

### B6. Lite, minimal operational complexity

The first version is the smallest thing that captures most of the impact. One adapter for
uniformity rather than several; nothing added because it might be needed.

> "Lets not overcomplicate things. Lets do only [tool name removed] adapter for uniformity
> now, trying to do [tool name removed] too - would only complicate things. Okay, lets try to
> gather all our valuable findings, and try to combine that into our prototype of our
> workflow that is lite and achieves most of impact with least efforts spent. How would it
> look like?"

### B7. A shadow lane that gathers data without gating

A lightweight shadow execution lane runs alongside the real work purely to accumulate
evidence, to be evaluated later. It is a source of empirical data and of errors discovered
in the background, and it does not block the pipeline.

> "Why not keep best-of-N in some lite form, like to run [model name removed] alongside each
> [model name removed]/[model name removed] run just to aggregate the data and then evaluate
> it some time later to see what comes out of that? and why drop 'the AVO discover stream'"

### B8. The operator can see his own flow

The path from an idea, or from a ready plan, through to verified implementation is
explainable to the operator as a flow he moves through, not as internal machinery.

> "Okay, can you in simple way explain me with diagrams in your response on how my workflow
> would look for me as a user whenever I start working on some ready to use plan or
> preparing the plan and then implementing it?"

### B9. Plan readiness is verified, and stages are separately invocable

A plan is checked for completeness and readiness before execution, and each stage of the
workflow can be invoked on its own.

> "How to make sure that plan is complete and ready for execution to get quality rich
> results? could we for both entries have like some skills that i use to invoke the
> workflows steps/stages?"

### B10. Design specification and execution plan are separate artifacts

The design specification and the execution plan are distinct stages producing distinct
artifacts. The specification is a real, stable, committed spec, not a document rewritten on
every run.

> "why have /build-design and /build-plan as separate steps?"

> "can DESIGN.md stay in [a stable committed location] and be committed in repo and be as a
> real spec and not something changed often?"

> "could we rename DESIGN.md to docs/spec.md ?"

### B11. Multiple concurrent plans, in a deterministic layout

Several plans can be in flight at once, and the layout on disk says which artifacts belong
to which plan and which run. All pipeline artifacts live under one agreed directory, with
plans and runs each in their own subdirectory.

> "Okay, about one directory. Could we agree on putting all in [one directory]? I mean all
> the artifacts that would be stored and worked with along the implementation and different
> stages?"

> "why current? what if we run multiple plans?"

### B12. Generic across projects; agents adapt

The pipeline is not written against one project. It is generic enough that agents adapt it
to the repository they are in.

> "you understand that [the framework] is not designed to work only in [one project]? it
> should be generic enough! and agents should be able to adapt. check how /polish-new and
> /polish are written"

## Provenance

- Primary source: session `35c0a261-0fc0-4b21-9e11-a5751711d5e8`, 2026-09-02 08:02:38Z to
  13:26:38Z. First statement of the build pipeline requirements, preceding the creation of
  the first implementation repository at 13:05Z the same day. Contains the metric ranking
  (B1), the architectural choices (B4, B7) and the staged workflow vision (B8 to B11).
  Confidence: high. B1 through B11 are quoted from it; B12 is quoted from the same session
  thread on 2026-09-03 16:26:15Z.
- Not found: any standalone written specification of the stages predating that session. The
  stages emerged directly from the design dialogue, so there is no earlier document to
  reconcile against.
- Not found: an explicit scoring function for the scorer of B3 predating the 2026-09-02
  literature and evaluation analysis. B3 therefore states the requirement for a scoring gate
  and stops there; the form of the score is not a recovered requirement.
- Two quotes under B10 and B11 have had a concrete path or tool name replaced with a
  description in square brackets, because the original wording named the prior
  implementation's directories. The requirement is the separation and the determinism, not
  the specific paths.
