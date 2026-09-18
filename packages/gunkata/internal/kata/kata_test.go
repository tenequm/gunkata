package kata

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const kataPerm = 0o600

// repoRoot is where the shipped katas live, relative to this package.
const repoRoot = "../../../.."

func writeKata(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "k.kata.yml")
	if err := os.WriteFile(path, []byte(body), kataPerm); err != nil {
		t.Fatalf("write kata: %v", err)
	}

	return path
}

// TestLoadShippedKatas holds every kata the repo ships, and the spec's full
// surface, to the loader.
func TestLoadShippedKatas(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(filepath.Join(repoRoot, "katas", "*.kata.yml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob katas: %v (%d found)", err, len(paths))
	}

	for _, path := range append(paths, filepath.Join(repoRoot, "docs", "full.kata.yml")) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			if _, err := Load(path); err != nil {
				t.Errorf("Load() returned error: %v", err)
			}
		})
	}
}

// TestLoadMergesAmendments pins the merge rule: scalars replace, lists append,
// options keys win.
func TestLoadMergesAmendments(t *testing.T) {
	t.Parallel()

	k, err := Load(filepath.Join(repoRoot, "docs", "full.kata.yml"))
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	check := k.Workflow["check"].Executor
	if check.Model != "claude-sonnet-5" || check.TimeoutSeconds != 900 || check.Harness != "claude" {
		t.Errorf("check executor = %+v, want the amended scalars", check)
	}

	if len(check.Skills) != 3 || !strings.HasSuffix(check.Skills[2], "/polish") {
		t.Errorf("check skills = %v, want the profile's two plus polish", check.Skills)
	}

	if check.Options["reasoning_effort"] != "high" {
		t.Errorf("check options = %v, want the profile's options", check.Options)
	}

	if k.Workflow["fetch"].Executor != nil {
		t.Error("a job without a prompt resolved an executor")
	}

	work := k.Workflow["work"].Executor
	if len(work.Skills) != 2 {
		t.Errorf("work skills = %v, amending check leaked into the base profile", work.Skills)
	}
}

func TestLoadDefaultsTheTimeout(t *testing.T) {
	t.Parallel()

	k, err := Load(writeKata(t, `
name: k
agents:
  a: {harness: claude, model: m}
workflow:
  j:
    agent: a
    prompt: go
    outputs: [out]
`))
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if got := k.Workflow["j"].Executor.TimeoutSeconds; got != DefaultTimeoutSeconds {
		t.Errorf("timeout = %d, want %d", got, DefaultTimeoutSeconds)
	}
}

