package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Test literals, named because the lint config forbids bare ones.
const (
	scriptPerm = 0o700
	kataPerm   = 0o600
	newline    = "\n"
	outcomeFmt = "outcome = %q, want %q"
	testGrace  = 500 * time.Millisecond
	reapWait   = 5 * time.Second
	pollWait   = 50 * time.Millisecond
	mcpSecret  = "s3cret-value"
	mcpVarName = "GUNKATA_TEST_MCP_KEY"
)

// stubScript is a test double for acpx: no test involves a model. It records
// its argv, environment and stdin in the engine-owned HOME it was given, then
// acts on the directives the test kata wrote into the prompt.
const stubScript = `#!/usr/bin/env bash
set -u

prompt="${@: -1}"
printf '%s\n' "$@" > "$HOME/argv.txt"
env | sort > "$HOME/env.txt"
if [[ " $* " == *" --mcp-config "* ]]; then cat > "$HOME/mcp.json"; fi
echo "stub acpx stdout marker"

field() {
  printf '%s\n' "$prompt" | sed -n "s/^$1=//p" | head -n 1
}

action="$(field ACTION)"
target="$(field TARGET)"
origin="$(field SOURCE)"

case "$action" in
  write)  printf 'hello\n' > "$target" ;;
  derive) printf '%s derived\n' "$(cat "$origin")" > "$target" ;;
  empty)  : > "$target" ;;
  mkdir)  mkdir -p "$target" ;;
  none)   echo "wrote nothing" ;;
  fail)   exit 3 ;;
  hang)
    sleep 600 &
    child=$!
    printf '%s\n%s\n' "$$" "$child" > "$target"
    wait "$child"
    ;;
  *)      echo "stub: unknown ACTION '$action'" >&2; exit 64 ;;
esac
`

// passKata is a three-job DAG: a deterministic fetch copies a file param, an
// agent derives from fetch's artifact, and a deterministic check gates on it.
const passKata = `
name: stub-pass
params:
  seed: {}
  greeting: {default: hi}
agents:
  stub:
    harness: claude
    model: stub-model
    timeout_seconds: 7
workflow:
  fetch:
    pre-steps:
      - cp {{param:seed}} {{output:seed.txt}}
    outputs: [seed.txt]
  derive:
    needs: [fetch]
    agent: stub
    prompt: |
      ACTION=derive
      SOURCE={{artifact:fetch/seed.txt}}
      TARGET={{output:out.txt}}
    outputs: [out.txt]
    post-steps:
      - [grep, -qx, "seeded derived", "{{output:out.txt}}"]
  check:
    needs: [derive]
    post-steps:
      - grep -q derived {{artifact:derive/out.txt}}
      - test {{param:greeting}} = hi
`

// stubACPX puts the stub on PATH and poisons the environment, so a test can
// prove what does and does not cross the executor boundary.
func stubACPX(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, acpxBin), []byte(stubScript), scriptPerm); err != nil {
		t.Fatalf("write stub acpx: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GUNKATA_POISON", "must-not-cross")
}

func writeFile(t *testing.T, path, body string) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), scriptPerm); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(path, []byte(body), kataPerm); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	return path
}

func runKata(t *testing.T, body string, params map[string]string) (Result, error) {
	t.Helper()

	return Run(context.Background(), Options{
		KataPath: writeFile(t, filepath.Join(t.TempDir(), "k.kata.yml"), body),
		RunsRoot: filepath.Join(t.TempDir(), "runs"),
		Params:   params,
	})
}

// mustRun fails the test if the run could not be carried out at all. A
// parked run is a result, not a failure.
func mustRun(t *testing.T, body string, params map[string]string) Result {
	t.Helper()

	res, err := runKata(t, body, params)
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	return res
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(raw)
}

func readRecord(t *testing.T, runDir string) record {
	t.Helper()

	var rec record
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(runDir, recordName))), &rec); err != nil {
		t.Fatalf("parse record: %v", err)
	}

	return rec
}

