# Review-graph port plan: what lands, in what order, what is deferred

Status: PLANNED 2026-09-17, after milestone 2 (first live review run, cuttle#62,
7/7 nodes green, 1 confirmed finding). Source of the ported mechanisms:
`~/pj/build-workflow/docs/plans/2609-11-review-v1.md` and its knowledge bundle.
Principle: build in parallel where files allow, but LAND and MEASURE one batch
at a time - each mechanism gets its own before/after number (requirement B3;
review-v1 attributed 3/14 -> 5/14 -> 8/14 to single levers only because it
moved one at a time).

## Batch A - land first (justified by our own run 1 evidence)

1. Model routing per work shape: per-node `model:` overrides in
   `graphs/review-cuttle-62.yaml`. flash-low on every role produced 1 finding;
   strongest models go to reasoning-heavy lenses and all verifies.
2. Cross-family blinded verifiers: verify nodes run a different model family
   than their lens, and their prompts drop the lens framing (self-preference
   >50%; review-v1's "13/15 confirmed with no real check" class).
3. Engine work the above requires (the only Go changes in the whole plan):
   - per-node `agent:` override (today agent is defaults-only;
     `internal/graph/graph.go` has per-node Model already);
   - `linkAuth` grows per-family credential links, MINIMAL per family
     (for pi: only `~/.pi/agent/models.json`, never the whole `~/.pi` -
     user-level skills/extensions/MCP hang off HOME and must not leak into
     bare executor homes);
   - support acpx built-in agent modes (`acpx pi ...`) alongside `--agent`.
4. Optional rider, non-gating: qwen shadow twins of each lens via pi
   (requirement B7 - data collection that never gates). Free lane:
   `litellm/qwen3.8-flash-next` (209 tok/s), `litellm/qwen3.8-27b-nvfp4`
   (78 tok/s), 262k context, zero cost.

Known traps for the implementer (from build-workflow's bundle, verified
relevant):

- acpx applies `--model` to a non-Claude ACP agent only if the agent
  advertises models in session/new; probe `acpx pi --model ...` on acpx
  0.17.0 BEFORE wiring, sessions die pre-first-turn otherwise.
- pi thinking level rides the model id suffix (`<id>:medium`), there is no
  effort knob.
- pi writes nothing to stdout until the turn ends; an empty executor.log
  mid-run is normal, size timeouts accordingly.

Measurement A: rerun cuttle#62 (pinned head 8fd8af8f), compare
findings/dispositions against run 1 (run dir 20260917T205540Z20ae).

## Batch B - author in parallel (worktree), land after Measurement A

5. Claim-vs-implementation lens + its verify node: prose (comments, docs, PR
   body) asserting what the code does not do; authority files read END TO END,
   not grepped - review-v1's biggest measured recall lever. Counter-lesson
   from their bundle: a mandatory checklist crowds out coequal duties -
   the lens brief must restate what remains in scope.
6. Executable rubric per finding: `rubric` key in the findings schema
   ("run X, expect Y"); verifiers execute it instead of re-reading prose;
   a finding with no statable rubric cannot rise above suggestion severity.

Land 5 alone, rerun, measure; then 6, rerun, measure.

## Deferred (build when a real run demands it)

- Zero-model lint node before the lenses (check-only node; engine already
  supports it - waiting for a case that needs it).
- Precision ledger across runs (append run ledger.json + metadata to a
  runs.jsonl via a small script; pays off after ~20 runs).
- pond#289 ground-truth replay - DEFERRED by operator decision 2026-09-17
  (too slow for now). The grading key is banked and SHA-pinned in
  `docs/knowledge/references/pond-pr-289-review-ground-truth.md`; replay must
  run against head 6a83a53, never the branch's current head.
- pond#197 (goose adapter, ~2k added lines) as the width pressure-test that
  should eventually force dynamic lens width (G4).
- PoC-or-demote and PR-body claim re-execution: need sandboxed execution of
  untrusted code; real design work, and review-v1's own replay showed the
  claim extractor misfiring (exit 127 miscounted as over-claiming).
- Locks 2 and 4 (pond transcript sync, failure classification/retry): design
  from real failure transcripts once review runs produce them; review-v1's
  per-run-pond-store and witness-retry decision records are ready-made input.

## Context for a fresh session

- Milestone 2 shipped and pushed: run-inputs feature
  (`--input name=path`, `{{input:<name>}}`), PATH-portable starter examples,
  `graphs/review-cuttle-62.yaml`, KB decision record on inputs. Commits
  21229ba..d4719d2 on main.
- cuttle checkout: `~/pjv/glim-sh/cuttle` detached at 8fd8af8f (PR 62 head);
  diff file regenerable via `gh pr diff 62 --repo glim-sh/cuttle`.
- Gates: `nix develop -c just check` (staged) / `just check-ci` (whole repo);
  commits must run through `nix develop -c` or the stale-shell guard fails
  them. `just corpus` spends model quota; keep it manual.
- Worktrees on this host come from the treehouse pool
  (`treehouse get --lease`), never `git worktree add`.
