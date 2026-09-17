# gunkata

A DAG orchestrator for agent executors. Work is a graph of steps; each step runs in an
isolated, bare executor, and a dependent is released only when the upstream step's named
evidence artifact exists and passes its check. Generic and unopinionated out of the box:
no built-in roles, no built-in prompts.

**Status: milestone 2 passed.** On top of the milestone-1 engine, grader and starter
corpus, the first real review graph ran end to end: a 7-node lens/verify/reconcile DAG
reviewed a live pull request (glim-sh/cuttle#62), every node released against its jq
evidence check, and the reconciled ledger came out with every finding carrying a
disposition. Run inputs (`--input name=path`, `{{input:<name>}}`) landed to feed the diff
in; see `graphs/review-cuttle-62.yaml`.

- [AGENTS.md](AGENTS.md) - design intent, the executor contract, and the seven locks.
- [docs/](docs/) - requirement documents and the starter corpus case.
- [docs/knowledge/](docs/knowledge/index.md) - durable decisions and findings.
- [examples/](examples/) - the graded corpus, doubling as the usage examples.
- [packages/gunkata/](packages/gunkata/) - the Go engine; all Go tooling runs from there.

`nix develop` gives the pinned toolchain; `just check` is the full gate.