func TestLoadRejectsInvalidKatas(t *testing.T) {
	t.Parallel()

	const head = "name: k\nparams:\n  p: {}\nagents:\n  a: {harness: claude, model: m}\n"

	cases := map[string]struct {
		body string
		want error
	}{
		"no name":            {"workflow:\n  j: {outputs: [o]}\n", ErrName},
		"no jobs":            {"name: k\n", ErrNoJobs},
		"bad param name":     {"name: k\nparams:\n  ../x: {}\nworkflow:\n  j: {outputs: [o]}\n", ErrParamName},
		"empty job":          {head + "workflow:\n  j:\n", ErrEmptyJob},
		"no harness":         {"name: k\nagents:\n  a: {model: m}\nworkflow:\n  j: {outputs: [o]}\n", ErrHarness},
		"no model":           {"name: k\nagents:\n  a: {harness: claude}\nworkflow:\n  j: {outputs: [o]}\n", ErrModel},
		"negative timeout":   {"name: k\nagents:\n  a: {harness: c, model: m, timeout_seconds: -1}\nworkflow:\n  j: {outputs: [o]}\n", ErrTimeout},
		"prompt no agent":    {head + "workflow:\n  j: {prompt: x, outputs: [o]}\n", ErrAgentPrompt},
		"agent no prompt":    {head + "workflow:\n  j: {agent: a, outputs: [o]}\n", ErrAgentPrompt},
		"unknown profile":    {head + "workflow:\n  j: {agent: b, prompt: x, outputs: [o]}\n", ErrUnknownAgent},
		"amend harness":      {head + "workflow:\n  j: {agent: {profile: a, harness: codex}, prompt: x, outputs: [o]}\n", ErrAmendHarness},
		"amend unknown":      {head + "workflow:\n  j: {agent: {profile: a, color: red}, prompt: x, outputs: [o]}\n", ErrAmendField},
		"no evidence":        {head + "workflow:\n  j: {pre-steps: [\"true\"]}\n", ErrNoEvidence},
		"nested output":      {head + "workflow:\n  j: {outputs: [a/b]}\n", ErrOutputName},
		"duplicate output":   {head + "workflow:\n  j: {outputs: [o, o]}\n", ErrDupOutput},
		"unknown need":       {head + "workflow:\n  j: {needs: [x], outputs: [o]}\n", ErrUnknownNeed},
		"cycle":              {head + "workflow:\n  a: {needs: [b], outputs: [o]}\n  b: {needs: [a], outputs: [o]}\n", ErrCycle},
		"self cycle":         {head + "workflow:\n  a: {needs: [a], outputs: [o]}\n", ErrCycle},
		"unknown kind":       {head + "workflow:\n  j: {post-steps: [\"cat {{input:x}}\"]}\n", ErrPlaceholder},
		"undeclared param":   {head + "workflow:\n  j: {post-steps: [\"cat {{param:q}}\"]}\n", ErrUnknownParam},
		"undeclared output":  {head + "workflow:\n  j: {outputs: [o], post-steps: [\"cat {{output:x}}\"]}\n", ErrUnknownOut},
		"artifact no slash":  {head + "workflow:\n  i: {outputs: [o]}\n  j: {post-steps: [\"cat {{artifact:i}}\"]}\n", ErrArtifactRef},
		"artifact no output": {head + "workflow:\n  i: {outputs: [o]}\n  j: {post-steps: [\"cat {{artifact:i/x}}\"]}\n", ErrUnknownOut},
		"shell pipe":         {head + "workflow:\n  j: {post-steps: [\"cat a | wc\"]}\n", ErrShell},
		"shell var":          {head + "workflow:\n  j: {post-steps: [\"echo $HOME\"]}\n", ErrShell},
		"empty argv":         {head + "workflow:\n  j: {post-steps: [[]]}\n", ErrStep},
		"open quote":         {head + "workflow:\n  j: {post-steps: [\"echo 'a\"]}\n", ErrStep},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := Load(writeKata(t, tc.body))
			if !errors.Is(err, tc.want) {
				t.Errorf("Load() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	_, err := Load(writeKata(t, "name: k\nworkflow:\n  j: {outputs: [o], retry: 3}\n"))
	if err == nil || !strings.Contains(err.Error(), "retry") {
		t.Errorf("Load() error = %v, want the unknown field named", err)
	}
}

func TestSplit(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"git clone a b":               {"git", "clone", "a", "b"},
		`grep -qx "hello world" f`:    {"grep", "-qx", "hello world", "f"},
		`echo 'it''s'  "" x`:          {"echo", "its", "", "x"},
		"  spaced\targs\n":            {"spaced", "args"},
		`test -s "{{output:a b.md}}"`: {"test", "-s", "{{output:a b.md}}"},
	}

	for line, want := range cases {
		got, err := Split(line)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("Split(%q) = %q, %v; want %q", line, got, err, want)
		}
	}
}

// TestExpandKeepsArgumentsWhole pins that expansion happens after splitting:
// a value with spaces stays one argument.
func TestExpandKeepsArgumentsWhole(t *testing.T) {
	t.Parallel()

	job := &Job{Name: "j"}
	params := map[string]string{"brief": "/runs/x/params/brief/my brief.md"}

	got := ExpandAll(job, []string{"cp", "{{param:brief}}", "{{output:out}}", "{{artifact:up/repo}}"},
		params, "/runs/x/artifacts")
	want := []string{
		"cp", "/runs/x/params/brief/my brief.md",
		"/runs/x/artifacts/j/out", "/runs/x/artifacts/up/repo",
	}

	if !slices.Equal(got, want) {
		t.Errorf("ExpandAll() = %q, want %q", got, want)
	}
}