func assertStates(t *testing.T, rec record, want map[string]jobState) {
	t.Helper()

	for name, state := range want {
		job := rec.Jobs[name]
		if job == nil {
			t.Errorf("record has no job %s", name)

			continue
		}

		if job.State != state {
			t.Errorf("job %s state = %q (%s), want %q", name, job.State, job.Failure, state)
		}
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)

	return err == nil
}

func TestRunPassesAVerifiedDAG(t *testing.T) {
	stubACPX(t)

	seed := writeFile(t, filepath.Join(t.TempDir(), "seed.txt"), "seeded\n")
	res := mustRun(t, passKata, map[string]string{"seed": seed})

	if res.Outcome != OutcomeSucceeded {
		t.Errorf(outcomeFmt, res.Outcome, OutcomeSucceeded)
	}

	rec := readRecord(t, res.RunDir)
	assertStates(t, rec, map[string]jobState{"fetch": stateDone, "derive": stateDone, "check": stateDone})

	if rec.RunID != filepath.Base(res.RunDir) || rec.Kata != "stub-pass" {
		t.Errorf("record run_id = %q, kata = %q", rec.RunID, rec.Kata)
	}

	if rec.Params["seed"] != seed || rec.Params["greeting"] != "hi" {
		t.Errorf("record params = %v, want the bound source and the default", rec.Params)
	}

	if code := rec.Jobs["derive"].ExecutorExit; code == nil || *code != exitOK {
		t.Errorf("derive executor_exit = %v, want 0", code)
	}

	if rec.Jobs["check"].ExecutorExit != nil {
		t.Error("a deterministic job recorded an executor exit")
	}

	snapshot := filepath.Join(res.RunDir, paramsDir, "seed", "seed.txt")
	if readFile(t, snapshot) != "seeded\n" {
		t.Error("the file param was not snapshotted into the run dir")
	}

	if !strings.Contains(readFile(t, filepath.Join(res.RunDir, kataName)), "stub-pass") {
		t.Error("the kata was not copied into the run dir")
	}

	if exists(filepath.Join(res.RunDir, jobsDir, "check", homeDir)) {
		t.Error("a deterministic job got an executor home")
	}
}

func TestRunParksAndHoldsDependents(t *testing.T) {
	stubACPX(t)

	res := mustRun(t, `
name: stub-park
agents:
  stub: {harness: claude, model: m, timeout_seconds: 7}
workflow:
  produce:
    agent: stub
    prompt: |
      ACTION=write
      TARGET={{output:out.txt}}
    outputs: [out.txt]
    post-steps:
      - grep -qx goodbye {{output:out.txt}}
  consume:
    needs: [produce]
    post-steps:
      - "true"
`, nil)

	if res.Outcome != OutcomeParked {
		t.Errorf(outcomeFmt, res.Outcome, OutcomeParked)
	}

	rec := readRecord(t, res.RunDir)
	assertStates(t, rec, map[string]jobState{"produce": stateParked, "consume": statePending})

	if got := rec.Jobs["produce"].Failure; got != "post-step 1 exited 1" {
		t.Errorf("produce failure = %q", got)
	}

	if rec.Jobs["consume"].StartedAt != nil || exists(filepath.Join(res.RunDir, jobsDir, "consume")) {
		t.Error("a job whose need parked left a trace")
	}
}

func TestRunParksOnFailedEvidence(t *testing.T) {
	stubACPX(t)

	cases := map[string]struct {
		action, preStep, failure string
	}{
		"executor exits non-zero": {"fail", "true", "executor exited 3"},
		"output never written":    {"none", "true", "output out is missing or empty"},
		"output empty":            {"empty", "true", "output out is missing or empty"},
		"output dir empty":        {"mkdir", "true", "output out is missing or empty"},
		"pre-step fails":          {"write", "false", "pre-step 1 exited 1"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res := mustRun(t, `
name: stub-evidence
agents:
  stub: {harness: claude, model: m, timeout_seconds: 7}
workflow:
  produce:
    agent: stub
    pre-steps: ["`+tc.preStep+`"]
    prompt: |
      ACTION=`+tc.action+`
      TARGET={{output:out}}
    outputs: [out]
`, nil)

			rec := readRecord(t, res.RunDir)
			assertStates(t, rec, map[string]jobState{"produce": stateParked})

			if got := rec.Jobs["produce"].Failure; got != tc.failure {
				t.Errorf("failure = %q, want %q", got, tc.failure)
			}
		})
	}
}

