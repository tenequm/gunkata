# gunkata

A DAG orchestrator for agent executors. Work is a graph of steps; each step runs in an
isolated, bare executor, and a dependent is released only when the upstream step's named
evidence artifact exists and passes its check. Generic and unopinionated out of the box:
no built-in roles, no built-in prompts.

**Status: milestone 2 passed, first measured review lever landed.** On top of the
milestone-1 engine, grader and starter corpus, a lens/verify/reconcile DAG reviews a live
pull request (glim-sh/cuttle#62) end to end: every node releases against its jq evidence
check, and the reconciled ledger carries every finding with a disposition. Run inputs
(`--input name=path`, `{{input:<name>}}`) feed the diff in; see
`graphs/review-cuttle-62.yaml`.

The graph now routes models per work shape, runs each verify in a different model family
than the lens it checks with the lens framing stripped out, and carries non-gating shadow
lenses on a free lane for comparison data. Measured against the flat-model baseline, that
took the run from one finding nobody re-derived to three findings that survive a hostile
re-read, with half the raw findings rejected on evidence. The staging plan and every
measurement live in `docs/plans/2609-17-review-graph-port-plan.md`.

- [AGENTS.md](AGENTS.md) - design intent, the executor contract, and the seven locks.
- [docs/](docs/) - requirement documents and the starter corpus case.
- [docs/knowledge/](docs/knowledge/index.md) - durable decisions and findings.
- [examples/](examples/) - the graded corpus, doubling as the usage examples.
- [packages/gunkata/](packages/gunkata/) - the Go engine; all Go tooling runs from there.

`nix develop` gives the pinned toolchain; `just check` is the full gate.
