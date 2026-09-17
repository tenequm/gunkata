# Handoff 2026-09-17 19:09 gunkata@main

## Position

Milestone 1 is **complete and on main** (`db441ce`), CI green
([run 35259144737 was red, 35259995060 is green](https://github.com/tenequm/gunkata/actions/runs/35259995060)).
The engine, the model-free grader and the starter corpus case are built, and the live
two-variant corpus run - the milestone's exit gate under lock 7 - passes with real model
executors.

Work in flight when this was written, all in background agents:

1. **`chore/one-gate`** (being built by an agent, branch not yet pushed at time of writing):
   restructures the quality gate into exactly two `just` verbs, wires them into lefthook and
   collapses CI to one job. Design is operator-approved; see Decisions.
2. **Three tool-evaluation research agents** running in parallel against primary docs, to
   answer whether the three-tool stack (nix + just + lefthook) should collapse: one on nix as
   a task runner (flake apps vs sandboxed checks, treefmt-nix, git-hooks.nix), one on
   lefthook as a task runner (arbitrary named groups via `lefthook run <group>`,
   `stage_fixed` semantics), one on moonrepo (studying `~/pj/pond`, which already uses moon,
   as the live reference).

Nothing from the research has landed yet. `chore/one-gate` is deliberately not on main so it
can be discarded if the research points elsewhere.

## Next steps

1. **Collect the four agent reports.** The builder's report must include the final Justfile
   gate section, lefthook.yml, ci.yml, verification output with timings, and its
   `stage_fixed` finding.
2. **Review `chore/one-gate`**, then decide merge vs discard against the research verdicts.
   Merge is the parent session's job, never an agent's.
   ```
   git -C /home/tenequm/pj/gunkata switch main
   git -C /home/tenequm/pj/gunkata merge --ff-only chore/one-gate
   git -C /home/tenequm/pj/gunkata push
   gh run watch <id> --repo tenequm/gunkata --exit-status
   ```
3. **On merge, update `AGENTS.md`** - its "Working in this repo" section still says
   "`just check` is the full gate (format, lint config, lint, test, knowledge index)", which
   the two-verb split makes false. The replacement law: `just check` is the staged-only
   auto-fixing gate (pre-commit); `just check-ci` is the full verify-only gate (pre-push and
   CI).
4. **Operator action outstanding: read the two requirement docs**
   `docs/2609-17-review-pr-requirements.md` (G1-G10) and
   `docs/2609-17-build-pipeline-requirements.md` (B1-B12) and confirm they match what was
   envisioned. They were recovered from pond by a flash model and are the repo's foundation.
   Nothing should be built on them before that read.
5. **Next engine milestone** is the review-pr use case, which begins only after step 4.
6. **Lock 2 is only half-implemented** - see Open questions 4.

## Decisions

- **Milestone 1 landed on main via fast-forward, no PR.** The operator asked for everything
  pushed to main; history stayed linear so `feat/starter-engine` merged with `--ff-only`.
- **Two-verb gate design** (operator-approved, being built on `chore/one-gate`):
  - `just check` - staged-only, auto-applies fixes, `[parallel]` group:
    `fmt-lint-staged`, `test-staged`, `tidy`, `secrets-staged`, `kb-index`. Pre-commit hook.
  - `just check-ci` - full repo, verify-only (mutates nothing), `[parallel]` group:
    `fmt-lint`, `test`, `tidy-check`, `secrets`, `vuln`, `actions`, `flake`, `kb-check`.
    Pre-push hook **and** CI, so CI runs the literal same command as the hook.
  - Reason for the verify-only twin: a fixing gate in CI either passes while the repo stays
    unformatted or mutates a runner-local tree. A recipe that cannot mutate cannot drift.
  - `check-full` was dropped as a third verb - `check-ci` does the full job.
- **`fmt` and `lint` are ONE serial recipe, never parallel siblings.** golangci-lint takes a
  global lock in the system temp dir and exits with "parallel golangci-lint is running"
  rather than queueing; with fixes on they would also write the same files.
- **`*-staged` recipes resolve their own paths** with
  `git diff --cached --name-only --diff-filter=ACMR`, rather than relying on lefthook to
  pass `{staged_files}`, so `just check` works standalone from any shell.
- **Empty staged set must be loud** - each `*-staged` recipe prints "nothing staged" and
  exits 0, so a green `just check` is never mistaken for "repo is clean".
- **`just corpus` is excluded from every gate.** It spends real model quota; lock 7's graded
  run is a milestone gate, not a commit gate.
- **`secrets` belongs in the base gate, not only the full one** - a secret caught at push is
  already in local history and needs a rewrite to remove.
- **Repo layout** (operator-decided, landed): all Go code plus `.golangci.yml` and
  `testdata/` under `packages/gunkata/`; the graded corpus at `examples/starter/`, where
  everything is graded corpus doubling as usage examples (lock 7's growth rule governs the
  directory). `lefthook.yml` must stay at the repo root - lefthook only discovers configs
  next to `.git`, and moving it silently disables hooks.
- **Corpus model is `gemini-3.7-flash-low`** through acpx to the agy ACP server.
  `gpt-oss-120b-medium` was the operator's first pick but is TUI-only: see Findings.
- **Keep nix AND just** (pre-research reasoning, now under formal review): nix pins which
  tools exist, just decides what runs. A nix build cannot write fixes back to the worktree
  and cannot see the git index, so a fixing staged-only gate cannot live in a sandboxed
  derivation. Whether flake *apps* (unsandboxed) change this answer is exactly what the
  research is settling.

## Findings

- **The live corpus passes, both variants** (the milestone-1 exit gate):
  ```
  pass: produce -> gate -> consume all done, run succeeded, graded clean
  fail: produce done, gate parked, consume never started, graded clean
  corpus ok: pass succeeded, fail parked, both graded clean
  ```
  Run dirs from the verified executions: pass `20260917T182831Z0462`,
  fail `20260917T182856Zd3bc`, and after the lint fix fail `20260917T183723Z6dcc`, all under
  `/home/tenequm/.local/state/gunkata/runs/`.
- **`gpt-oss-120b-medium` is not reachable over ACP.** The probe failed with:
  `Cannot apply --model "gpt-oss-120b-medium": the ACP agent did not advertise that model.
  Available models: gemini-3.8-flash-high, gemini-3.8-flash-medium, gemini-3.8-flash-low,
  gemini-3.7-flash-high, gemini-3.7-flash-medium, gemini-3.7-flash-low,
  gemini-3.6-flash-high, gemini-3.6-flash-medium, gemini-3.6-flash-low, gemini-pro-agent,
  gemini-3.1-pro-low.` The model list in the `agy` TUI is not the list the ACP server serves.
- **A bare engine-owned HOME broke executor startup**, fixed in `26246c2`. The failure was:
  `[acpx] error: RUNTIME AGENT_STARTUP_FAILED ACP agent exited before initialize completed
  (exit=127, signal=null): agy-acp-server: missing
  <runDir>/nodes/produce/home/.local/lib/antigravity-acp/agy_acp_server.par - run
  agy-acp-install`. The wrapper `~/.local/bin/agy-acp-server` resolves its `.par` through
  `$HOME`, so `.local/lib/antigravity-acp` now rides the same explicit symlink inheritance as
  the credentials. Captured as a knowledge-base finding.
- **A stale golangci-lint cache hid a real lint error.** `just check` passed locally while CI
  failed on
  `internal/engine/executor.go:249: line-length-limit: line is 83 characters, out of limit 80
  (revive)`. `golangci-lint cache clean` before a pre-merge verification run is the
  mitigation; the go-dev skill documents this footgun.
- **Gate timings measured on this host** (they decided the design - everything fits in one
  command): govulncheck 1.9s, `nix flake check` 2.6s, lint ~10s (dominant), tests 3.7s,
  fmt-check 2s, kb-check 0.1s, lint-config 0.2s. Sequential full gate ~22s.
- **`just` 1.58.0 supports `[parallel]`** on recipe dependencies, with `--jobs` / `JUST_JOBS`
  to cap concurrency. `[parallel]` runs a recipe's *dependencies* concurrently, so nesting
  one parallel recipe inside another does not flatten into a single wide pool.
- **The platform wipes the deploy layer.** Home Manager generation 81 (2026-09-17 17:50)
  re-activated the Devboxes baseline and removed `acpx`, `agy` and the glue that routes `gh`
  API traffic through the OneCLI gateway; `git push` kept working (the git proxy config
  survived) while `gh` returned bare 401s. The fix is the operator running
  `just deploy-pond-sb` from their Mac - never restoring credentials locally. After the
  redeploy: acpx 0.15.1, agy 1.2.0, `gh` working.
- **pond has an `agy` adapter** rooted at `~/.gemini`, so lock 2's sync-at-task-boundary
  needs no new adapter work: `pond sync agy --path <node-home>/.gemini`. Full adapter list on
  this host: agy, claude-code, codex-cli, oh-my-pi, opencode, pi-coding-agent, grok-build
  (detected).
- **`just check-ci` from the repo root with `./...` fails**:
  `typechecking error: pattern ./...: directory prefix . does not contain main module`. Go
  commands must run with `packages/gunkata` as cwd, which the Justfile does via
  `[working-directory('packages/gunkata')]`.

## Open questions

1. **Which tool owns the gate: nix + just + lefthook (current), nix + lefthook, or moon?**
   Recommended answer: keep nix for provisioning and pick ONE orchestrator. If lefthook can
   run arbitrary named groups (`lefthook run check`) it is the strongest candidate, because
   staged-file handling then lives where the staged concept natively exists and the
   `git diff --cached` plumbing leaves the Justfile; `just` would stay only for dev
   conveniences (`build`, `test-watch`, `corpus`). Decide on the three agent reports, not on
   taste.
2. **Pre-push scope: `check-ci` (full, ~15s) as currently designed, or a narrower gate?**
   Recommended: keep `check-ci`, since `vuln`, `actions` and `flake` would otherwise run
   only in CI and a broken flake or workflow would reach origin before anything noticed.
3. **Do the requirement docs match what the operator envisioned?** Recommended: operator
   reads `docs/2609-17-review-pr-requirements.md` and
   `docs/2609-17-build-pipeline-requirements.md` before any review-pr work starts. Only the
   operator can answer this.
4. **Lock 2's pond half is not implemented.** The engine writes transcripts into
   engine-owned HOMEs (the capture half) but nothing syncs them into gunkata's own dedicated
   pond, and no failure classification reads them. Recommended: implement it when the first
   real failure needs classifying - milestone 1's park path needs no classifier, because
   under lock 4 an unclassified failure parks, which is the asserted behaviour. Do not build
   it speculatively.
5. **Is `pnpm.fetchDeps` worth spending on to flake-pin acpx?** Recommended: no, until a
   driver version skew actually causes a failure. Tracked already in
   `docs/knowledge/findings/runtime-deps-not-yet-flake-pinned.md`.

## Undone instructions

- **Read the two requirement docs** - asked of the operator repeatedly, still outstanding
  (Open question 3).
- **`chore/one-gate` is unfinished and unmerged** - the builder agent was still running.
- **The three tool-evaluation reports are unread** - all three agents were still running.
- **`AGENTS.md` "Working in this repo" not yet updated** for the two-verb gate; correct only
  after `chore/one-gate` merges (Next steps 3).
- Nothing else was asked and left undone.

## References

- `AGENTS.md` (symlinked as `CLAUDE.md`) - design intent, executor contract, the seven locks,
  the MVP YAGNI/KISS build discipline.
- `docs/knowledge/index.md` - the OKF bundle; one decision record per lock. Regenerate the
  listing with `python3 scripts/kb_index.py` after any concept change; pre-commit enforces
  `--check`.
- `docs/2609-17-starter-corpus-case.md` - the lock-7 starter case, now at `examples/starter/`.
- `packages/gunkata/internal/engine/` - layout.go (run dirs, record.json), executor.go (the
  one generic acpx invocation, bare env whitelist, auth symlinks, process groups), engine.go
  (scheduler, evidence evaluation).
- `packages/gunkata/internal/grader/` - the model-free grader; fixtures in
  `packages/gunkata/testdata/grader/` (15 miniature run dirs).
- [gunkata on GitHub](https://github.com/tenequm/gunkata) - main at `db441ce`.
- [build-workflow](https://github.com/tenequm/build-workflow) - the archived predecessor;
  `docs/bernstein-events-log/` there holds the 88-entry event log and the cost attribution
  that motivated gunkata.
- Local skills consulted: `.claude/skills/go-dev/` (Justfile, lefthook and golangci-lint
  references, plus the stale-cache and monorepo footguns), `acpx-faq` (agy on NixOS needs
  `--uid=` and `SSL_CERT_FILE`; `exec` for dispatched work).
- `~/pj/pond` - the live moon reference implementation the third research agent is studying.
