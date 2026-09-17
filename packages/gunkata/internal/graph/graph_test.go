package graph

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test literals, named because the lint config forbids bare ones.
const (
	graphPerm   = 0o600
	nodeCount   = 3
	oneNeed     = 1
	ownTimeout  = 30
	setTimeout  = 120
	firstNeed   = 0
	nodeProduce = "produce"
	nodeGate    = "gate"
	nodeConsume = "consume"
	loadFailed  = "Load() returned error: %v"
)

const validGraph = `
name: starter-pass
defaults:
  agent: /bin/agent
  model: model-a
  timeout_seconds: 120
nodes:
  - name: produce
    prompt: |
      write one line to {{artifact}}
    artifact: produce.txt
  - name: gate
    needs: [produce]
    check: ["grep", "-qx", "hi", "{{artifact:produce}}"]
  - name: consume
    needs: [gate]
    prompt: read {{artifact:produce}} into {{artifact}}
    artifact: consume.txt
    check: ["test", "-s", "{{artifact}}"]
    model: model-b
    timeout_seconds: 30
`

// invalidGraphs is one graph per way a graph can fail validation.
var invalidGraphs = map[string]struct {
	body string
	want error
}{
	"no name": {
		body: `
defaults: {agent: /bin/a, model: m}
nodes: [{name: n, check: ["true"]}]
`,
		want: ErrGraphName,
	},
	"no agent": {
		body: `
name: t
defaults: {model: m}
nodes: [{name: n, check: ["true"]}]
`,
		want: ErrDefaultAgent,
	},
	"no model": {
		body: `
name: t
defaults: {agent: /bin/a}
nodes: [{name: n, check: ["true"]}]
`,
		want: ErrDefaultModel,
	},
	"negative default timeout": {
		body: `
name: t
defaults: {agent: /bin/a, model: m, timeout_seconds: -1}
nodes: [{name: n, check: ["true"]}]
`,
		want: ErrTimeout,
	},
	"no nodes": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes: []
`,
		want: ErrNoNodes,
	},
	"unnamed node": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes: [{check: ["true"]}]
`,
		want: ErrNodeName,
	},
	"duplicate node": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes:
  - {name: n, check: ["true"]}
  - {name: n, check: ["false"]}
`,
		want: ErrDuplicateNode,
	},
	"unknown need": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes: [{name: n, needs: [ghost], check: ["true"]}]
`,
		want: ErrUnknownNeed,
	},
	"self cycle": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes: [{name: n, needs: [n], check: ["true"]}]
`,
		want: ErrCycle,
	},
	"cycle": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes:
  - {name: a, needs: [b], check: ["true"]}
  - {name: b, needs: [a], check: ["true"]}
`,
		want: ErrCycle,
	},
	"no evidence": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes: [{name: n}]
`,
		want: ErrNoEvidence,
	},
	"negative node timeout": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes: [{name: n, check: ["true"], timeout_seconds: -5}]
`,
		want: ErrTimeout,
	},
	"absolute artifact": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes: [{name: n, artifact: /etc/passwd}]
`,
		want: ErrArtifactPath,
	},
	"escaping artifact": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes: [{name: n, artifact: ../outside.txt}]
`,
		want: ErrArtifactPath,
	},
	"reference to unknown node": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes: [{name: n, prompt: "see {{artifact:ghost}}", artifact: n.txt}]
`,
		want: ErrUnknownRef,
	},
	"reference to node without artifact": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes:
  - {name: a, check: ["true"]}
  - {name: b, prompt: "see {{artifact:a}}", artifact: b.txt}
`,
		want: ErrArtifactRef,
	},
	"own reference without artifact": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes: [{name: n, prompt: "write {{artifact}}", check: ["true"]}]
`,
		want: ErrArtifactRef,
	},
	"check reference to unknown node": {
		body: `
