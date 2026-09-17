# gunkata

A DAG orchestrator for agent executors. Work is a graph of steps; each step runs in an
isolated, bare executor, and a dependent is released only when the upstream step's named
evidence artifact exists and passes its check. Generic and unopinionated out of the box:
no built-in roles, no built-in prompts.

**Status: milestone 1 passed.** The engine, the model-free grader and the starter corpus
case are built, and the live graded run passes both variants - the pass graph completes and
the fail graph parks at its gate without releasing dependents.

- [AGENTS.md](AGENTS.md) - design intent, the executor contract, and the seven locks.
- [docs/](docs/) - requirement documents and the starter corpus case.
- [docs/knowledge/](docs/knowledge/index.md) - durable decisions and findings.
- [examples/](examples/) - the graded corpus, doubling as the usage examples.
- [packages/gunkata/](packages/gunkata/) - the Go engine; all Go tooling runs from there.

`nix develop` gives the pinned toolchain; `just check` is the full gate.
