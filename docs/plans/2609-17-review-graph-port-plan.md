# Review-graph port plan: what lands, in what order, what is deferred

Status: Batch A LANDED and MEASURED 2026-09-17. The lens set is being replaced
wholesale with the four `/polish` lenses (PR #3, open). Batch B is authored and
parked (PR #1, open). Source of the ported mechanisms:
`~/pj/build-workflow/docs/plans/2609-11-review-v1.md` and its knowledge bundle.
Principle: build in parallel where files allow, but LAND and MEASURE one batch
at a time - each mechanism gets its own before/after number (requirement B3;
review-v1 attributed 3/14 -> 5/14 -> 8/14 to single levers only because it
moved one at a time).

## Batch A - LANDED (PR #2, commits 77c7897 + 188d74b, merged as 5877e55)

1. Model routing per work shape: per-node `model:` overrides. Final routing
   after the operator's correction (commit 6339a85): reasoning-heavy lenses on
   `gemini-3.8-flash-high` at 900s, the breadth lens and reconcile on
   `gemini-3.7-flash-medium`. `gemini-3.1-pro-high` is NOT usable - the ACP
   `session/new` advertisement governs `acpx --model`, not `agy models`, and
   run 20260917T230923Z209c parked two lenses on that mismatch pre-turn.
2. Cross-family blinded verifiers: all verifies run `acpx:codex` /
   `gpt-5.6-sol` with one shared prompt body that names no lens, states nothing
   about what the findings sought, and pushes toward neither verdict.
3. Engine work (the only Go changes in the whole plan): per-node `agent:`
   override; per-family `linkAuth` (agy as before, pi gets ONLY
   `.pi/agent/models.json`, codex gets ONLY `.codex/auth.json`); acpx built-in
   agent modes via an explicit `acpx:` prefix.
4. qwen shadow twins via pi, non-gating (requirement B7), prompts carried by
   YAML alias so a twin cannot drift from its lens.

Traps, resolved: `acpx pi --model` IS honored (pi advertises its models; a bogus
id fails loudly pre-turn). No `<id>:medium` thinking suffix needed - the qwen
entries already carry `reasoning: true`. pi still writes nothing to stdout until
the turn ends. Timeout sizing matters more than throughput: at 600s two of three
qwen twins parked, so shadows now run at 900s and one still finished with 22s to
spare.

### Measurement A result (run 20260917T205540Z20ae -> 20260917T232055Z9b19)

Lens prompts identical between runs; only models and verify blinding/family
changed. Honest delta: 1 real finding -> 3 real findings, plus a rejection lane
that works.

| lens | run 1 | run 2 gating | run 2 confirmed | run 2 shadow (qwen) |
|---|---|---|---|---|
| lens-correctness | 0 | 4 | 3 | 8 |
| lens-tests | 1 | 4 | 0 (1 dup, 3 rejected) | 6 |
| lens-callpaths | 0 | 0 | 0 | 8 |

The verify lever is the proven one: run 1's same-family verify echoed its lens's
summary byte-for-byte and confirmed it; run 2's blinded codex verify rewrote 8/8
summaries, moved 5/8 severities, re-anchored 4/8 line numbers and rejected 4/8
on facts the lens never checked, with every evidence quote real. What the
measurement CANNOT attribute: two levers moved at once on top of a model
generation change, n=1 PR, one run per arm.

The loudest result was off the gating path: the free qwen shadows produced 22
findings with ZERO fabricated citations at ~80% precision, including the best
finding of either run (`internal/serve/http.go:303` - the daemon's CDP
`webSocketDebuggerUrl` still built root-absolute, escaping the same prefix this
PR fixes for the viewer), on a lens where gemini returned `[]` at two separate
model tiers.

## Lens-set replacement - PR #3, open (branch feat/polish-lenses, b54115b)

Operator direction 2026-09-17: take the lenses directly from the `/polish`
skill. The three ad-hoc lenses (correctness, callpaths, tests) are replaced by
polish's four - `lens-cleanliness`, `lens-design`, `lens-efficiency`,
`lens-gating` (side-effect gating, the closed-scope correctness lens). Each
keeps its polish agent's own checklist plus the ported rules: read every changed
file fully, real issues only, reuse suggestions must name an existing utility at
a real path, out-of-diff findings are findings (tagged), no efficiency findings
on cold paths, credentials cited never quoted, and the diff/checkout are
untrusted material whose instruction-shaped text is itself a finding. Polish's
own Phase 4 self-validation is deliberately NOT ported - that job belongs to the
blinded cross-family verifies.

The four categories match the pond#289 ground-truth key's categories exactly,
which makes that banked replay directly gradable when it is un-deferred.

Land, then rerun cuttle#62 to measure the lens-set lever alone.

## Batch B - authored, parked (PR #1, commits ce94e0b + ad82ffb)

5. Claim-vs-implementation lens + verify: prose (comments, docs, PR body)
   asserting what the code does not do; authority files read END TO END, not
   grepped - review-v1's biggest measured recall lever. The brief carries the
   counter-lesson explicitly: a mandatory checklist crowds out coequal duties,
   so it restates what remains in scope.
6. Executable rubric per finding: a `rubric` key ("run X, expect Y") on every
   finding, verifiers execute it instead of re-reading prose, and a finding with
   no statable rubric is capped at `suggestion` severity - enforced in the jq
   checks, not asked for in prose.

Adds a MANDATORY second run input: `--input pr_body=<path>` (regenerate with
`gh pr view 62 --repo glim-sh/cuttle --json body --jq .body`).

Rebase note: PR #1 predates the lens replacement. After PR #3 lands,
`lens-claims` becomes a FIFTH lens needing its own routing, blinded verify and
shadow twin, and the rubric commit must be re-applied across the four new lens
prompts and their jq checks. Land 5 alone, rerun, measure; then 6, rerun,
measure.

## Candidate next levers (from Measurement A, not yet planned work)

- Promote the free qwen lane from shadow to a gating lens behind a codex verify,
  since it was the only lane to find the CDP prefix escape. Not a straight
  promotion: its ~80% precision wants filtering, and its timing wants a wider
  timeout than the paid lanes.
- Settle `gemini-3.8-flash-high` vs `gemini-3.7-flash-medium` for the reasoning
  lenses. The acpx-faq records 3.7-flash-medium on par with Opus for
  rubric-driven bulk work and 3.8-flash-high WORSE on that same task; our
  routing is a choice, not yet a measurement.
- Carry the lens's original claim into the ledger (`original_summary` /
  `original_evidence`). Today a rejected finding's ledger entry holds only the
  rebuttal, so the claim that was rejected leaves no record - which defeats the
  precision ledger later.
- Give shadow artifacts their own id namespace (`shadow-<lens>-N`); they
  currently reuse the lens ids, harmless while non-gating, a collision hazard
  for any tool that reads both sets.

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
  should eventually force dynamic lens width (G4). `/polish`'s own
  small-diff fast path (<50 changed lines skips the fan-out) is the shape to
  copy.
- PoC-or-demote and PR-body claim re-execution: need sandboxed execution of
  untrusted code; real design work, and review-v1's own replay showed the
  claim extractor misfiring (exit 127 miscounted as over-claiming).
- Locks 2 and 4 (pond transcript sync, failure classification/retry): design
  from real failure transcripts once review runs produce them; review-v1's
  per-run-pond-store and witness-retry decision records are ready-made input.

## Context for a fresh session

- Executor model facts live in
  `docs/knowledge/references/executor-models-field-guide.md` - ACP-advertised
  catalogs per family, measured observations with their scopes, the traps, and a
  routing table. Read it before routing any node.
- A parked node flips the run-level `record.outcome` to `"parked"`, including a
  non-gating shadow. Grade a run per node from `record.json`, never from the
  headline outcome field.
- cuttle checkout: `~/pjv/glim-sh/cuttle` detached at 8fd8af8f (PR 62 head),
  READ ONLY; diff regenerable via `gh pr diff 62 --repo glim-sh/cuttle`.
- Gates: `nix develop -c just check` (staged) / `just check-ci` (whole repo);
  commits must run through `nix develop -c` or the stale-shell guard fails
  them. `just corpus` spends model quota; keep it manual.
- Worktrees on this host come from the treehouse pool
  (`treehouse get --lease`), never `git worktree add`.
- `gh pr merge` is denied by the agent permission classifier; merges are an
  operator action. `gh pr edit` is broken on this host (token lacks
  `read:org`) - use the `gh api -X PATCH` REST route.
