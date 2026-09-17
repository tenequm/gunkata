package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Test literals, named because the lint config forbids bare ones.
const (
	scriptPerm   = 0o700
	graphPerm    = 0o600
	newline      = "\n"
	once         = 1
	never        = 0
	exitZero     = 0
	stubFailExit = 3
	// killedExit is what the record states for a process the engine killed.
	killedExit    = -1
	checkExit     = 7
	nodeProduce   = "produce"
	nodeGate      = "gate"
	nodeConsume   = "consume"
	seedName      = "seed.txt"
	seedBody      = "seeded" + newline
	derivedBody   = "seeded consumed" + newline
	produceRef    = "artifacts/produce.txt"
	consumeRef    = "artifacts/consume.txt"
	outcomeFmt    = "outcome = %q, want %q"
	fieldStarted  = "started_at"
	fieldFinished = "finished_at"
	testGrace     = 500 * time.Millisecond
	reapWait      = 5 * time.Second
	pollWait      = 50 * time.Millisecond
)

// stubScript is a test double for acpx (lock 7: no test involves a model). It
// records its argv and environment in the engine-owned HOME it was given, then
// acts on the directives the test graph wrote into the prompt.
const stubScript = `#!/usr/bin/env bash
set -u

prompt="${@: -1}"
printf '%s\n' "$@" > "$HOME/argv.txt"
env | sort > "$HOME/env.txt"
echo "stub acpx stdout marker"
echo "stub acpx stderr marker" >&2

field() {
  printf '%s\n' "$prompt" | sed -n "s/^$1=//p" | head -n 1
}

action="$(field ACTION)"
target="$(field TARGET)"
content="$(field CONTENT)"
origin="$(field SOURCE)"
pidfile="$(field PIDFILE)"

case "$action" in
  write)  printf '%s\n' "$content" > "$target" ;;
  derive) printf '%s consumed\n' "$(cat "$origin")" > "$target" ;;
  empty)  : > "$target" ;;
  none)   echo "wrote nothing" ;;
  fail)   echo "stub failing on purpose" >&2; exit 3 ;;
  hang)
    sleep 600 &
    child=$!
    printf '%s\n%s\n' "$$" "$child" > "$pidfile"
    wait "$child"
    ;;
  *)      echo "stub: unknown ACTION '$action'" >&2; exit 64 ;;
esac
`

// passGraph mirrors the starter corpus pass variant, with stub directives in
// place of the prompts a model would read.
const passGraph = `
name: stub-pass
defaults:
  agent: /nonexistent/agent
  model: stub-model
  timeout_seconds: 7
nodes:
  - name: produce
    prompt: |
      ACTION=write
      TARGET={{artifact}}
      CONTENT=hello from gunkata
    artifact: produce.txt
  - name: gate
    needs: [produce]
    check: ["grep", "-qx", "hello from gunkata", "{{artifact:produce}}"]
  - name: consume
    needs: [gate]
    prompt: |
      ACTION=derive
      SOURCE={{artifact:produce}}
      TARGET={{artifact}}
    artifact: consume.txt
    check: ["grep", "-qx", "hello from gunkata consumed", "{{artifact}}"]
`

// builtinGraph overrides the default agent with an acpx built-in mode, the
// other of the two invocation shapes.
const builtinGraph = `
name: stub-builtin
defaults:
  agent: /nonexistent/agent
  model: stub-model
  timeout_seconds: 7
nodes:
  - name: produce
    agent: acpx:pi
    prompt: |
      ACTION=write
      TARGET={{artifact}}
      CONTENT=hello from gunkata
    artifact: produce.txt
`

// hangGraph spawns a child and waits, so a test can watch the engine take the
// whole process group down.
const hangGraph = `
name: stub-hang
defaults:
  agent: /nonexistent/agent
  model: stub-model
  timeout_seconds: 1
nodes:
  - name: hang
    prompt: |
      ACTION=hang
      PIDFILE={{artifact}}
    artifact: pids.txt
`

