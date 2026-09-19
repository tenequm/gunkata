package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
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

// stubStream is what the stub prints on stdout: an ACP event stream as acpx
// --format json emits it, with a chatty chunk the log must leave out and two
// lines that are not JSON, which must cost one warning.
const stubStream = `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":1}}
{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":{"sessionUpdate":"tool_call","toolCallId":"toolu_1","title":"Terminal","kind":"execute","status":"pending"}}}
{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":{"sessionUpdate":"tool_call_update","toolCallId":"toolu_1","title":"ls -la","kind":"execute","status":null}}}
{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"pondering"}}}}
{"jsonrpc":"2.0","id":0,"method":"session/request_permission","params":{"toolCall":{"toolCallId":"toolu_1","title":"ls -la","kind":"execute"}}}
not json
{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":{"sessionUpdate":"tool_call_update","toolCallId":"toolu_1","title":null,"kind":null,"status":"completed","rawOutput":"ok"}}}
{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":{"sessionUpdate":"usage_update","used":100,"size":200000,"cost":{"amount":0.01,"currency":"USD"}}}}
also not json
{"jsonrpc":"2.0","id":3,"result":{"stopReason":"end_turn","usage":{"inputTokens":18,"outputTokens":719,"totalTokens":737}}}
`

// stubScript is a test double for acpx: no test involves a model. It records
// its argv, environment and stdin in the engine-owned HOME it was given,
// prints stubStream and a stderr marker, then acts on the directives the test
// kata wrote into the prompt.
const stubScript = `#!/usr/bin/env bash
set -u

prompt="${@: -1}"
printf '%s\n' "$@" > "$HOME/argv.txt"
env | sort > "$HOME/env.txt"
find "$HOME" -type l | sort | while IFS= read -r l; do
  printf '%s %s\n' "${l#"$HOME"/}" "$(readlink "$l")"
done > "$HOME/links.txt"
if [[ " $* " == *" --mcp-config "* ]]; then cat > "$HOME/mcp.json"; fi
cat <<'JSONL'
` + stubStream + `JSONL
echo "stub acpx stderr marker" >&2

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
  rotate) # agy's token refresh: the link becomes a file, via a temp file
    token="$HOME/.gemini/antigravity-acp/acp_token.json"
    printf 'hello\n' > "$target"
    printf 'refreshed\n' > "$token.1.tmp"
    rm "$token"
    printf 'refreshed\n' > "$token"
    ;;
  refresh|mcp|clear) # Claude Code's login write: a temp file renamed over the link
    login="$HOME/.claude/.credentials.json"
    case "$action" in
      refresh) body='{"claudeAiOauth":{"expiresAt":2000},"mcpOAuth":{"a":{"expiresAt":4000},"b":{"expiresAt":3000}}}' ;;
      mcp)     body='{"claudeAiOauth":{"expiresAt":1000},"mcpOAuth":{"b":{"expiresAt":3000}}}' ;;
      clear)   body='{"claudeAiOauth":{"expiresAt":0}}' ;;
    esac
    printf 'hello\n' > "$target"
    printf '%s\n' "$body" > "$login.tmp.1"
    mv "$login.tmp.1" "$login"
    for _ in $(seq 100); do [[ -L "$login" ]] && break; sleep 0.05; done
    readlink "$login" > "$HOME/relinked.txt"
    ;;
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
	// The shared npm cache lands here, never in the host's.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
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

	if entries, _ := os.ReadDir(filepath.Join(res.RunDir, jobsDir, "check", homeDir)); len(entries) != 1 || entries[0].Name() != workDir {
		t.Error("a deterministic job's home holds more than its work dir")
	}

	assertExecutorOutput(t, filepath.Join(res.RunDir, jobsDir, "derive"))
	assertRunLog(t, res.RunDir)
}

// assertExecutorOutput holds that the event stream is kept verbatim and
// stderr apart from it.
func assertExecutorOutput(t *testing.T, jobDir string) {
	t.Helper()

	if got := readFile(t, filepath.Join(jobDir, executorFeed)); got != stubStream {
		t.Errorf("%s = %q, want the stub's stdout verbatim", executorFeed, got)
	}

	if got := readFile(t, filepath.Join(jobDir, executorLog)); got != "stub acpx stderr marker\n" {
		t.Errorf("%s = %q, want the stub's stderr", executorLog, got)
	}
}

// assertRunLog holds gunkata.log to JSON lines that carry the lifecycle and
// the executor events worth watching, and nothing of the chatty chunks.
func assertRunLog(t *testing.T, runDir string) {
	t.Helper()

	raw := readFile(t, filepath.Join(runDir, runLog))
	counts := map[string]int{}

	for line := range strings.SplitSeq(strings.TrimRight(raw, newline), newline) {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("%s line %q is not JSON: %v", runLog, line, err)
		}

		counts[logKey(event)]++
	}

	for _, want := range []string{
		"run start", "run end", "record written",
		"job start derive", "job end derive done", "job end check done",
		"step fetch pre-step", "step derive post-step",
		"output derive out.txt",
		"executor start derive", "executor exit derive",
		"tool start derive execute Terminal",
		"tool end derive execute ls -la completed",
		"permission request derive execute ls -la",
		"turn end derive end_turn",
	} {
		if counts[want] != 1 {
			t.Errorf("%s has %d %q events, want 1:\n%s", runLog, counts[want], want, raw)
		}
	}

	if got := counts["executor stream has a line that is not JSON derive"]; got != 1 {
		t.Errorf("%s warned %d times about lines that are not JSON, want once", runLog, got)
	}

	if strings.Contains(raw, "pondering") {
		t.Errorf("%s logged a thought chunk", runLog)
	}
}

// logKey names an event by its message and the attrs that tell it apart.
func logKey(event map[string]any) string {
	parts := []string{event["msg"].(string)}

	for _, key := range []string{keyJob, keyState, keyKind, "output", keyTitle, "status", "stop_reason"} {
		if value, ok := event[key].(string); ok && value != unset {
			parts = append(parts, value)
		}
	}

	return strings.Join(parts, " ")
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
	assertLinks(t, realHome, home, ".claude/.credentials.json", ".config/gh")

	if readFile(t, filepath.Join(home, ".claude", "skills", "local-skill", "SKILL.md")) != "skill" {
		t.Error("the declared skill was not materialized")
	}

	mcp := readFile(t, filepath.Join(home, "mcp.json"))
	if !strings.Contains(mcp, `"name":"example"`) || !strings.Contains(mcp, "key="+mcpSecret) {
		t.Errorf("mcp config = %s, want the expanded server", mcp)
	}

	for _, name := range []string{recordName, runLog} {
		if strings.Contains(readFile(t, filepath.Join(res.RunDir, name)), mcpSecret) {
			t.Errorf("the expanded MCP secret reached %s", name)
		}
	}
}

// TestRunStartsAgyBare holds agy's own shape: the host's ACP server wrapper
// as acpx's agent, its login linked in, skills in the Gemini home, and no
// token left once the server has rewritten its link into a file.
func TestRunStartsAgyBare(t *testing.T) {
	stubACPX(t)

	realHome := t.TempDir()
	t.Setenv("HOME", realHome)

	auth := harnessAuth[harnessAgy]
	token := ".gemini/antigravity-acp/acp_token.json"

	for _, rel := range slices.Concat(auth, []string{".claude/.credentials.json", ".gemini/config/skills/host-skill/SKILL.md"}) {
		writeFile(t, filepath.Join(realHome, rel), "x")
	}

	kataDir := t.TempDir()
	writeFile(t, filepath.Join(kataDir, "skills", "local-skill", "SKILL.md"), "skill")

	kataPath := writeFile(t, filepath.Join(kataDir, "k.kata.yml"), `
