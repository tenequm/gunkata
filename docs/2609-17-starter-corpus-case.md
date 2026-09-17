# Starter corpus case

Lock 7 says the corpus starts with exactly one deterministic case that the engine must pass
before anything else is built. This is that case. It is synthetic on purpose: it exercises
the engine, not a model. It is the only case the corpus is allowed to contain until a real
case forces a second one.

## What it proves

That the engine can schedule a graph, run an executor in isolation, verify completion from
evidence rather than from a claim, release a dependent only after that verification, and
park a task whose evidence is missing instead of letting the graph proceed.

## Shape

A three-node DAG.

- **produce** - one generic executor, given a per-step prompt, writes a named artifact to a
  path the graph declared. Nothing about the executor's report is consulted.
- **gate** - depends on `produce`. Verifies the artifact: it exists at the declared path and
  passes its declared check. The check is a command with an exit code; the exit code is the
  verdict. No model is involved.
- **consume** - depends on `gate`. Reads the artifact and produces a second artifact derived
  from it, with its own check. Its release proves that dependents wait on verification and
  not on the upstream task merely finishing.

## The two variants

Both variants run the same graph. They differ only in the step prompt given to `produce`.

**Pass variant.** `produce` writes the artifact as specified. `gate` passes. `consume` runs
and its own check passes. Expected outcome: all three nodes complete, run succeeds.

**Fail variant.** `produce` is instructed to do something that leaves the evidence absent or
wrong - the artifact is not written at the declared path, or it is written with content its
check rejects. Expected outcome: `gate` fails, `consume` never starts, and the task is
**parked**, not retried. Parking rather than retrying is the assertion: lock 4 forbids a
retry without a classification, and a missing artifact on its own is not a classification.

The fail variant is the load-bearing half. An engine that passes only the pass variant has
proven nothing about verification, because a graph that never checks anything also passes
it.

## Grading

Model-free, per lock 7. The grader asserts, from files and exit codes only:

- which nodes reached completion and which did not;
- that `consume` did not start in the fail variant;
- that the fail variant ended in a parked state rather than a retried or failed-and-swept
  one;
- that the engine's own record of each node's outcome matches what is on disk.

The grader is covered by ordinary unit tests with no engine and no model in the loop, so
that a disagreement between the grader and the engine fails in milliseconds rather than
costing a run.

## Budget

Deterministic and fast: target under two minutes wall time for both variants together, well
inside lock 7's ten-minute ceiling. If the case cannot be made to run in that budget, the
problem is the case, not the budget.

## Growth rule

The corpus grows only from real cases encountered later - a failure the engine actually had,
reduced to a case. It never grows speculatively, and no case is added because it seems like
something that should be covered.