// inputGraph gates on an input file the engine copied in, then derives an
// artifact from it, so both the check and the prompt expand {{input:...}}.
const inputGraph = `
name: stub-input
defaults:
  agent: /nonexistent/agent
  model: stub-model
  timeout_seconds: 7
inputs:
  - seed.txt
nodes:
  - name: gate
    check: ["grep", "-qx", "seeded", "{{input:seed.txt}}"]
  - name: consume
    needs: [gate]
    prompt: |
      ACTION=derive
      SOURCE={{input:seed.txt}}
      TARGET={{artifact}}
    artifact: consume.txt
    check: ["grep", "-qx", "seeded consumed", "{{artifact}}"]
`

var runIDPattern = regexp.MustCompile(`^\d{8}T\d{6}Z[0-9a-f]{4}$`)

// recordFields is the record's node object, field for field.
var recordFields = []string{
	"artifact", "attempts", "executor_exit", fieldFinished, "gate_exit",
	fieldStarted, "state",
}

// stubACPX puts the stub on PATH and poisons the environment, so a test can
// prove what does and does not cross the executor boundary.
func stubACPX(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, acpxBin),
		[]byte(stubScript), scriptPerm); err != nil {
		t.Fatalf("write stub acpx: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GUNKATA_POISON", "must-not-cross")
}

func writeGraph(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "graph.yaml")
	if err := os.WriteFile(path, []byte(body), graphPerm); err != nil {
		t.Fatalf("write graph: %v", err)
	}

	return path
}

// runGraphFile runs body and fails the test if the run could not be carried
// out at all. A parked run is a result, not a failure.
func runGraphFile(t *testing.T, body string) Result {
	t.Helper()

	res, err := Run(context.Background(), Options{
		GraphPath: writeGraph(t, body),
		RunsRoot:  filepath.Join(t.TempDir(), "runs"),
	})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	return res
}

// runWithInputs runs body with the given bindings and hands back whatever Run
// concluded, so a caller can assert on the result or on the error.
func runWithInputs(
	t *testing.T, body string, inputs map[string]string,
) (Result, error) {
	t.Helper()

	return Run(context.Background(), Options{
		GraphPath: writeGraph(t, body),
		RunsRoot:  filepath.Join(t.TempDir(), "runs"),
		Inputs:    inputs,
	})
}

// writeSource writes a file for an --input binding to point at.
func writeSource(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), seedName)
	if err := os.WriteFile(path, []byte(content), graphPerm); err != nil {
		t.Fatalf("write input source: %v", err)
	}

	return path
}

type wantNode struct {
	state    nodeState
	attempts int
	ran      bool
	execExit any
	gateExit any
	artifact any
}

// assertRecord holds record.json to the shared contract, field for field.
func assertRecord(
	t *testing.T, res Result, graphName string, want map[string]wantNode,
) {
	t.Helper()

	rec := readRecord(t, res.RunDir)

	assertKeys(t, "record", rec, []string{
		fieldFinished, "graph", "nodes", "outcome", "run_id", fieldStarted,
	})
	assertString(t, "run_id", rec["run_id"], filepath.Base(res.RunDir))
	assertString(t, "graph", rec["graph"], graphName)
	assertString(t, "outcome", rec["outcome"], string(res.Outcome))
	assertStamp(t, fieldStarted, rec[fieldStarted])
	assertStamp(t, fieldFinished, rec[fieldFinished])

	nodes, ok := rec["nodes"].(map[string]any)
	if !ok {
		t.Fatalf("record nodes = %T, want an object", rec["nodes"])
	}

	names := make([]string, emptyLen, len(want))
	for name := range want {
		names = append(names, name)
	}

	assertKeys(t, "nodes", nodes, names)

	for name, wanted := range want {
		node, ok := nodes[name].(map[string]any)
		if !ok {
			t.Fatalf("node %s = %T, want an object", name, nodes[name])
		}

		assertNode(t, name, node, wanted)
	}
}