name: stub-agy
agents:
  stub: {harness: agy, model: stub-model, timeout_seconds: 7, skills: [./skills/local-skill]}
workflow:
  produce:
    agent: stub
    prompt: |
      ACTION=rotate
      TARGET={{output:out.txt}}
    outputs: [out.txt]
`)

	res, err := Run(context.Background(), Options{KataPath: kataPath, RunsRoot: filepath.Join(t.TempDir(), "runs")})
	if err != nil || res.Outcome != OutcomeSucceeded {
		t.Fatalf("Run() = %+v, %v", res, err)
	}

	home := filepath.Join(res.RunDir, jobsDir, "produce", homeDir)

	argv := readFile(t, filepath.Join(home, "argv.txt"))
	if want := "--json-strict\n--agent\nagy-acp-server\nexec\n"; !strings.Contains(argv, want) {
		t.Errorf("acpx argv = %q, want %q after the global flags", argv, want)
	}

	skills := filepath.Join(home, ".gemini", "config", "skills")
	if readFile(t, filepath.Join(skills, "local-skill", "SKILL.md")) != "skill" || exists(filepath.Join(skills, "host-skill")) {
		t.Error("the Gemini home's skills are not exactly the declared one")
	}

	assertLinks(t, realHome, home, auth...)

	if exists(filepath.Join(home, token+".1.tmp")) {
		t.Error("the token's temp file outlived the executor")
	}

	if readFile(t, filepath.Join(realHome, token)) != "x" {
		t.Error("the executor wrote through to the real token")
	}
}

// TestRunReturnsClaudeLogin holds that a login Claude Code refreshes in its
// link's place reaches the host while the executor still runs, and the link
// comes back. Each entry - the subscription login, each MCP server's - keeps
// whichever copy expires later, so a login the host gained meanwhile
// survives; a login cleared as dead never overwrites the host's.
func TestRunReturnsClaudeLogin(t *testing.T) {
	const hostLogin = `{"claudeAiOauth":{"expiresAt":1000},` +
		`"mcpOAuth":{"a":{"expiresAt":5000},"b":{"expiresAt":1000},"c":{"expiresAt":1}}}`

	for action, want := range map[string]string{
		"refresh": `{"claudeAiOauth":{"expiresAt":2000},` +
			`"mcpOAuth":{"a":{"expiresAt":5000},"b":{"expiresAt":3000},"c":{"expiresAt":1}}}` + newline,
		"mcp": `{"claudeAiOauth":{"expiresAt":1000},` +
			`"mcpOAuth":{"a":{"expiresAt":5000},"b":{"expiresAt":3000},"c":{"expiresAt":1}}}` + newline,
		"clear": hostLogin,
	} {
		t.Run(action, func(t *testing.T) {
			stubACPX(t)

			realHome := t.TempDir()
			t.Setenv("HOME", realHome)

			login := filepath.Join(realHome, claudeLogin)
			writeFile(t, login, hostLogin)

			res := mustRun(t, `
