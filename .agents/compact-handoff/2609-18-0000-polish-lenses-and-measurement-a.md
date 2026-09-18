# Handoff 2026-09-18 00:00 UTC gunkata@main

## Position

Building the PR-review capability on top of the gunkata engine, one lever at a
time, measuring each on a rerun of the same PR (glim-sh/cuttle#62) so every
change gets its own before/after number (requirement B3).

Batch A is landed and measured. The lens set is being replaced wholesale with
the four lenses from the `/polish` skill (operator direction: "our lenses are
stupid... can you take lenses directly from /polish skill?") - that work is
authored and pushed as PR #3, NOT yet merged. We stopped waiting for the
operator to merge PR #3; the auto-mode permission classifier denies
`gh pr merge` ("Merge Without Review"), so merges are operator-run or need an
explicit instruction.

A background subagent is still running a free-lane model comparison (qwen
27b vs flash-next on the shadow lenses); it had not reported at compaction
time. Its transcript:
`/home/tenequm/.cache/tmp/claude-10003/-home-tenequm-pj-gunkata/01404a65-4a81-425a-84b5-e5cb5fda9c6f/tasks/a3d083a8afd8fe4d1.output`
(do NOT read that file directly - it is a full JSONL transcript; ask the agent
via SendMessage to `a3d083a8afd8fe4d1`, or just wait for the notification).

## Next steps

1. Merge PR #3 (operator action; the classifier blocks the agent):
   `gh pr merge 3 --merge` then `git pull --ff-only`
   PR: https://github.com/tenequm/gunkata/pull/3 (branch `feat/polish-lenses`,
   commit `b54115b`, +451/-157, graph file only).
2. Rerun the review to measure the lens-set lever alone (~15 min, spends
   gemini + codex subscription quota, no Claude quota):
   ```
   cd /home/tenequm/pj/gunkata && nix develop -c go -C packages/gunkata run \
     ./cmd/gunkata run --input diff=<diff path> graphs/review-cuttle-62.yaml
   ```
   The diff input lived at
   `/home/tenequm/.cache/tmp/claude-10003/-home-tenequm-pj-gunkata/a371f06e-a84b-4790-93ea-42d110a445f5/scratchpad/cuttle-pr62.diff`
   (session scratchpad - assume gone; regenerate with
   `gh pr diff 62 --repo glim-sh/cuttle > <path>`, 483 lines).
3. Compare that run against run 2 (`20260917T232055Z9b19`) the same way
   Measurement A was compared - delegate to an Opus subagent, read-only over
   the run dirs plus the cuttle checkout at `~/pjv/glim-sh/cuttle` (detached at
   `8fd8af8f96ca8f5cba5975c9228f2cb5e252c6e3`, READ ONLY).
4. Read the qwen-27b comparison report when it lands; decide the shadow lane's
   model.
5. Then Batch B, PR #1 (https://github.com/tenequm/gunkata/pull/1, branch
   `feat/review-graph-batch-b`, commits `ce94e0b` + `ad82ffb`): rebase over the
   polish lens set first - `lens-claims` becomes a fifth lens needing its own
   model routing, blinded verify and shadow twin, and the rubric commit must be
   re-applied across the four new lens prompts and their jq checks. Land commit
   1, rerun, measure; then commit 2, rerun, measure.
   Batch B also adds a MANDATORY second run input: every invocation then needs
   `--input pr_body=<path>` (regenerate with
   `gh pr view 62 --repo glim-sh/cuttle --json body --jq .body`).

## Decisions

- **Batch A lands before Batch B, one lever per measurement.** Reason: B3 wants
  per-mechanism attribution; review-v1 could attribute 3/14 -> 5/14 -> 8/14
  only because it moved one lever at a time.
- **Batch A and Batch B authored in parallel treehouse worktrees, landed
  serially.** Reason: the files are nearly disjoint, so parallel authoring costs
  nothing; serial landing preserves attribution.
- **Both batches go through PRs, not direct pushes to main.** Operator
  direction. PR #2 (Batch A) was merged by the operator; #1 and #3 are open.
- **The lens set is taken from `/polish`, replacing the three ad-hoc lenses.**
  Operator direction. The four polish lenses (cleanliness, design & reuse,
  efficiency, side-effect gating) also match the pond#289 ground-truth key's
  categories exactly, which makes that banked replay directly gradable.
