// Package graph loads and validates gunkata work graphs.
package graph

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultTimeoutSeconds bounds an executor when neither the node nor the
// graph defaults declare a timeout.
const DefaultTimeoutSeconds = 240

// The lint config forbids bare literals, so the ones this package repeats are
// named here.
const (
	unset      = ""
	emptyLen   = 0
	minTimeout = 0
	// Submatch groups of placeholder, and the kind that is not an artifact.
	wholeMatch   = 0
	kindGroup    = 1
	refNameGroup = 2
	kindInput    = "input"
	nodeErrFmt   = "node %q: %w"
	inputErrFmt  = "input %q: %w"
)

// Validation failures. Each is wrapped with the offending node's name.
var (
	ErrGraphName     = errors.New("graph declares no name")
	ErrDefaultAgent  = errors.New("defaults.agent is required")
	ErrDefaultModel  = errors.New("defaults.model is required")
	ErrNoNodes       = errors.New("graph declares no nodes")
	ErrNodeName      = errors.New("node declares no name")
	ErrDuplicateNode = errors.New("duplicate node name")
	ErrUnknownNeed   = errors.New("needs names an unknown node")
	ErrCycle         = errors.New("graph contains a cycle")
	ErrNoEvidence    = errors.New("node declares no prompt, artifact or check")
	ErrArtifactPath  = errors.New("artifact must stay inside the artifacts dir")
	ErrTimeout       = errors.New("timeout_seconds must not be negative")
	ErrArtifactRef   = errors.New("reference names a node with no artifact")
	ErrUnknownRef    = errors.New("reference names an unknown node")

	// Input failures, wrapped with the offending input's name.
	ErrInputName      = errors.New("input must stay inside the inputs dir")
	ErrDuplicateInput = errors.New("duplicate input name")
	ErrUnknownInput   = errors.New("reference names an undeclared input")
)

// Graph is a declared unit of work: nodes, the files the run is given, and the
// evidence each node owes.
type Graph struct {
	Name     string   `yaml:"name"`
	Defaults Defaults `yaml:"defaults"`
	Inputs   []string `yaml:"inputs"`
	Nodes    []Node   `yaml:"nodes"`
}