name: stub-login
agents:
  stub: {harness: claude, model: stub-model, timeout_seconds: 7}
workflow:
  produce:
    agent: stub
    prompt: |
      ACTION=`+action+`
      TARGET={{output:out.txt}}
    outputs: [out.txt]
`, nil)

			home := filepath.Join(res.RunDir, jobsDir, "produce", homeDir)
			if got := readFile(t, filepath.Join(home, "relinked.txt")); got != login+newline {
				t.Errorf("login link during the run = %q, want %q", got, login)
			}

			if got := readFile(t, login); got != want {
				t.Errorf("host login = %q, want %q", got, want)
			}

			if exists(filepath.Join(home, claudeLogin)) {
				t.Error("the login outlived the executor in the run dir")
			}
		})
	}
}

func assertArgv(t *testing.T, runDir, home string) {
	t.Helper()

	argv := strings.Split(strings.TrimRight(readFile(t, filepath.Join(home, "argv.txt")), newline), newline)
	want := []string{
		"--cwd", filepath.Join(home, workDir),
		"--model", "stub-model",
		"--timeout", "7",
		"--approve-all",
		"--format", "json", "--json-strict",
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
		// Engine-side, shared by every executor, so no job re-installs the
		// ACP adapter into its bare home.
		"npm_config_cache": filepath.Join(os.Getenv("XDG_CACHE_HOME"), npmCacheDir),
	}

	if !exists(want["npm_config_cache"]) {
		t.Error("the shared npm cache was not created")
	}

	for key, value := range want {
		if env[key] != value {
			t.Errorf("executor %s = %q, want %q", key, env[key], value)
		}
	}

	claudeEnv := map[string]string{}

	for _, pair := range harnessEnv[harnessClaude] {
		key, value, _ := strings.Cut(pair, "=")
		claudeEnv[key] = value

		if env[key] != value {
			t.Errorf("claude executor %s = %q, want %q", key, env[key], value)
		}
	}

	if claudeEnv["ENABLE_CLAUDEAI_MCP_SERVERS"] != "false" || claudeEnv["ACPX_CLAUDE_INCLUDE_USER_SETTINGS"] != "1" {
		t.Error("the claude harness may load claude.ai connectors or skip its skills")
	}

	// bash sets the rest of these itself; nothing else may appear.
	allowed := slices.Concat([]string{"PWD", "OLDPWD", "SHLVL", "_"}, inherited, slices.Collect(maps.Keys(claudeEnv)))
	for key := range env {
		if _, own := want[key]; !own && !slices.Contains(allowed, key) {
			t.Errorf("%s crossed the executor boundary", key)
		}
	}
}

// assertLinks holds the sole inheritance: while the executor runs, exactly
// the harness's own credentials and the host's GitHub access are linked in,
// never another harness's, never other config; once it is gone, none are left.
func assertLinks(t *testing.T, realHome, home string, want ...string) {
	t.Helper()

	wantLinks := map[string]string{}
	for _, rel := range want {
		wantLinks[rel] = filepath.Join(realHome, rel)
	}

	links := map[string]string{}

	for line := range strings.SplitSeq(readFile(t, filepath.Join(home, "links.txt")), newline) {
		if rel, target, ok := strings.Cut(line, " "); ok {
			links[rel] = target
		}
	}

	if !maps.Equal(links, wantLinks) {
		t.Errorf("links during the run = %v, want %v", links, wantLinks)
	}

	for _, rel := range want {
		if exists(filepath.Join(home, rel)) {
			t.Errorf("%s outlived the executor in the run dir", rel)
		}
	}
}

// TestRunStartsCodexBare holds what Codex needs beyond the shared contract:
// its secret-free config written into the executor's HOME, the declared
// skill where Codex loads user skills, full access set, only its own
// credentials linked.
func TestRunStartsCodexBare(t *testing.T) {
	stubACPX(t)

	realHome := t.TempDir()
	t.Setenv("HOME", realHome)

	for _, rel := range []string{".codex/auth.json", ".codex/config.toml", ".agents/skills/host-skill/SKILL.md", ".claude/.credentials.json"} {
		writeFile(t, filepath.Join(realHome, rel), "host")
	}

	kataDir := t.TempDir()
	writeFile(t, filepath.Join(kataDir, "skills", "local-skill", "SKILL.md"), "skill")

	kataPath := writeFile(t, filepath.Join(kataDir, "k.kata.yml"), `
name: stub-codex
agents:
  stub: {harness: codex, model: m, skills: [./skills/local-skill]}
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

	if got := readFile(t, filepath.Join(home, ".codex", "config.toml")); got != codexConfig {
		t.Errorf("codex config = %q, want the engine's own", got)
	}

	if readFile(t, filepath.Join(home, ".agents", "skills", "local-skill", "SKILL.md")) != "skill" || exists(filepath.Join(home, ".agents", "skills", "host-skill")) {
		t.Error("the codex skills dir does not hold exactly the declared skill")
	}

	assertLinks(t, realHome, home, ".codex/auth.json")

	if !strings.Contains(readFile(t, filepath.Join(home, "env.txt")), "INITIAL_AGENT_MODE=agent-full-access\n") {
		t.Error("the codex executor does not start in full access")
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

	want := `level=WARN msg="skipping MCP: variable unset" job=j server=example var=` + mcpVarName
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
		"skills on pi":     {"name: k\nagents:\n  a: {harness: pi, model: m, skills: [./s]}\n" + job, nil, errSkillDirs},
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