func readRecord(t *testing.T, runDir string) map[string]any {
	t.Helper()

	var rec map[string]any
	if err := json.Unmarshal(
		[]byte(readFile(t, filepath.Join(runDir, recordName))), &rec,
	); err != nil {
		t.Fatalf("parse record: %v", err)
	}

	return rec
}

func assertNode(t *testing.T, name string, node map[string]any, want wantNode) {
	t.Helper()

	assertKeys(t, "node "+name, node, recordFields)
	assertString(t, name+".state", node["state"], string(want.state))
	assertNumberOrNull(t, name+".attempts", node["attempts"], want.attempts)
	assertNodeStamps(t, name, node, want)
	assertNumberOrNull(t, name+".executor_exit",
		node["executor_exit"], want.execExit)
	assertNumberOrNull(t, name+".gate_exit", node["gate_exit"], want.gateExit)

	if want.artifact == nil {
		assertNull(t, name+".artifact", node["artifact"])

		return
	}

	assertString(t, name+".artifact", node["artifact"], want.artifact)
}

// assertNodeStamps insists a node that never started says so: a pending node
// carries no timestamps at all.
func assertNodeStamps(
	t *testing.T, name string, node map[string]any, want wantNode,
) {
	t.Helper()

	for _, field := range []string{fieldStarted, fieldFinished} {
		if want.ran {
			assertStamp(t, name+"."+field, node[field])

			continue
		}

		assertNull(t, name+"."+field, node[field])
	}
}

func assertKeys(t *testing.T, what string, obj map[string]any, want []string) {
	t.Helper()

	got := make([]string, emptyLen, len(obj))
	for key := range obj {
		got = append(got, key)
	}

	slices.Sort(got)

	sorted := slices.Clone(want)
	slices.Sort(sorted)

	if !slices.Equal(got, sorted) {
		t.Errorf("%s fields = %v, want %v", what, got, sorted)
	}
}

