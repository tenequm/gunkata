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

	if want := k.Agents["executor"].ACPAdapter; want == "" || work.ACPAdapter != want || check.ACPAdapter != want {
		t.Errorf("acp_adapter = %q, %q; want the profile's %q", work.ACPAdapter, check.ACPAdapter, want)
	}
}

// TestLoadAmendsTheACPAdapter holds acp_adapter to the scalar rule: an
// amendment replaces it, and a profile without one leaves it unset.
func TestLoadAmendsTheACPAdapter(t *testing.T) {
	t.Parallel()

	k, err := Load(writeKata(t, `
name: k
agents:
  a: {harness: claude, model: m, acp_adapter: "@agentclientprotocol/claude-agent-acp@0.79.0"}
  b: {harness: codex, model: m}
workflow:
  pinned: {agent: a, prompt: go, outputs: [o]}
  amended: {agent: {profile: a, acp_adapter: "@agentclientprotocol/claude-agent-acp@^0.80.0"}, prompt: go, outputs: [o]}
  builtin: {agent: b, prompt: go, outputs: [o]}
`))
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	want := map[string]string{
		"pinned":  "@agentclientprotocol/claude-agent-acp@0.79.0",
		"amended": "@agentclientprotocol/claude-agent-acp@^0.80.0",
		"builtin": "",
	}
	for job, adapter := range want {
		if got := k.Workflow[job].Executor.ACPAdapter; got != adapter {
			t.Errorf("%s acp_adapter = %q, want %q", job, got, adapter)
		}
	}
}

// TestLoadAppendsTheSystemPrompt holds append_system_prompt to the list rule:
// a job's amendment appends to its profile's text as a paragraph of its own.
func TestLoadAppendsTheSystemPrompt(t *testing.T) {
	t.Parallel()

	k, err := Load(writeKata(t, `
name: k
agents:
  a:
    harness: claude
    model: m
    append_system_prompt: |
      Be terse.
  b: {harness: claude, model: m}
workflow:
  profile: {agent: a, prompt: go, outputs: [o]}
  amended: {agent: {profile: a, append_system_prompt: Cite sources.}, prompt: go, outputs: [o]}
  only: {agent: {profile: b, append_system_prompt: Cite sources.}, prompt: go, outputs: [o]}
  unset: {agent: b, prompt: go, outputs: [o]}
`))
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	want := map[string]string{
		"profile": "Be terse.\n",
		"amended": "Be terse.\n\nCite sources.",
		"only":    "Cite sources.",
		"unset":   "",
	}
	for job, text := range want {
		if got := k.Workflow[job].Executor.AppendSystemPrompt; got != text {
			t.Errorf("%s append_system_prompt = %q, want %q", job, got, text)
		}
	}
}