// Defaults carries what every node inherits unless it overrides it.
type Defaults struct {
	Agent          string `yaml:"agent"`
	Model          string `yaml:"model"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

// Node is one step. A prompt makes an executor run; an artifact and a check
// are the evidence the node owes before its dependents may start.
type Node struct {
	Name           string   `yaml:"name"`
	Needs          []string `yaml:"needs"`
	Prompt         string   `yaml:"prompt"`
	Artifact       string   `yaml:"artifact"`
	Check          []string `yaml:"check"`
	Agent          string   `yaml:"agent"`
	Model          string   `yaml:"model"`
	TimeoutSeconds int      `yaml:"timeout_seconds"`
}

// placeholder matches {{artifact}}, {{artifact:<node>}} and {{input:<name>}}.
var placeholder = regexp.MustCompile(`\{\{(artifact|input)(?::([^}]*))?\}\}`)

// cycle marks a node's state during depth-first cycle detection.
type cycle int

const (
	visiting cycle = iota + 1 // the zero value means unvisited
	visited
)

// Load reads a graph from path, rejecting unknown fields, and validates it.
// The returned graph carries the defaults resolved onto every node.
func Load(path string) (*Graph, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open graph: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	var g Graph
	if err := decoder.Decode(&g); err != nil {
		return nil, fmt.Errorf("decode graph %s: %w", path, err)
	}

	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("graph %s: %w", path, err)
	}

	g.resolveDefaults()

	return &g, nil
}

// Node returns the named node, or nil when the graph does not declare it.
func (g *Graph) Node(name string) *Node {
	for i := range g.Nodes {
		if g.Nodes[i].Name == name {
			return &g.Nodes[i]
		}
	}

	return nil
}

// Expand replaces {{artifact}} with n's own artifact path,
// {{artifact:<node>}} with that node's - both absolute under artifactsDir -
// and {{input:<name>}} with that input's path under inputsDir. A reference
// Validate would have rejected is left as written.
func (g *Graph) Expand(n *Node, s, artifactsDir, inputsDir string) string {
	return placeholder.ReplaceAllStringFunc(s, func(match string) string {
		return g.expandOne(n, placeholder.FindStringSubmatch(match),
			artifactsDir, inputsDir)
	})
}

// expandOne resolves one matched placeholder to its path.
func (g *Graph) expandOne(
	n *Node, groups []string, artifactsDir, inputsDir string,
) string {
	name := groups[refNameGroup]

	if groups[kindGroup] == kindInput {
		if !g.hasInput(name) {
			return groups[wholeMatch]
		}

		return filepath.Join(inputsDir, name)
	}

	target, err := g.reference(n, name)
	if err != nil || target.Artifact == unset {
		return groups[wholeMatch]
	}

	return filepath.Join(artifactsDir, target.Artifact)
}

// ExpandAll expands every element of argv from n's point of view.
func (g *Graph) ExpandAll(
	n *Node, argv []string, artifactsDir, inputsDir string,
) []string {
	expanded := make([]string, len(argv))
	for i, arg := range argv {
		expanded[i] = g.Expand(n, arg, artifactsDir, inputsDir)
	}

	return expanded
}

// hasInput reports whether the graph declares the named input.
func (g *Graph) hasInput(name string) bool {
	return slices.Contains(g.Inputs, name)
}

// Validate reports the first way in which the graph is not runnable.
func (g *Graph) Validate() error {
	if err := g.validateHeader(); err != nil {
		return err
	}

	if err := g.validateNames(); err != nil {
		return err
	}

	if err := g.validateInputs(); err != nil {
		return err
	}

	for i := range g.Nodes {
		if err := g.validateNode(&g.Nodes[i]); err != nil {
			return err
		}
	}

	return g.detectCycle()
}

func (g *Graph) validateHeader() error {
	switch {
	case strings.TrimSpace(g.Name) == unset:
		return ErrGraphName
	case g.Defaults.Agent == unset:
		return ErrDefaultAgent
	case g.Defaults.Model == unset:
		return ErrDefaultModel
	case g.Defaults.TimeoutSeconds < minTimeout:
		return fmt.Errorf("defaults: %w", ErrTimeout)
	case len(g.Nodes) == emptyLen:
		return ErrNoNodes
	}

	return nil
}

func (g *Graph) validateNames() error {
	seen := make(map[string]bool, len(g.Nodes))

	for i := range g.Nodes {
		name := g.Nodes[i].Name

		if name == unset {
			return ErrNodeName
		}

		if seen[name] {
			return fmt.Errorf(nodeErrFmt, name, ErrDuplicateNode)
		}

		seen[name] = true
	}

	return nil
}

// validateInputs holds every declared input to the artifact path rule and to
// a single declaration, so an input names exactly one file in the run.
func (g *Graph) validateInputs() error {
	seen := make(map[string]bool, len(g.Inputs))

	for _, name := range g.Inputs {
		if !insideRunDir(name) {
			return fmt.Errorf(inputErrFmt, name, ErrInputName)
		}

		if seen[name] {
			return fmt.Errorf(inputErrFmt, name, ErrDuplicateInput)
		}

		seen[name] = true
	}

	return nil
}

func (g *Graph) validateNode(n *Node) error {
	switch {
	case n.Prompt == unset && n.Artifact == unset &&
		len(n.Check) == emptyLen:
		return fmt.Errorf(nodeErrFmt, n.Name, ErrNoEvidence)
	case n.TimeoutSeconds < minTimeout:
		return fmt.Errorf(nodeErrFmt, n.Name, ErrTimeout)
	case n.Artifact != unset && !insideRunDir(n.Artifact):
		return fmt.Errorf("node %q artifact %q: %w",
			n.Name, n.Artifact, ErrArtifactPath)
	}

	for _, need := range n.Needs {
		if g.Node(need) == nil {
			return fmt.Errorf("node %q needs %q: %w",
				n.Name, need, ErrUnknownNeed)
		}
	}

	return g.validateRefs(n)
}

// validateRefs holds every placeholder in the node's prompt and check to a
// node that actually declares an artifact, or to a declared input.
func (g *Graph) validateRefs(n *Node) error {
	if err := g.validateRefsIn(n, n.Prompt); err != nil {
		return err
	}

	for _, arg := range n.Check {
		if err := g.validateRefsIn(n, arg); err != nil {
			return err
		}
	}

	return nil
}

func (g *Graph) validateRefsIn(n *Node, s string) error {
	for _, groups := range placeholder.FindAllStringSubmatch(s, -1) {
		if err := g.validateRef(n, groups); err != nil {
			return err
		}
	}

	return nil
}

// validateRef holds one matched placeholder to what the graph declares.
func (g *Graph) validateRef(n *Node, groups []string) error {
	name := groups[refNameGroup]

	if groups[kindGroup] == kindInput {
		if !g.hasInput(name) {
			return fmt.Errorf("node %q references input %q: %w",
				n.Name, name, ErrUnknownInput)
		}

		return nil
	}

	target, err := g.reference(n, name)
	if err != nil {
		return err
	}

	if target.Artifact == unset {
		return fmt.Errorf("node %q references %q: %w",
			n.Name, target.Name, ErrArtifactRef)
	}

	return nil
}

// reference resolves one artifact placeholder to the node whose artifact it
// names. An empty name is the node's own artifact.
func (g *Graph) reference(n *Node, name string) (*Node, error) {
	if name == unset {
		return n, nil
	}

	target := g.Node(name)
	if target == nil {
		return nil, fmt.Errorf("node %q references %q: %w",
			n.Name, name, ErrUnknownRef)
	}

	return target, nil
}

func (g *Graph) detectCycle() error {
	state := make(map[string]cycle, len(g.Nodes))

	for i := range g.Nodes {
		if err := g.visit(&g.Nodes[i], state); err != nil {
			return err
		}
	}

	return nil
}

func (g *Graph) visit(n *Node, state map[string]cycle) error {
	switch state[n.Name] {
	case visiting:
		return fmt.Errorf(nodeErrFmt, n.Name, ErrCycle)
	case visited:
		return nil
	}

	state[n.Name] = visiting

	for _, need := range n.Needs {
		if err := g.visit(g.Node(need), state); err != nil {
			return err
		}
	}

	state[n.Name] = visited

	return nil
}

// resolveDefaults copies the graph defaults onto the nodes that did not
// override them, so the engine never consults two places for one setting.
func (g *Graph) resolveDefaults() {
	if g.Defaults.TimeoutSeconds == emptyLen {
		g.Defaults.TimeoutSeconds = DefaultTimeoutSeconds
	}

	for i := range g.Nodes {
		g.resolveNode(&g.Nodes[i])
	}
}

// resolveNode copies onto one node whatever it did not override itself.
func (g *Graph) resolveNode(n *Node) {
	if n.Agent == unset {
		n.Agent = g.Defaults.Agent
	}

	if n.Model == unset {
		n.Model = g.Defaults.Model
	}

	if n.TimeoutSeconds == emptyLen {
		n.TimeoutSeconds = g.Defaults.TimeoutSeconds
	}
}

// insideRunDir reports whether p stays under the run directory it is stated
// relative to. Artifact and input paths share the rule.
func insideRunDir(p string) bool {
	if filepath.IsAbs(p) {
		return false
	}

	clean := filepath.Clean(p)
	escape := ".." + string(filepath.Separator)

	return clean != "." && clean != ".." &&
		!strings.HasPrefix(clean, escape)
}