func assertString(t *testing.T, what string, got, want any) {
	t.Helper()

	if got != want {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func assertNull(t *testing.T, what string, got any) {
	t.Helper()

	if got != nil {
		t.Errorf("%s = %v, want null", what, got)
	}
}

// assertNumberOrNull compares a JSON number against an int the test wants, or
// insists on null when it wants nothing.
func assertNumberOrNull(t *testing.T, what string, got, want any) {
	t.Helper()

	if want == nil {
		assertNull(t, what, got)

		return
	}

	code, ok := want.(int)
	if !ok {
		t.Fatalf("%s: test wants a %T, want an int", what, want)
	}

	if got != float64(code) {
		t.Errorf("%s = %v, want %d", what, got, code)
	}
}

func assertStamp(t *testing.T, what string, got any) {
	t.Helper()

	text, ok := got.(string)
	if !ok {
		t.Fatalf("%s = %v, want an RFC3339 string", what, got)
	}

	if _, err := time.Parse(time.RFC3339, text); err != nil {
		t.Errorf("%s = %q, does not parse as RFC3339: %v", what, text, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(raw)
}

func TestRunPassVariant(t *testing.T) {
	stubACPX(t)

	res := runGraphFile(t, passGraph)

	if res.Outcome != OutcomeSucceeded {
		t.Errorf(outcomeFmt, res.Outcome, OutcomeSucceeded)
	}

	if !runIDPattern.MatchString(filepath.Base(res.RunDir)) {
		t.Errorf("run id %q does not match %v",
			filepath.Base(res.RunDir), runIDPattern)
	}

	assertRecord(t, res, "stub-pass", map[string]wantNode{
		nodeProduce: {
			state: stateDone, attempts: once, ran: true,
			execExit: exitZero, gateExit: nil, artifact: produceRef,
		},
		nodeGate: {
			state: stateDone, attempts: once, ran: true,
			execExit: nil, gateExit: exitZero, artifact: nil,
		},
		nodeConsume: {
			state: stateDone, attempts: once, ran: true,
			execExit: exitZero, gateExit: exitZero, artifact: consumeRef,
		},
	})

	produced := readFile(t, filepath.Join(res.RunDir, produceRef))
	if produced != "hello from gunkata"+newline {
		t.Errorf("produce.txt = %q", produced)
	}

	consumed := readFile(t, filepath.Join(res.RunDir, consumeRef))
	if consumed != "hello from gunkata consumed"+newline {
		t.Errorf("consume.txt = %q", consumed)
	}
}

func TestRunCopiesGraphAndLogsExecutorOutput(t *testing.T) {
	stubACPX(t)

	path := writeGraph(t, passGraph)

	res, err := Run(context.Background(), Options{
		GraphPath: path,
		RunsRoot:  filepath.Join(t.TempDir(), "runs"),
	})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	copied := readFile(t, filepath.Join(res.RunDir, graphName))
	if copied != readFile(t, path) {
		t.Error("graph.yaml in the run dir is not a verbatim copy")
	}

	log := readFile(t,
		filepath.Join(res.RunDir, nodesDir, nodeProduce, logName))
	for _, marker := range []string{"stdout marker", "stderr marker"} {
		if !strings.Contains(log, marker) {
			t.Errorf("executor.log is missing the %s", marker)
		}
	}
}

func TestRunCreatesNodeDirsOnlyForExecutors(t *testing.T) {
	stubACPX(t)

	res := runGraphFile(t, passGraph)

	for _, name := range []string{nodeProduce, nodeConsume} {
		for _, dir := range []string{homeDir, workDir} {
			path := filepath.Join(res.RunDir, nodesDir, name, dir)
			if !isDir(path) {
				t.Errorf("%s is missing", path)
			}
		}
	}

	if exists(filepath.Join(res.RunDir, nodesDir, nodeGate)) {
		t.Error("check-only node gate got a nodes/ directory")
	}

	if exists(filepath.Join(res.RunDir, inputsDir)) {
		t.Error("a graph with no inputs got an inputs/ directory")
	}
}

func TestRunStartsTheExecutorBare(t *testing.T) {
	stubACPX(t)

	res := runGraphFile(t, passGraph)
	home := filepath.Join(res.RunDir, nodesDir, nodeProduce, homeDir)

	argv := strings.Split(strings.TrimRight(
		readFile(t, filepath.Join(home, "argv.txt")), newline), newline)

	want := []string{
		"--agent", "/nonexistent/agent",
		"--cwd", filepath.Join(res.RunDir, nodesDir, nodeProduce, workDir),
		"--model", "stub-model",
		"--timeout", "7",
		"--approve-all",
		"--format", "quiet",
		"exec",
	}

	if !slices.Equal(argv[:len(want)], want) {
		t.Errorf("acpx argv = %v, want it to start with %v", argv, want)
	}

	prompt := strings.Join(argv[len(want):], newline)
	target := "TARGET=" + filepath.Join(res.RunDir, produceRef)

	if !strings.Contains(prompt, target) {
		t.Errorf("prompt argument = %q, want it to carry %q", prompt, target)
	}

	assertExecutorEnv(t, home)
}

// TestRunStartsAnACPXBuiltinAgent holds the other invocation shape: a node
// whose agent names an acpx mode is started as `acpx <mode> exec`, with no
// --agent, and the mode sits after the global flags where acpx requires it.
func TestRunStartsAnACPXBuiltinAgent(t *testing.T) {
	stubACPX(t)

	res := runGraphFile(t, builtinGraph)
	home := filepath.Join(res.RunDir, nodesDir, nodeProduce, homeDir)

	argv := strings.Split(strings.TrimRight(
		readFile(t, filepath.Join(home, "argv.txt")), newline), newline)

	want := []string{
		"--cwd", filepath.Join(res.RunDir, nodesDir, nodeProduce, workDir),
		"--model", "stub-model",
		"--timeout", "7",
		"--approve-all",
		"--format", "quiet",
		"pi",
		"exec",
	}

	if !slices.Equal(argv[:len(want)], want) {
		t.Errorf("acpx argv = %v, want it to start with %v", argv, want)
	}
}

// TestPrepareHomeLinksOneFamilysCredentials holds the bare-executor contract
// where it is easiest to break: a node inherits the credentials of its own
// agent family and nothing else that hangs off the same home.
func TestPrepareHomeLinksOneFamilysCredentials(t *testing.T) {
	realHome := t.TempDir()

	for _, rel := range []string{
		".gemini/antigravity-acp/settings.json",
		".gemini/antigravity-acp/acp_token.json",
		".local/lib/antigravity-acp",
		".pi/agent/models.json",
		".pi/agent/mcp.json",
		".pi/agent/skills",
		".codex/auth.json",
		".codex/config.toml",
	} {
		writeCredential(t, filepath.Join(realHome, rel))
	}

	t.Setenv("HOME", realHome)

	cases := map[string]struct {
		agent  string
		linked []string
		absent []string
	}{
		"agy": {
			agent: "/nonexistent/agy-acp-server",
			linked: []string{
				".gemini/antigravity-acp/settings.json",
				".gemini/antigravity-acp/acp_token.json",
				".local/lib/antigravity-acp",
			},
			absent: []string{".pi/agent/models.json", ".codex/auth.json"},
		},
		"pi": {
			agent:  "acpx:pi",
			linked: []string{".pi/agent/models.json"},
			absent: []string{
				".pi/agent/mcp.json",
				".pi/agent/skills",
				".gemini/antigravity-acp/acp_token.json",
			},
		},
		"codex": {
			agent:  "acpx:codex",
			linked: []string{".codex/auth.json"},
			absent: []string{".codex/config.toml", ".pi/agent/models.json"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			home := filepath.Join(t.TempDir(), homeDir)
			if err := prepareHome(home, filepath.Join(home, workDir),
				tc.agent); err != nil {
				t.Fatalf("prepareHome() returned error: %v", err)
			}

			for _, rel := range tc.linked {
				if !exists(filepath.Join(home, rel)) {
					t.Errorf("%s was not linked into the node home", rel)
				}
			}

			for _, rel := range tc.absent {
				if exists(filepath.Join(home, rel)) {
					t.Errorf("%s crossed the executor boundary", rel)
				}
			}
		})
	}
}

func writeCredential(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), scriptPerm); err != nil {
		t.Fatalf("create credential dir: %v", err)
	}

	if err := os.WriteFile(path, []byte("{}"), graphPerm); err != nil {
		t.Fatalf("write credential: %v", err)
	}
}

// assertExecutorEnv holds the executor boundary: the engine-owned home and the
// whitelist, and nothing the parent environment happened to carry.
func assertExecutorEnv(t *testing.T, home string) {
	t.Helper()

	env := map[string]string{}

	for line := range strings.SplitSeq(
		readFile(t, filepath.Join(home, "env.txt")), newline) {
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

	assertEnvWhitelist(t, env, want)
}

func assertEnvWhitelist(t *testing.T, env, own map[string]string) {
	t.Helper()

	// bash sets the rest of these itself; nothing else may appear.
	allowed := append([]string{"PWD", "OLDPWD", "SHLVL", "_"}, inherited...)
	for key := range own {
		allowed = append(allowed, key)
	}

	for key := range env {
		if !slices.Contains(allowed, key) {
			t.Errorf("%s crossed the executor boundary", key)
		}
	}
}

func TestRunFailVariantParksGate(t *testing.T) {
	stubACPX(t)

	body := strings.Replace(passGraph,
		"CONTENT=hello from gunkata", "CONTENT=wrong content", once)
	body = strings.Replace(body, "name: stub-pass", "name: stub-fail", once)

	res := runGraphFile(t, body)

	if res.Outcome != OutcomeParked {
		t.Errorf(outcomeFmt, res.Outcome, OutcomeParked)
	}

	assertRecord(t, res, "stub-fail", map[string]wantNode{
		nodeProduce: {
			state: stateDone, attempts: once, ran: true,
			execExit: exitZero, gateExit: nil, artifact: produceRef,
		},
		nodeGate: {
			state: stateParked, attempts: once, ran: true,
			execExit: nil, gateExit: once, artifact: nil,
		},
		nodeConsume: {
			state: statePending, attempts: never, ran: false,
			execExit: nil, gateExit: nil, artifact: consumeRef,
		},
	})

	if exists(filepath.Join(res.RunDir, nodesDir, nodeConsume)) {
		t.Error("consume started: it has a nodes/ directory")
	}

	if exists(filepath.Join(res.RunDir, consumeRef)) {
		t.Error("consume started: its artifact is on disk")
	}
}

func TestRunParksOnFailedEvidence(t *testing.T) {
	cases := map[string]struct {
		action   string
		execExit int
	}{
		"executor exits non-zero": {action: "fail", execExit: stubFailExit},
		"artifact never written":  {action: "none", execExit: exitZero},
		"artifact is empty":       {action: "empty", execExit: exitZero},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stubACPX(t)

			body := strings.Replace(passGraph,
				"ACTION=write", "ACTION="+tc.action, once)

			res := runGraphFile(t, body)

			if res.Outcome != OutcomeParked {
				t.Errorf(outcomeFmt, res.Outcome, OutcomeParked)
			}

			assertRecord(t, res, "stub-pass",
				parkedAtProduce(tc.execExit))
		})
	}
}

// parkedAtProduce is the record a run leaves when produce parks: nothing
// downstream ran, and nothing was retried.
func parkedAtProduce(execExit any) map[string]wantNode {
	return map[string]wantNode{
		nodeProduce: {
			state: stateParked, attempts: once, ran: true,
			execExit: execExit, gateExit: nil, artifact: produceRef,
		},
		nodeGate: {
			state: statePending, attempts: never, ran: false,
			execExit: nil, gateExit: nil, artifact: nil,
		},
		nodeConsume: {
			state: statePending, attempts: never, ran: false,
			execExit: nil, gateExit: nil, artifact: consumeRef,
		},
	}
}

func TestRunTimeoutKillsTheProcessGroup(t *testing.T) {
	stubACPX(t)

	grace := killGrace
	killGrace = testGrace

	t.Cleanup(func() { killGrace = grace })

	res := runGraphFile(t, hangGraph)

	if res.Outcome != OutcomeParked {
		t.Errorf(outcomeFmt, res.Outcome, OutcomeParked)
	}

	assertRecord(t, res, "stub-hang", map[string]wantNode{
		"hang": {
			state: stateParked, attempts: once, ran: true,
			execExit: killedExit, gateExit: nil,
			artifact: "artifacts/pids.txt",
		},
	})

	pids := readFile(t, filepath.Join(res.RunDir, artifactsDir, "pids.txt"))
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
		if err := syscall.Kill(pid, syscall.Signal(never)); err != nil {
			return
		}

		time.Sleep(pollWait)
	}

	t.Errorf("process %d survived the engine timeout", pid)
}