- **`/polish` Phase 4 self-validation was deliberately NOT ported.** Reason: in
  this architecture that job belongs to the blinded cross-family verify nodes,
  which are stronger than the same model re-checking its own agents.
- **Reasoning lenses route to `gemini-3.8-flash-high`, not `gemini-3.1-pro-high`.**
  Operator objection ("3.1 pro is very old outdated") plus a hard fact: the ACP
  lane does not advertise `gemini-3.1-pro-high` at all.
- **No Claude model anywhere in the graph.** Reason: operator is conserving
  Claude Max quota; all fan-out subagents are `model: opus`, graph nodes are
  gemini/codex/qwen only.
- **Shadow twins stay non-gating** (no `needs`, nothing needs them) and a per-node
  "advisory" flag was deliberately NOT added to the engine - that is the
  speculative knob the build discipline forbids.
- **pond#289 replay stays deferred** (operator: "that would be too slow").

## Findings

- **Measurement A verdict (run 1 `20260917T205540Z20ae` vs run 2
  `20260917T232055Z9b19`, lens prompts identical, only models + verify
  blinding/family changed):** real improvement, smaller than the raw counts.
  Honest delta is 1 real issue -> 3 real issues plus a working rejection lane,
  not 1 -> 8.

  | lens | run 1 | run 2 gating | run 2 confirmed | run 2 shadow (qwen) |
  |---|---|---|---|---|
  | lens-correctness | 0 | 4 | 3 | 8 |
  | lens-tests | 1 | 4 | 0 (1 dup, 3 rejected) | 6 |
  | lens-callpaths | 0 | 0 | 0 | 8 |
  | total | 1 | 8 | 3 + 1 duplicate | 22 |

- **The verify lever is the proven one.** Run 1's same-family verify was a
  rubber stamp: `verify-tests.json[0].summary` is byte-identical to
  `lens-tests.json[0].summary`, same severity, same line, evidence re-quoting
  the lens's own snippet. Run 2's blinded cross-family codex verify rewrote
  8/8 summaries, changed severity on 5/8, re-anchored file:line on 4/8, and
  rejected 4/8 on facts the lens never checked - every `verified_evidence`
  quote checked out as real code. One soft call: `lens-correctness-1` was
  confirmed after the verify itself articulated the refutation.
- **What Measurement A CANNOT attribute:** two levers moved at once (model tier
  AND verify blinding/family) on top of a model-generation change
  (3.7-flash-low -> 3.8-flash-high / 3.7-flash-medium). n=1 PR, 1 run per arm.
  A blinded same-family verify at 3.7-flash-low was never run.
- **The free qwen lane over-delivered.** 22 findings, **zero fabricated
  citations** (all 22 file:line anchors exist and contain what they claim),
  ~80% plausible-real (~18/22). It produced the best finding of either run:
  `internal/serve/http.go:303` (also :331/:333) - the daemon's CDP
  `webSocketDebuggerUrl` is still built root-absolute
  (`scheme://host + "/" + wsPath`), escaping the very proxy prefix this PR
  fixes for the viewer. Gemini's `lens-callpaths` returned `[]` in BOTH runs at
  two different model tiers while qwen returned 8 verifiable findings on the
  identical prompt - a model/lens fit problem, and a stronger lead than either
  Batch A lever.
- **Other real qwen-only findings:** `test/smoke/viewer.go:294-296`
  (`waitForViewerConnection` returns on the first `Runtime.evaluate` error
  inside its 30s poll -> flaky on a healthy run); `viewer.go:103,157-163`
  (8-slot `attempts` channel drained once per check while the viewer reconnects
  every 2s -> a broken deployment blocks the proxy and hangs
  `httptest.Server.Close()`); `internal/cli/navigate.go:70-72` (viewer-tab
  exclusion matches only `http://127.0.0.1:<port>/`).
- **Confirmed findings on cuttle#62 (run 2 ledger):** `lens-correctness-2` and
  `lens-correctness-3` are the good pair - resolving `websockify` against a
  slashless prefix URL escapes the prefix
  (`new URL("websockify", "http://h/v/smoke/platform")` -> `/v/smoke/websockify`),
  and the smoke proxy never tests that form (`viewerPrefix = "/v/smoke/platform/"`
  at `test/smoke/viewer.go:22`, so `HasPrefix` is false and nothing redirects).
  The in-diff comment asserts a proxy redirects the slashless form; nothing in
  the repo does.