func TestLoadParsesBothMCPForms(t *testing.T) {
	t.Parallel()

	k, err := Load(writeKata(t, `
name: k
agents:
  a:
    harness: claude
    model: m
    mcps:
      - https://a.example/mcp
      - {url: https://b.example/mcp, required: true}
workflow:
  j: {outputs: [o]}
`))
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	want := []mcpEntry{{URL: "https://a.example/mcp"}, {URL: "https://b.example/mcp", Required: true}}
	if got := k.Agents["a"].MCPs; !slices.Equal(got, want) {
		t.Errorf("mcps = %+v, want %+v", got, want)
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

	// fan spells a workflow whose head h fans out; tmpl is a plain template.
	const tmpl = "  t: {outputs: [o]}\n"

	fan := func(body string) string {
		return "workflow:\n  h:\n    outputs: [o]\n    fan-out:\n      " +
			body + "\n      max_rounds: 2\n      max_items: 2\n"
	}

	cases := map[string]struct {
		body string
		want error
	}{
		"no name":              {"workflow:\n  j: {outputs: [o]}\n", ErrName},
		"no jobs":              {"name: k\n", ErrNoJobs},
		"bad param name":       {"name: k\nparams:\n  ../x: {}\nworkflow:\n  j: {outputs: [o]}\n", ErrParamName},
		"empty job":            {head + "workflow:\n  j:\n", ErrEmptyJob},
		"no harness":           {"name: k\nagents:\n  a: {model: m}\nworkflow:\n  j: {outputs: [o]}\n", ErrHarness},
		"no model":             {"name: k\nagents:\n  a: {harness: claude}\nworkflow:\n  j: {outputs: [o]}\n", ErrModel},
		"negative timeout":     {"name: k\nagents:\n  a: {harness: c, model: m, timeout_seconds: -1}\nworkflow:\n  j: {outputs: [o]}\n", ErrTimeout},
		"prompt no agent":      {head + "workflow:\n  j: {prompt: x, outputs: [o]}\n", ErrAgentPrompt},
		"agent no prompt":      {head + "workflow:\n  j: {agent: a, outputs: [o]}\n", ErrAgentPrompt},
		"unknown profile":      {head + "workflow:\n  j: {agent: b, prompt: x, outputs: [o]}\n", ErrUnknownAgent},
		"amend harness":        {head + "workflow:\n  j: {agent: {profile: a, harness: codex}, prompt: x, outputs: [o]}\n", ErrAmendHarness},
		"amend unknown":        {head + "workflow:\n  j: {agent: {profile: a, color: red}, prompt: x, outputs: [o]}\n", ErrAmendField},
		"no evidence":          {head + "workflow:\n  j: {pre-steps: [\"true\"]}\n", ErrNoEvidence},
		"nested output":        {head + "workflow:\n  j: {outputs: [a/b]}\n", ErrOutputName},
		"message, no prompt":   {head + "workflow:\n  j: {outputs: [message.md]}\n", ErrMessageJob},
		"duplicate output":     {head + "workflow:\n  j: {outputs: [o, o]}\n", ErrDupOutput},
		"unknown need":         {head + "workflow:\n  j: {needs: [x], outputs: [o]}\n", ErrUnknownNeed},
		"cycle":                {head + "workflow:\n  a: {needs: [b], outputs: [o]}\n  b: {needs: [a], outputs: [o]}\n", ErrCycle},
		"self cycle":           {head + "workflow:\n  a: {needs: [a], outputs: [o]}\n", ErrCycle},
		"unknown kind":         {head + "workflow:\n  j: {post-steps: [\"cat {{input:x}}\"]}\n", ErrPlaceholder},
		"undeclared param":     {head + "workflow:\n  j: {post-steps: [\"cat {{param:q}}\"]}\n", ErrUnknownParam},
		"system prompt param":  {"name: k\nagents:\n  a: {harness: claude, model: m, append_system_prompt: \"{{param:q}}\"}\nworkflow:\n  j: {agent: a, prompt: x, outputs: [o]}\n", ErrUnknownParam},
		"undeclared output":    {head + "workflow:\n  j: {outputs: [o], post-steps: [\"cat {{output:x}}\"]}\n", ErrUnknownOut},
		"artifact no slash":    {head + "workflow:\n  i: {outputs: [o]}\n  j: {post-steps: [\"cat {{artifact:i}}\"]}\n", ErrArtifactRef},
		"artifact no output":   {head + "workflow:\n  i: {outputs: [o]}\n  j: {post-steps: [\"cat {{artifact:i/x}}\"]}\n", ErrUnknownOut},
		"shell pipe":           {head + "workflow:\n  j: {post-steps: [\"cat a | wc\"]}\n", ErrShell},
		"shell var":            {head + "workflow:\n  j: {post-steps: [\"echo $HOME\"]}\n", ErrShell},
		"empty argv":           {head + "workflow:\n  j: {post-steps: [[]]}\n", ErrStep},
		"MCP unknown key":      {"name: k\nagents:\n  a: {harness: c, model: m, mcps: [{url: u, optional: true}]}\nworkflow:\n  j: {outputs: [o]}\n", ErrMCPField},
		"MCP no url":           {"name: k\nagents:\n  a: {harness: c, model: m, mcps: [{required: true}]}\nworkflow:\n  j: {outputs: [o]}\n", ErrMCPURL},
		"open quote":           {head + "workflow:\n  j: {post-steps: [\"echo 'a\"]}\n", ErrStep},
		"adapter two words":    {"name: k\nagents:\n  a: {harness: claude, model: m, acp_adapter: \"pkg --flag\"}\nworkflow:\n  j: {outputs: [o]}\n", ErrACPAdapter},
		"adapter npx flag":     {"name: k\nagents:\n  a: {harness: claude, model: m, acp_adapter: \"-y\"}\nworkflow:\n  j: {outputs: [o]}\n", ErrACPAdapter},
		"adapter shell":        {"name: k\nagents:\n  a: {harness: claude, model: m, acp_adapter: \"pkg@1;rm\"}\nworkflow:\n  j: {outputs: [o]}\n", ErrACPAdapter},
		"adapter quote":        {"name: k\nagents:\n  a: {harness: claude, model: m, acp_adapter: \"pkg@'1'\"}\nworkflow:\n  j: {outputs: [o]}\n", ErrACPAdapter},
		"adapter path":         {"name: k\nagents:\n  a: {harness: claude, model: m, acp_adapter: ../pkg}\nworkflow:\n  j: {outputs: [o]}\n", ErrACPAdapter},
		"amend bad adapter":    {head + "workflow:\n  j: {agent: {profile: a, acp_adapter: \"a b\"}, prompt: x, outputs: [o]}\n", ErrACPAdapter},
		"bad job name":         {head + "workflow:\n  a/b: {outputs: [o]}\n", ErrJobName},
		"fan-out items":        {head + fan("items: x\n      job: t") + tmpl, ErrFanItems},
		"fan-out unknown job":  {head + fan("items: o\n      job: nope") + tmpl, ErrFanJob},
		"fan-out self":         {head + fan("items: o\n      job: h") + tmpl, ErrFanSelf},
		"fan-out no rounds":    {head + "workflow:\n  h: {outputs: [o], fan-out: {items: o, job: t, max_items: 1}}\n" + tmpl, ErrFanRounds},
		"fan-out no items":     {head + "workflow:\n  h: {outputs: [o], fan-out: {items: o, job: t, max_rounds: 1}}\n" + tmpl, ErrFanMaxItems},
		"fan-out shared":       {head + fan("items: o\n      job: t") + "  g: {outputs: [o], fan-out: {items: o, job: t, max_rounds: 1, max_items: 1}}\n" + tmpl, ErrFanShared},
		"template needs":       {head + fan("items: o\n      job: t") + "  t: {needs: [h], outputs: [o]}\n", ErrFanNeeds},
		"template needed":      {head + fan("items: o\n      job: t") + tmpl + "  z: {needs: [t], outputs: [o]}\n", ErrFanNeeded},
		"template fans out":    {head + fan("items: o\n      job: t") + "  t: {outputs: [o], fan-out: {items: o, job: u, max_rounds: 1, max_items: 1}}\n  u: {outputs: [o]}\n", ErrFanNested},
		"fanout ref":           {head + "workflow:\n  i: {outputs: [o]}\n  j: {post-steps: [\"cat {{fanout:i}}\"]}\n", ErrFanOutRef},
		"artifact of a head":   {head + fan("items: o\n      job: t") + tmpl + "  z: {post-steps: [\"cat {{artifact:h/o}}\"]}\n", ErrFanArtifact},
		"item outside":         {head + "workflow:\n  j: {post-steps: [\"cat {{item}}\"]}\n", ErrItemRef},
		"template owns parked": {head + fan("items: o\n      job: t") + "  t: {outputs: [parked.md]}\n", ErrParkedOwned},
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