func TestRunChecksFromTheRunDir(t *testing.T) {
	res := runGraphFile(t, `
name: stub-check
defaults:
  agent: /nonexistent/agent
  model: stub-model
nodes:
  - name: gate
    check: ["test", "-f", "graph.yaml"]
`)

	if res.Outcome != OutcomeSucceeded {
		t.Errorf(outcomeFmt, res.Outcome, OutcomeSucceeded)
	}

	if exists(filepath.Join(res.RunDir, nodesDir)) {
		t.Error("a check-only run created a nodes/ directory")
	}
}

func TestRunRejectsAnUnloadableGraph(t *testing.T) {
	res, err := Run(context.Background(), Options{
		GraphPath: filepath.Join(t.TempDir(), "missing.yaml"),
		RunsRoot:  filepath.Join(t.TempDir(), "runs"),
	})
	if err == nil {
		t.Fatal("Run() accepted a missing graph, want an error")
	}

	if res.RunDir != unset {
		t.Errorf("RunDir = %q, want none: no run started", res.RunDir)
	}
}

func TestRunParksWhenACPXIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	res := runGraphFile(t, passGraph)

	if res.Outcome != OutcomeParked {
		t.Errorf(outcomeFmt, res.Outcome, OutcomeParked)
	}

	assertRecord(t, res, "stub-pass", parkedAtProduce(nil))
}

