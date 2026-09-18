# gunkata - agent instructions

## Design intent

gunkata is a DAG orchestrator for agent executors. Work is declared as a kata - a
workflow file, spec in `docs/spec.md` - the engine schedules it, runs each job in an
isolated executor, and releases dependents only against verified evidence. It is
assembled out of blocks the operator understands and can debug end to end, after two
weeks with an engine whose out-of-the-box machinery - its own prompts, its own roles,
its own retry and grading behaviour - contaminated every run and made it impossible to
tell whether an outcome came from the work or from the harness. gunkata is generic,
flexible and unopinionated out of the box: no built-in roles, no built-in prompts, no
built-in review shape. What a run does is what the kata says it does.

## Core driver

acpx is locked as the core driver for isolated executor runs. It is the single adapter
between the engine and any executor; the engine speaks to it and to nothing else.

## Executor contract

There is exactly ONE generic executor type, usable at any step of the DAG, carrying a
per-step prompt and per-step config. There are no preconfigured typed executors - a role
such as reviewer, planner or tester is a per-step prompt, never an engine concept.

Every executor starts BARE. It inherits no skills, no agent instruction files, no configs
and no MCP servers from the outer environment. The sole inheritance is subscription
credentials and auth. Every addition is explicitly specified by the step that needs it.

## The locks

1. Completion is verified, never claimed. A task is done when its named evidence artifact
   exists and passes its check; dependents release only after verification.
2. Evidence is files and exit codes. Executors run in engine-owned HOME directories at
   predictable per-run paths; at task boundary the engine syncs the run's transcripts into
   gunkata's own dedicated local pond (its own `pond serve` over its own storage dir,
   holding run transcripts only, separate from any personal pond) and gates on the artifact
   check (the done-bit) plus transcript-informed failure classification; pond live-write is
   the upgrade path for mid-run supervision, never a v1 dependency. The engine never trusts
   a stream, a status field, or an agent's say-so.
3. Zero process-global state. Every run gets its own ports, temp dirs, config paths; no
   defaults shared between runs.
4. No retry without classification. A failure must be classified (quota / timeout /
   refusal / real defect) from transcript evidence before any retry; unclassified failures
   park the task.
5. A run ends when its process tree is dead. Teardown is part of the run contract; the
   engine owns the full tree via process groups.
6. Cost is observability, never control flow. No bound may read a dollar amount.
7. Model-free grading from day 0, one case first. The corpus starts with exactly ONE
   deterministic, unambiguous case verifiable in under 10 minutes; the engine must pass it
   before anything else is built; the corpus grows only from real cases encountered later,
   never speculatively; grader tests never involve a model.

Each lock has a decision record in `docs/knowledge/decisions/` carrying the why. Change a
lock there first, then here.

## Where things are

- `docs/knowledge/` - durable project knowledge as an OKF bundle. Read `index.md` first,
  and load the okf-project-knowledge-base skill before reading or writing it. After
  substantial work, review whether a durable decision or finding should be captured. The
  index listing is generated: run `python3 scripts/kb_index.py` after any concept change
  (pre-commit enforces `--check`).
- `docs/spec.md` - the kata spec, the normative authority on the workflow file format.
  `docs/full.kata.yml` shows the complete surface.
- `docs/2609-17-starter-corpus-case.md` - the single starter case of lock 7.
- `katas/` - real katas, not graded.
- `examples/` - the graded corpus, doubling as the usage examples. Everything in it is
  graded, so lock 7's growth rule governs the directory.
- `packages/gunkata/` - the Go engine (module `github.com/tenequm/gunkata`). All Go tooling
  runs from there; the `just` recipes and the lefthook Go jobs already do.

## Build discipline (MVP)

YAGNI and KISS govern every change while the MVP is being built.

- Build only what the current milestone needs. No speculative features, options, config
  knobs, interfaces, plugin points or abstraction layers "for later" - later earns them
  when a real case demands them (the same growth rule lock 7 applies to the corpus).
- Prefer the simplest working construction: stdlib over a dependency, a function over an
  interface, one package over three, a literal over a generic. Reach for the complex form
  only when the simple one demonstrably cannot carry the requirement.
- Every new file, dependency and exported symbol must justify its existence. If it can be
  inlined, inline it.

## Working in this repo

There are exactly two gate verbs, and nothing else needs to be guessed at:

- `just check` - staged-only, applies fixes, runs its gates in parallel. This is the
  pre-commit hook. Each staged recipe prints "nothing staged" and exits 0 when the staged
  set is empty, so a green run is never mistaken for a clean repo.
- `just check-ci` - whole repo, verify-only, mutates nothing. This is the pre-push hook
  *and* the single CI job, which run the identical command and therefore cannot drift.

Run both from a shell that has the pinned toolchain, or prefix with `nix develop -c`: a gate
inherits the caller's environment, and one cached before a `flake.nix` change fails with a
bare exit 127. `just corpus` is in neither gate - it spends real model quota.

`nix develop` provides the pinned toolchain plus pond; acpx and the executor CLIs are
host-provided and the shell reports their versions on entry.