name: t
defaults: {agent: /bin/a, model: m}
nodes: [{name: n, check: ["test", "-s", "{{artifact:ghost}}"]}]
`,
		want: ErrUnknownRef,
	},
}

func writeGraph(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "graph.yaml")
	if err := os.WriteFile(path, []byte(body), graphPerm); err != nil {
		t.Fatalf("write graph: %v", err)
	}

	return path
}

func loadBody(t *testing.T, body string) *Graph {
	t.Helper()

	g, err := Load(writeGraph(t, body))
	if err != nil {
		t.Fatalf(loadFailed, err)
	}

	return g
}

func TestLoadValidGraph(t *testing.T) {
	t.Parallel()

	g := loadBody(t, validGraph)

	if g.Name != "starter-pass" {
		t.Errorf("name = %q, want starter-pass", g.Name)
	}

	if g.Defaults.Agent != "/bin/agent" {
		t.Errorf("defaults.agent = %q, want /bin/agent", g.Defaults.Agent)
	}

	if len(g.Nodes) != nodeCount {
		t.Fatalf("loaded %d nodes, want %d", len(g.Nodes), nodeCount)
	}

	gate := g.Node(nodeGate)
	if len(gate.Needs) != oneNeed || gate.Needs[firstNeed] != nodeProduce {
		t.Errorf("gate needs = %v, want [produce]", gate.Needs)
	}

	if g.Node("ghost") != nil {
		t.Error("Node() returned a node the graph does not declare")
	}
}

func TestLoadResolvesDefaults(t *testing.T) {
	t.Parallel()

	g := loadBody(t, validGraph)

	produce := g.Node(nodeProduce)
	if produce.Model != "model-a" {
		t.Errorf("produce model = %q, want the default model-a", produce.Model)
	}

	if produce.TimeoutSeconds != setTimeout {
		t.Errorf("produce timeout = %d, want %d",
			produce.TimeoutSeconds, setTimeout)
	}

	consume := g.Node(nodeConsume)
	if consume.Model != "model-b" {
		t.Errorf("consume model = %q, want its own model-b", consume.Model)
	}

	if consume.TimeoutSeconds != ownTimeout {
		t.Errorf("consume timeout = %d, want its own %d",
			consume.TimeoutSeconds, ownTimeout)
	}
}

func TestLoadAppliesDefaultTimeout(t *testing.T) {
	t.Parallel()

	g := loadBody(t, `
name: t
defaults:
  agent: /bin/agent
  model: m
nodes:
  - name: only
    check: ["true"]
`)

	if got := g.Node("only").TimeoutSeconds; got != DefaultTimeoutSeconds {
		t.Errorf("timeout = %d, want %d", got, DefaultTimeoutSeconds)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	t.Parallel()

	_, err := Load(writeGraph(t, `
name: t
defaults:
  agent: /bin/agent
  model: m
nodes:
  - name: only
    check: ["true"]
    retries: 3
`))
	if err == nil {
		t.Fatal("Load() accepted an unknown field, want an error")
	}

	if !strings.Contains(err.Error(), "retries") {
		t.Errorf("error %v, want it to name the unknown field", err)
	}
}

func TestLoadRejectsInvalidGraphs(t *testing.T) {
	t.Parallel()

	for name, tc := range invalidGraphs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := Load(writeGraph(t, tc.body))
			if !errors.Is(err, tc.want) {
				t.Errorf("Load() error = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestLoadStarterCorpus keeps the shipped corpus loadable without a run.
func TestLoadStarterCorpus(t *testing.T) {
	t.Parallel()

	for _, variant := range []string{"pass", "fail"} {
		t.Run(variant, func(t *testing.T) {
			t.Parallel()

			assertCorpusVariant(t, variant)
		})
	}
}

func assertCorpusVariant(t *testing.T, variant string) {
	t.Helper()

	path := filepath.Join("..", "..", "..", "..", "examples", "starter", variant+".yaml")

	g, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%s) returned error: %v", path, err)
	}

	if g.Name != "starter-"+variant {
		t.Errorf("name = %q, want starter-%s", g.Name, variant)
	}

	for _, name := range []string{nodeProduce, nodeGate, nodeConsume} {
		if g.Node(name) == nil {
			t.Errorf("%s.yaml declares no %q node", variant, name)
		}
	}
}

func TestExpand(t *testing.T) {
	t.Parallel()

	g := loadBody(t, validGraph)

	const dir = "/run/artifacts"

	produce := g.Node(nodeProduce)

	got := g.Expand(produce, produce.Prompt, dir)
	if !strings.Contains(got, filepath.Join(dir, "produce.txt")) {
		t.Errorf("expanded prompt = %q, want the artifact path", got)
	}

	if strings.Contains(got, "{{artifact") {
		t.Errorf("expanded prompt = %q, still holds a placeholder", got)
	}
}

func TestExpandAllLeavesTheGraphAlone(t *testing.T) {
	t.Parallel()

	g := loadBody(t, validGraph)
	gate := g.Node(nodeGate)

	argv := g.ExpandAll(gate, gate.Check, "/a")
	want := "/a/produce.txt"

	if last := argv[len(argv)-oneNeed]; last != want {
		t.Errorf("expanded check ends in %q, want %q", last, want)
	}

	if last := gate.Check[len(gate.Check)-oneNeed]; last !=
		"{{artifact:produce}}" {
		t.Errorf("ExpandAll() rewrote the graph's own check argv: %q", last)
	}
}

func TestExpandMixedReferences(t *testing.T) {
	t.Parallel()

	g := loadBody(t, validGraph)

	consume := g.Node(nodeConsume)
	got := strings.TrimSpace(g.Expand(consume, consume.Prompt, "/a"))
	want := "read /a/produce.txt into /a/consume.txt"

	if got != want {
		t.Errorf("expanded prompt = %q, want %q", got, want)
	}
}