func TestRunCheckReportsTheExitCode(t *testing.T) {
	code, err := runCheck(context.Background(),
		[]string{"sh", "-c", "exit 7"}, t.TempDir())
	if err != nil {
		t.Fatalf("runCheck() returned error: %v", err)
	}

	if code != checkExit {
		t.Errorf("exit code = %d, want %d", code, checkExit)
	}
}

func TestRunCheckRejectsAnEmptyArgv(t *testing.T) {
	if _, err := runCheck(context.Background(), nil, t.TempDir()); err == nil {
		t.Error("runCheck() accepted an empty argv, want an error")
	}
}

// TestRunBindsDeclaredInputs holds the whole path: the bound file is copied
// into the run, a check and a prompt both see it there, and the record states
// where it came from.
func TestRunBindsDeclaredInputs(t *testing.T) {
	stubACPX(t)

	src := writeSource(t, seedBody)

	res, err := runWithInputs(t, inputGraph, map[string]string{seedName: src})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if res.Outcome != OutcomeSucceeded {
		t.Errorf(outcomeFmt, res.Outcome, OutcomeSucceeded)
	}

	copied := filepath.Join(res.RunDir, inputsDir, seedName)
	if got := readFile(t, copied); got != seedBody {
		t.Errorf("inputs/%s = %q, want %q", seedName, got, seedBody)
	}

	assertCopiedInput(t, copied)

	if got := readFile(t, filepath.Join(res.RunDir, consumeRef)); got !=
		derivedBody {
		t.Errorf("consume.txt = %q, want %q", got, derivedBody)
	}

	rec := readRecord(t, res.RunDir)

	inputs, ok := rec["inputs"].(map[string]any)
	if !ok {
		t.Fatalf("record inputs = %T, want an object", rec["inputs"])
	}

	assertString(t, "inputs."+seedName, inputs[seedName], src)
}