// TestRunStartsTheExecutorBare holds the executor contract: whitelisted env,
// engine-owned HOME, only the harness's credentials and the host's GitHub
// access linked in, declared skills copied in, MCP config on stdin.
func TestRunStartsTheExecutorBare(t *testing.T) {
	stubACPX(t)

	realHome := t.TempDir()
	t.Setenv("HOME", realHome)
	t.Setenv(mcpVarName, mcpSecret)

	for _, rel := range []string{".claude/.credentials.json", ".config/gh/hosts.yml", ".codex/auth.json", ".claude/settings.json"} {
		writeFile(t, filepath.Join(realHome, rel), "x")
	}

	kataDir := t.TempDir()
	writeFile(t, filepath.Join(kataDir, "skills", "local-skill", "SKILL.md"), "skill")

	kataPath := writeFile(t, filepath.Join(kataDir, "k.kata.yml"), `
name: stub-bare
agents:
  stub:
    harness: claude
    model: stub-model
    timeout_seconds: 7
    options: {reasoning_effort: high, a: b}
    skills: [./skills/local-skill]
    mcps: ["https://mcp.example.com/mcp?key=${`+mcpVarName+`}"]
workflow:
  produce:
    agent: stub
    prompt: |
      ACTION=write
      TARGET={{output:out.txt}}
    outputs: [out.txt]
`)

	res, err := Run(context.Background(), Options{KataPath: kataPath, RunsRoot: filepath.Join(t.TempDir(), "runs")})
	if err != nil || res.Outcome != OutcomeSucceeded {
		t.Fatalf("Run() = %+v, %v", res, err)
	}

	home := filepath.Join(res.RunDir, jobsDir, "produce", homeDir)

	assertArgv(t, res.RunDir, home)
	assertExecutorEnv(t, home)
	assertLinks(t, realHome, home)

	if readFile(t, filepath.Join(home, ".claude", "skills", "local-skill", "SKILL.md")) != "skill" {
		t.Error("the declared skill was not materialized")
	}

	mcp := readFile(t, filepath.Join(home, "mcp.json"))
	if !strings.Contains(mcp, `"name":"example"`) || !strings.Contains(mcp, "key="+mcpSecret) {
		t.Errorf("mcp config = %s, want the expanded server", mcp)
	}

	if strings.Contains(readFile(t, filepath.Join(res.RunDir, recordName)), mcpSecret) {
		t.Error("the expanded MCP secret reached the record")
	}
}

func assertArgv(t *testing.T, runDir, home string) {
	t.Helper()

	argv := strings.Split(strings.TrimRight(readFile(t, filepath.Join(home, "argv.txt")), newline), newline)
	want := []string{
		"--cwd", filepath.Join(runDir, jobsDir, "produce", workDir),
		"--model", "stub-model",
		"--timeout", "7",
		"--approve-all",
		"--format", "quiet",
		"--mcp-config", mcpStdin,
		"claude", "exec",
		"--config-option", "a=b",
		"--config-option", "reasoning_effort=high",
	}

	if !slices.Equal(argv[:len(want)], want) {
		t.Errorf("acpx argv = %q, want it to start with %q", argv, want)
	}

	prompt := strings.Join(argv[len(want):], newline)
	if target := "TARGET=" + filepath.Join(runDir, artifactsDir, "produce", "out.txt"); !strings.Contains(prompt, target) {
		t.Errorf("prompt = %q, want it to carry %q", prompt, target)
	}
}