- **Reconcile integrity is clean (G5):** 8 ledger entries for 8 findings, each
  exactly once, ids intact lens -> verify -> ledger, `duplicate_of` pointer
  correct. Two audit-trail wrinkles, not integrity failures: (a) the ledger
  keeps the VERIFY's summary and drops the lens's original evidence, so for a
  rejected finding the ledger shows the rebuttal, never the claim; (b) shadow
  artifacts reuse the lens id namespace (`lens-correctness-1`...), harmless
  while non-gating, a collision hazard if ever merged.
- **Timing, run 2:** gating path 431s, wall clock 878s. Per node -
  lens-correctness 267s, lens-tests 201s, lens-callpaths 85s,
  verify-correctness 118s, verify-tests 117s, verify-callpaths 27s,
  reconcile 46s; shadows 878s / 684s / 623s. **`shadow-lens-correctness`
  finished 22s inside its 900s timeout (97.6%)** - the free lane is the run's
  critical path and one slow turn from a timeout.
- **`gemini-3.1-pro-high` is not usable here.** The ACP `session/new`
  advertisement, not `agy models`, governs `acpx --model`. Run
  `20260917T230923Z209c` parked two lenses on it pre-turn. Advertised over ACP:
  `gemini-3.8-flash-{high,medium,low}`, `gemini-3.7-flash-{high,medium,low}`,
  `gemini-3.6-flash-{high,medium,low}`, `gemini-pro-agent`,
  `gemini-3.1-pro-low`. `agy models` additionally lists `gemini-3.1-pro-high`,
  `claude-sonnet-4-6`, `claude-opus-4-6-thinking`, `gpt-oss-120b-medium`, none
  of which the ACP list carries.
- **acpx `pi` honors `--model`** (pi advertises models in `session/new`; a bogus
  id fails loudly before the first turn and prints the three `litellm/qwen*`
  ids). No `<id>:medium` thinking suffix was needed - all three qwen entries
  already carry `reasoning: true`.
- **codex over acpx works from a bare home** with only `~/.codex/auth.json`
  linked; advertises `gpt-6-astra, gpt-5.6-sol, gpt-5.6-terra, gpt-5.6-luna,
  gpt-5.5`. It has no effort knob (no id suffix; effort is a `-c` config
  override the bare home does not link), so codex nodes run at family default.
- **The bare-executor contract holds empirically:** pi in an engine-style bare
  home with only `.pi/agent/models.json` symlinked loaded ZERO skills and ZERO
  extensions (under the real HOME the same probe printed ~25 SKILL.md paths).
- **The qwen lane is free, NOT private.** `~/.pi/agent/models.json` points its
  litellm provider at a remote HTTPS gateway, so prompts leave the machine -
  build-workflow's "local-first" label is wrong for this host.
- **pi traps:** writes nothing to stdout until the turn ends (an empty
  `executor.log` mid-run is normal, not a hang); an acpx
  `Failed to spawn agent command: npx pi-acp@...` error usually means a
  non-existent `--cwd`, not a missing npx.
- **A parked shadow twin flips the run-level `record.outcome` to `"parked"`**
  (the scheduler parks the run if any node is not done). No evidence gates on a
  twin. Grade runs per node from `record.json`, never from the headline outcome.
- **`gh pr merge` is denied by the auto-mode classifier** ("Merge Without
  Review"). `gh pr create` works. `gh pr edit` is broken on this host (token
  lacks `read:org`) - use the `gh api -X PATCH` REST route.
- **One subagent could not run `nix develop -c git commit`** (the worktree
  isolation guard refuses git as an operand of nix). Its workaround, verified
  sound: confirm the shell is already in the dev shell (`IN_NIX_SHELL` set,
  `just tools` passes), then commit bare - the pre-commit `just check` gate
  still ran green. From the main checkout `nix develop -c git commit` works.

## Open questions