// assertCopiedInput insists the run owns its own copy: a regular file, not a
// link to the source, and readable only by the engine.
func assertCopiedInput(t *testing.T, path string) {
	t.Helper()

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("stat copied input: %v", err)
	}

	if !info.Mode().IsRegular() {
		t.Errorf("inputs/%s is %v, want a regular file",
			seedName, info.Mode().Type())
	}

	if info.Mode().Perm() != os.FileMode(filePerm) {
		t.Errorf("inputs/%s mode = %v, want %v",
			seedName, info.Mode().Perm(), os.FileMode(filePerm))
	}
}

// TestRunRejectsBadInputBindings covers every way a binding fails. Each is an
// error rather than a parked run, and nothing of the graph starts.
func TestRunRejectsBadInputBindings(t *testing.T) {
	seed := writeSource(t, seedBody)
	empty := writeSource(t, unset)
	missing := filepath.Join(t.TempDir(), "gone.txt")

	cases := map[string]map[string]string{
		"declared input unbound": {},
		"undeclared name bound":  {seedName: seed, "extra.txt": seed},
		"source missing":         {seedName: missing},
		"source empty":           {seedName: empty},
		"source is a directory":  {seedName: t.TempDir()},
	}

	for name, inputs := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := runWithInputs(t, inputGraph, inputs)
			if err == nil {
				t.Fatalf("Run() accepted %v, want an error", inputs)
			}

			if exists(filepath.Join(res.RunDir, nodesDir)) {
				t.Error("a node started before the inputs were bound")
			}
		})
	}
}

func TestRunRejectsInputsAGraphDoesNotDeclare(t *testing.T) {
	stubACPX(t)

	_, err := runWithInputs(t, passGraph,
		map[string]string{seedName: writeSource(t, seedBody)})
	if err == nil {
		t.Fatal("Run() accepted an input for a graph that declares none")
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}
