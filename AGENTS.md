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

## Principles

The engine's design principles - verified completion, park-not-retry, process-tree
teardown - live in `docs/spec.md` under Principles.

One rule lives here: engine tests never involve a model - acpx is a stub on PATH.

## Where things are

- `docs/knowledge/` - durable project knowledge as an OKF bundle. Read `index.md` first,
  and load the okf-project-knowledge-base skill before reading or writing it. After
  substantial work, review whether a durable decision or finding should be captured. The
  index listing is generated: run `python3 scripts/kb_index.py` after any concept change
  (pre-commit enforces `--check`).
- `docs/spec.md` - the kata spec, the normative authority on the workflow file format.
  `docs/full.kata.yml` shows the complete surface.
- `katas/` - real katas. Run one with `gunkata run katas/<name>.kata.yml -p key=value`.
- `packages/gunkata/` - the Go engine (module `github.com/tenequm/gunkata`). All Go tooling
  runs from there; the `just` recipes and the lefthook Go jobs already do.

## Build discipline (MVP)

YAGNI and KISS govern every change while the MVP is being built.

- Build only what the current milestone needs. No speculative features, options, config
  knobs, interfaces, plugin points or abstraction layers "for later" - later earns them
  when a real case demands them.
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
bare exit 127.

`nix develop` provides the pinned toolchain plus pond and acpx; the executor CLIs are
host-provided and the shell reports their versions on entry. Bumping acpx means also
checking the engine's default claude adapter (`harnessAdapter` in `executor.go`).

## Quota ceiling for live runs

Every executor run and every subagent bills the same Claude subscription, whose binding
limit is a rolling 5-hour window. One window is worth roughly 120 Opus executor-minutes:
an Opus executor costs about 0.65% of a window per minute whatever the kata does, so a
`review` run is about 10% of a window and a `review-2c` run about 25%. A run that hits the
limit parks and returns nothing, having spent everything it already used. The measurements
behind these numbers are in `/docs/knowledge/references/2609-19-cuttle-pr73-review-forensics.md`.

- At most 2 executor runs at once, and a multi-job kata counts as both.
- At most 2 analysis subagents at once, and never while 2 executors run.
- Replication caps per window: 3 runs on Opus, 5 on Sonnet. Calibration and A/B work runs
  on Sonnet unless a result demands Opus.
- Read the window gauge before a batch, never after: cost data only exists afterwards. Stop
  launching executors at 60%, launch nothing longer than ten minutes above 75%, and above
  90% run the orchestrator only.
- Never leave an executor running unwatched past its expected duration, and never read
  transcripts, run logs or large files in the orchestrating session: delegate that, or
  aggregate in the shell and return numbers.