// assertExecutorEnv holds the executor boundary: the engine-owned home and the
// whitelist, and nothing the parent environment happened to carry.
func assertExecutorEnv(t *testing.T, home string) {
	t.Helper()

	env := map[string]string{}

	for line := range strings.SplitSeq(readFile(t, filepath.Join(home, "env.txt")), newline) {
		if key, value, ok := strings.Cut(line, "="); ok {
			env[key] = value
		}
	}

	want := map[string]string{
		"HOME":            home,
		"TERM":            "dumb",
		"TMPDIR":          filepath.Join(home, tmpDir),
		"XDG_CONFIG_HOME": filepath.Join(home, xdgConfigDir),
		"XDG_DATA_HOME":   filepath.Join(home, xdgDataDir),
		"XDG_STATE_HOME":  filepath.Join(home, xdgStateDir),
		"XDG_CACHE_HOME":  filepath.Join(home, xdgCacheDir),
	}

	for key, value := range want {
		if env[key] != value {
			t.Errorf("executor %s = %q, want %q", key, env[key], value)
		}
	}

	if env["ENABLE_CLAUDEAI_MCP_SERVERS"] != "false" {
		t.Error("the claude harness may load claude.ai connectors")
	}

	// bash sets the rest of these itself; nothing else may appear.
	allowed := append([]string{
		"PWD", "OLDPWD", "SHLVL", "_",
		"ENABLE_CLAUDEAI_MCP_SERVERS",
	}, inherited...)
	for key := range env {
		if _, own := want[key]; !own && !slices.Contains(allowed, key) {
			t.Errorf("%s crossed the executor boundary", key)
		}
	}
}

// assertLinks holds the sole inheritance: the harness's own credentials and
// the host's GitHub access, never another harness's, never other config.
func assertLinks(t *testing.T, realHome, home string) {
	t.Helper()

	for _, rel := range []string{".claude/.credentials.json", ".config/gh"} {
		target, err := os.Readlink(filepath.Join(home, rel))
		if err != nil || target != filepath.Join(realHome, rel) {
			t.Errorf("%s links to %q (%v), want the real home's", rel, target, err)
		}
	}

	for _, rel := range []string{".codex", ".claude/settings.json", ".onecli", ".config/git"} {
		if exists(filepath.Join(home, rel)) {
			t.Errorf("%s crossed into the executor home", rel)
		}
	}
}

func TestRunTimeoutKillsTheProcessGroup(t *testing.T) {
	stubACPX(t)

	grace := killGrace
	killGrace = testGrace

	t.Cleanup(func() { killGrace = grace })

	res := mustRun(t, `
name: stub-hang
agents:
  stub: {harness: claude, model: m, timeout_seconds: 1}
workflow:
  hang:
    agent: stub
    prompt: |
      ACTION=hang
      TARGET={{output:pids.txt}}
    outputs: [pids.txt]
`, nil)

	rec := readRecord(t, res.RunDir)
	assertStates(t, rec, map[string]jobState{"hang": stateParked})

	pids := readFile(t, filepath.Join(res.RunDir, artifactsDir, "hang", "pids.txt"))
	for line := range strings.SplitSeq(strings.TrimSpace(pids), newline) {
		pid, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("stub wrote %q, want a pid: %v", line, err)
		}

		assertDead(t, pid)
	}
}

// assertDead gives a killed process a moment to be reaped, then insists it is
// gone: the engine owns the whole tree, not just the process it started.
func assertDead(t *testing.T, pid int) {
	t.Helper()

	deadline := time.Now().Add(reapWait)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, syscall.Signal(0)); err != nil {
			return
		}

		time.Sleep(pollWait)
	}

	t.Errorf("process %d survived the engine timeout", pid)
}

func TestRunParksWhenACPXIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	res := mustRun(t, `
name: no-acpx
agents:
  stub: {harness: claude, model: m}
workflow:
  j: {agent: stub, prompt: x, outputs: [o]}
`, nil)

	rec := readRecord(t, res.RunDir)
	if !strings.Contains(rec.Jobs["j"].Failure, errACPXMissing.Error()) {
		t.Errorf("failure = %q, want acpx missing", rec.Jobs["j"].Failure)
	}
}