1. Should the qwen free lane be promoted from shadow to a gating lens, given it
   was the only lane to find `internal/serve/http.go:303`?
   **Recommendation:** not as a straight promotion. Run it as the next measured
   lever after the polish-lens rerun - specifically `lens-design` or the
   cross-cutting lens on qwen with a codex verify in front of it, so its ~80%
   precision is filtered rather than trusted. Its 878s/900s timing means a
   gating qwen node also needs a wider timeout than the paid lanes.
2. Which free-lane model for the shadows, `qwen3.8-flash-next` or
   `qwen3.8-27b-nvfp4`?
   **Recommendation:** wait for the running comparison; default to flash-next
   unless 27b shows materially better precision, because 27b runs at 78 tok/s
   vs 209 and flash-next already nearly timed out at 900s.
3. Should the ledger carry the lens's original claim alongside the verify's
   rebuttal?
   **Recommendation:** yes, add `original_summary` + `original_evidence` to the
   reconcile schema and its jq check - a rejected finding currently leaves no
   record of what was rejected, which defeats the precision ledger later.
4. Should shadow artifacts get their own id namespace (`shadow-<lens>-N`)?
   **Recommendation:** yes, cheap now (prompt + jq only) and it removes a
   collision hazard before any comparison tooling reads both sets.
5. Is `gemini-3.8-flash-high` actually better than `gemini-3.7-flash-medium`
   for the reasoning lenses?
   **Recommendation:** unresolved and worth one run. The acpx-faq records
   3.7-flash-medium on par with Opus for rubric-driven bulk work and
   3.8-flash-high WORSE on that same task; our routing choice is not yet a
   measurement. Test it after the lens-set lever, not before.

## Undone instructions

- **Merge PR #3 and rerun** - the operator's last direction chain ends here;
  blocked on the merge permission.
- **The cuttle#62 PR comment was drafted but never posted.** The operator
  approved drafting and posting-class actions in principle earlier in the
  session; it has not been posted, and the draft did not survive into any file.
  Re-draft from the run 2 ledger if still wanted.
- **pond#289 replay** - deferred by explicit operator decision, grading key
  banked at `docs/knowledge/references/pond-pr-289-review-ground-truth.md`
  (replay must run against head `6a83a53439d5b4b0463d8ee7d897605d69dc2ba8`,
  never the branch head).
- **Deferred items from the port plan** that remain untouched: zero-model lint
  node, precision ledger across runs, pond#197 width pressure-test,
  PoC-or-demote, PR-body claim re-execution, locks 2 and 4.

## References

- Plan: `docs/plans/2609-17-review-graph-port-plan.md`
- Graph: `graphs/review-cuttle-62.yaml` (10 nodes on main; 13 nodes on PR #3)
- Requirements: `docs/2609-17-review-pr-requirements.md` (G1-G10),
  `docs/2609-17-build-pipeline-requirements.md` (B1-B12, B3 = per-lever
  measurement, B7 = non-gating data collection)
- KB: `docs/knowledge/references/executor-models-field-guide.md` (commit
  `a9288d3` - ACP-advertised catalogs, measured observations with scopes,
  traps, routing table),
  `docs/knowledge/references/pond-pr-289-review-ground-truth.md`,
  `docs/knowledge/decisions/graphs-declare-input-names-runs-bind-the-files.md`
- Source designs: `/home/tenequm/pj/build-workflow/docs/plans/2609-11-review-v1.md`,
  `/home/tenequm/pj/build-workflow/docs/knowledge/findings/executor-model-capability-floor.md`,
  `/home/tenequm/pj/build-workflow/docs/knowledge/references/review-model-lanes-2609-11.md`
- Lens source: `/home/tenequm/.claude/skills/polish/SKILL.md` (Agents 1-4 plus
  the Rules section)
- Runs: `/home/tenequm/.local/state/gunkata/runs/20260917T205540Z20ae` (run 1),
  `.../20260917T230923Z209c` (parked on the unadvertised model),
  `.../20260917T232055Z9b19` (Measurement A)
- PRs: #1 Batch B (open, draft), #2 Batch A (merged), #3 polish lenses (open)
- Gates: `nix develop -c just check` (staged, pre-commit),
  `just check-ci` (whole repo, pre-push and CI). `just corpus` spends real model
  quota and stays manual.
- Worktrees: `treehouse get --lease --lease-holder <label>`, return with
  `treehouse return <path>`. Never `git worktree add`, never `rm -rf` a slot.
