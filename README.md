# gunkata

A DAG orchestrator for agent executors. Work is declared as a kata - a DAG of jobs; each
job runs in an isolated, bare executor, and a dependent is released only when the
upstream job's declared outputs exist and its post-steps pass. Generic and unopinionated
out of the box: no built-in roles, no built-in prompts.

**Status: the engine runs kata spec v1** (`docs/spec.md`).

```sh
gunkata run katas/review.kata.yml -p pr=https://github.com/<owner>/<repo>/pull/<n>
```

- [AGENTS.md](AGENTS.md) - design intent and the executor contract.
- [docs/spec.md](docs/spec.md) - the kata spec; [docs/full.kata.yml](docs/full.kata.yml)
  shows the complete surface.
- [katas/](katas/) - real katas.
- [docs/knowledge/](docs/knowledge/index.md) - durable findings and references.
- [packages/gunkata/](packages/gunkata/) - the Go engine; all Go tooling runs from there.

`nix develop` gives the pinned toolchain; `just check` is the full gate.