// TestRunSkipsAnOptionalMCP holds that an optional server whose variable is
// unset is left out, warned about and recorded, while the run proceeds.
func TestRunSkipsAnOptionalMCP(t *testing.T) {
	stubACPX(t)
	t.Setenv(mcpVarName, unset)
	os.Unsetenv(mcpVarName)

	var progress bytes.Buffer

	res, err := Run(context.Background(), Options{
		KataPath: writeFile(t, filepath.Join(t.TempDir(), "k.kata.yml"), `
name: k
agents:
  a:
    harness: claude
    model: m
    mcps: ["https://mcp.example.com/mcp?key=${`+mcpVarName+`}"]
workflow:
  j: {agent: a, prompt: "ACTION=write\nTARGET={{output:o}}", outputs: [o]}
`),
		RunsRoot: filepath.Join(t.TempDir(), "runs"),
		Progress: &progress,
	})
	if err != nil || res.Outcome != OutcomeSucceeded {
		t.Fatalf("Run() = %+v, %v", res, err)
	}

	home := filepath.Join(res.RunDir, jobsDir, "j", homeDir)
	if exists(filepath.Join(home, "mcp.json")) || strings.Contains(readFile(t, filepath.Join(home, "argv.txt")), "--mcp-config") {
		t.Error("the skipped server reached acpx")
	}

	want := "warning: job j: skipping MCP example: " + mcpVarName + " is unset"
	if !strings.Contains(progress.String(), want) {
		t.Errorf("progress = %q, want %q", progress.String(), want)
	}

	if got := readRecord(t, res.RunDir).Jobs["j"].SkippedMCPs; !slices.Equal(got, []string{"example"}) {
		t.Errorf("skipped_mcps = %v, want [example]", got)
	}
}

// TestRunRejectsBeforeRunning holds that a run which cannot be carried out
// fails before it creates anything.
func TestRunRejectsBeforeRunning(t *testing.T) {
	t.Setenv(mcpVarName, unset)
	os.Unsetenv(mcpVarName)

	const job = "workflow:\n  j: {agent: a, prompt: x, outputs: [o]}\n"

	cases := map[string]struct {
		body   string
		params map[string]string
		want   error
	}{
		"unbound param":    {"name: k\nparams:\n  p: {}\nworkflow:\n  j: {outputs: [o]}\n", nil, errUnboundParam},
		"undeclared param": {"name: k\nworkflow:\n  j: {outputs: [o]}\n", map[string]string{"q": "1"}, errUnknownParam},
		"unset MCP var":    {"name: k\nagents:\n  a: {harness: claude, model: m, mcps: [{url: \"https://x/${" + mcpVarName + "}\", required: true}]}\n" + job, nil, errMCPVar},
		"skills on codex":  {"name: k\nagents:\n  a: {harness: codex, model: m, skills: [./s]}\n" + job, nil, errSkillDirs},
		"bad skill URL":    {"name: k\nagents:\n  a: {harness: claude, model: m, skills: [\"https://github.com/o/r\"]}\n" + job, nil, errSkillURL},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := runKata(t, tc.body, tc.params)
			if !errors.Is(err, tc.want) {
				t.Errorf("Run() error = %v, want %v", err, tc.want)
			}

			if res.RunDir != unset {
				t.Errorf("a rejected run created %s", res.RunDir)
			}
		})
	}
}

func TestParseTreeURL(t *testing.T) {
	t.Parallel()

	got, err := parseTreeURL("https://github.com/tenequm/skills/tree/main/skills/polish")
	want := treeURL{repo: "https://github.com/tenequm/skills", ref: "main", path: "skills/polish"}

	if err != nil || got != want {
		t.Errorf("parseTreeURL() = %+v, %v; want %+v", got, err, want)
	}
}
