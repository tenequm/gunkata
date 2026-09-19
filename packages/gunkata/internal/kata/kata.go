// Package kata loads and validates gunkata katas, the workflow files
// docs/spec.md defines. The kata file's shapes are its whole API, and none of
// them is inlineable, so the public-struct count is over revive's default.
//
//nolint:revive // max-public-structs: six shapes, one per thing a kata declares
package kata

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// DefaultTimeoutSeconds bounds an executor whose profile declares no timeout.
const DefaultTimeoutSeconds = 240

// MessageOutput is the output the engine writes an agent job's final message
// to. A job declares it to require a non-empty one.
const MessageOutput = "message.md"

// ParkedNote is the file the engine writes into a parked fan-out instance's
// artifact directory, saying which item it was given and why it did not come
// back. A template may not declare an output of that name.
const ParkedNote = "parked.md"

// The lint config forbids bare literals, so the ones this package repeats are
// named here.
const (
	unset      = ""
	emptyLen   = 0
	minTimeout = 0
	kindGroup  = 1
	refGroup   = 2
	jobErrFmt  = "job %q: %w"
	refSep     = "/"
	kindParam  = "param"
	kindOutput = "output"
	kindArtif  = "artifact"
	kindFanOut = "fanout"
	kindItem   = "item"
	// minRound is the smallest legal fan-out cap: a head that runs once.
	minRound   = 1
	keyProfile = "profile"
	keyHarness = "harness"
	keyURL     = "url"
	singleQ    = '\''
	doubleQ    = '"'
	noQuote    = rune(0)
	quotedFmt  = "%w: %q"
	// A YAML mapping node's content alternates key and value.
	pairStride = 2
	// blankLine joins a job's appended system prompt text to its profile's.
	blankLine = "\n\n"
)

// Validation failures, wrapped with whatever they concern.
var (
	ErrName         = errors.New("kata declares no name")
	ErrNoJobs       = errors.New("workflow declares no jobs")
	ErrParamName    = errors.New("param name must be letters, digits, - or _")
	ErrEmptyJob     = errors.New("job is empty")
	ErrHarness      = errors.New("harness is required")
	ErrModel        = errors.New("model is required")
	ErrTimeout      = errors.New("timeout_seconds must not be negative")
	ErrUnknownAgent = errors.New("agent names an unknown profile")
	ErrAgentPrompt  = errors.New("agent is required iff prompt is present")
	ErrNoEvidence   = errors.New("job declares no output or post-step")
	ErrOutputName   = errors.New("output must be a single path element")
	ErrDupOutput    = errors.New("duplicate output")
	ErrMessageJob   = errors.New(MessageOutput + " needs a prompt")
	ErrUnknownNeed  = errors.New("needs names an unknown job")
	ErrCycle        = errors.New("workflow contains a cycle")
	ErrPlaceholder  = errors.New("unknown placeholder kind")
	ErrUnknownParam = errors.New("placeholder names an undeclared param")
	ErrUnknownOut   = errors.New("placeholder names an undeclared output")
	ErrArtifactRef  = errors.New("artifact placeholder wants <job>/<output>")
	ErrStep         = errors.New("malformed step")
	ErrShell        = errors.New("shell syntax in a step")
	ErrAmendField   = errors.New("agent amendment names an unknown field")
	ErrAmendHarness = errors.New("harness may not be amended")
	ErrMCPField     = errors.New("MCP entry names an unknown field")
	ErrMCPURL       = errors.New("MCP entry declares no url")
	ErrACPAdapter   = errors.New("acp_adapter must be one npm package spec")
	ErrJobName      = errors.New("job name must be letters, digits, - or _")
	ErrFanItems     = errors.New("fan-out items must name an output of the job")
	ErrFanJob       = errors.New("fan-out job names an unknown job")
	ErrFanSelf      = errors.New("fan-out job may not be the job itself")
	ErrFanShared    = errors.New("job is the template of more than one fan-out")
	ErrFanNested    = errors.New("a fan-out template may not fan out itself")
	ErrFanNeeds     = errors.New("a fan-out template may not declare needs")
	ErrFanNeeded    = errors.New("needs names a fan-out template")
	ErrFanRounds    = errors.New("fan-out max_rounds must be at least 1")
	ErrFanMaxItems  = errors.New("fan-out max_items must be at least 1")
	ErrFanOutRef    = errors.New(
		"fanout placeholder names no fan-out head or template")
	ErrFanArtifact = errors.New(
		"a fan-out head or template has no single artifact directory")
	ErrItemRef     = errors.New("{{item}} is only for a fan-out template")
	ErrParkedOwned = errors.New(
		ParkedNote + " is the engine's; a template may not declare it")
)

// shellTokens are load errors in a step's string form, which is never a
// shell.
var shellTokens = []string{"|", ">", "<", "&&", ";", "$", "`"}

// paramName keeps a param key safe as a run-dir path element. A job name is
// held to the same shape, so the engine's own instance keys - which carry
// path separators - can never be spelled by a kata.
var paramName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// acpAdapter is one npm package spec, optionally versioned: nothing that
// would split into two arguments or read as npx flags.
var acpAdapter = regexp.MustCompile(
	`^(@[a-z0-9][a-z0-9._-]*/)?[a-z0-9][a-z0-9._-]*(@[A-Za-z0-9._^~<>=|*-]+)?$`)

// placeholder matches any {{kind:ref}}, or a bare {{kind}} for the ref-less
// {{item}}; the kind is checked at load.
var placeholder = regexp.MustCompile(`\{\{([a-z]+)(?::([^}]*))?\}\}`)

// Kata is a declared choreography: params, executor profiles, and the DAG.
type Kata struct {
	Name     string             `yaml:"name"`
	Params   map[string]Param   `yaml:"params"`
	Agents   map[string]Profile `yaml:"agents"`
	Workflow map[string]*Job    `yaml:"workflow"`
	// Dir is the directory the kata file sits in; local skill paths resolve
	// against it.
	Dir string `yaml:"-"`
}

// Param is one invocation-specific value. A nil Default makes it required.
type Param struct {
	Default *string `yaml:"default"`
}

// Profile fully describes one executor shape.
type Profile struct {
	Harness        string            `yaml:"harness"`
	Model          string            `yaml:"model"`
	TimeoutSeconds int               `yaml:"timeout_seconds"`
	Options        map[string]string `yaml:"options"`
	Skills         []string          `yaml:"skills"`
	MCPs           []mcpEntry        `yaml:"mcps"`
	// ACPAdapter pins the npm package acpx runs as the harness's adapter;
	// unset keeps acpx's built-in one.
	ACPAdapter string `yaml:"acp_adapter"`
	// AppendSystemPrompt is text appended to the agent's system prompt.
	AppendSystemPrompt string `yaml:"append_system_prompt"`
}

// mcpEntry is one MCP server. An optional server whose URL references an
// unset variable is skipped; a required one refuses the run.
type mcpEntry struct {
	URL      string `yaml:"url"`
	Required bool   `yaml:"required"`
}

// Job is setup, judgment, evidence.
type Job struct {
	Needs     []string  `yaml:"needs"`
	PreSteps  []Step    `yaml:"pre-steps"`
	Agent     *agentRef `yaml:"agent"`
	Prompt    string    `yaml:"prompt"`
	Outputs   []string  `yaml:"outputs"`
	PostSteps []Step    `yaml:"post-steps"`
	// FanOut makes the job a round head: the engine reads its items output
	// after each round and runs the template job once per item.
	FanOut *FanOut `yaml:"fan-out"`
	// Name is the job's key in the workflow, or - for an instance the engine
	// derives at runtime - that key plus its round and item.
	Name string `yaml:"-"`
	// Template is set on the job a fan-out names: it is never scheduled on
	// its own.
	Template bool `yaml:"-"`
	// Item is the absolute path of the work item an instance was given;
	// {{item}} expands to it.
	Item string `yaml:"-"`
	// Executor is the job's profile with its amendments merged, resolved at
	// load; nil for a deterministic job.
	Executor *Profile `yaml:"-"`
}

// FanOut declares runtime fan-out: after each round the engine reads the
// entries of the head's items directory and runs Job once per entry. A round
// that yields no entry ends the loop, and MaxRounds bounds it either way.
type FanOut struct {
	Items     string `yaml:"items"`
	Job       string `yaml:"job"`
	MaxRounds int    `yaml:"max_rounds"`
	MaxItems  int    `yaml:"max_items"`
}

// ItemsOutput is the head's items output, or unset on any other job. An
// empty items directory is the loop's terminating answer, so it is the one
// output exempt from the non-empty check.
func (job *Job) ItemsOutput() string {
	if job.FanOut == nil {
		return unset
	}

	return job.FanOut.Items
}

// agentRef names a profile, optionally amending it.
type agentRef struct {
	Profile string
	Amend   Profile
}

// Step is one argv, never a shell.
type Step []string

// amendFields are the keys an agent amendment may carry.
var amendFields = []string{
	keyProfile, "model", "timeout_seconds", "options", "skills", "mcps",
	"acp_adapter", "append_system_prompt",
}

// UnmarshalYAML takes a profile name or an amendment object.
func (a *agentRef) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		a.Profile = node.Value

		return nil
	}

	for pair := range len(node.Content) / pairStride {
		key := node.Content[pair*pairStride].Value
		if key == keyHarness {
			return ErrAmendHarness
		}

		if !slices.Contains(amendFields, key) {
			return fmt.Errorf(quotedFmt, ErrAmendField, key)
		}
	}

	var doc struct {
		Profile string  `yaml:"profile"`
		Amend   Profile `yaml:",inline"`
	}

	if err := node.Decode(&doc); err != nil {
		return fmt.Errorf("decode agent: %w", err)
	}

	a.Profile, a.Amend = doc.Profile, doc.Amend

	return nil
}

// mcpFields are the keys an MCP entry's object form may carry.
var mcpFields = []string{keyURL, "required"}

// UnmarshalYAML takes a URL or a {url, required} object.
func (m *mcpEntry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		m.URL = node.Value
	} else if err := decodeMCP(node, m); err != nil {
		return err
	}

	if m.URL == unset {
		return ErrMCPURL
	}

	return nil
}

func decodeMCP(node *yaml.Node, m *mcpEntry) error {
	for pair := range len(node.Content) / pairStride {
		key := node.Content[pair*pairStride].Value
		if !slices.Contains(mcpFields, key) {
			return fmt.Errorf(quotedFmt, ErrMCPField, key)
		}
	}

	type plain mcpEntry // drops UnmarshalYAML, so Decode does not recurse
	if err := node.Decode((*plain)(m)); err != nil {
		return fmt.Errorf("decode MCP entry: %w", err)
	}

	return nil
}

// UnmarshalYAML takes an argv array, or a string split into one.
func (s *Step) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.SequenceNode {
		var argv []string
		if err := node.Decode(&argv); err != nil {
			return fmt.Errorf("%w: %w", ErrStep, err)
		}

		*s = argv
	} else {
		argv, err := Split(node.Value)
		if err != nil {
			return err
		}

		*s = argv
	}

	if len(*s) == emptyLen {
		return fmt.Errorf("%w: empty argv", ErrStep)
	}

	return nil
}

// Split parses a step's string form into argv: spaces separate, quotes group.
// Shell syntax anywhere is an error.
func Split(line string) ([]string, error) {
	for _, token := range shellTokens {
		if strings.Contains(line, token) {
			return nil, fmt.Errorf("%w: %q in %q", ErrShell, token, line)
		}
	}

	sp := splitter{quote: noQuote}
	for _, r := range line {
		sp.take(r)
	}

	if sp.quote != noQuote {
		return nil, fmt.Errorf("%w: unterminated quote in %q", ErrStep, line)
	}

	sp.endWord()

	return sp.argv, nil
}

// splitter is Split's state: the words so far, the one in progress, and the
// quote it is inside, if any.
type splitter struct {
	argv   []string
	word   strings.Builder
	inWord bool
	quote  rune
}

func (sp *splitter) take(r rune) {
	switch {
	case sp.quote != noQuote && r == sp.quote:
		sp.quote = noQuote
	case sp.quote != noQuote:
		sp.word.WriteRune(r)
	case r == singleQ || r == doubleQ:
		sp.quote, sp.inWord = r, true
	case unicode.IsSpace(r):
		sp.endWord()
	default:
		sp.word.WriteRune(r)
		sp.inWord = true
	}
}

func (sp *splitter) endWord() {
	if sp.inWord {
		sp.argv = append(sp.argv, sp.word.String())
		sp.word.Reset()
		sp.inWord = false
	}
}

// Load reads a kata, rejecting unknown fields, validates it, and resolves
// every job's executor profile.
func Load(path string) (*Kata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open kata: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	var k Kata
	if err := decoder.Decode(&k); err != nil {
		return nil, fmt.Errorf("decode kata %s: %w", path, err)
	}

	abs, absErr := filepath.Abs(path)
	if absErr != nil {
		return nil, fmt.Errorf("resolve kata path: %w", absErr)
	}

	k.Dir = filepath.Dir(abs)

	for name, job := range k.Workflow {
		if job == nil {
			return nil, fmt.Errorf("kata %s: "+jobErrFmt,
				path, name, ErrEmptyJob)
		}

		job.Name = name
	}

	if err := k.validate(); err != nil {
		return nil, fmt.Errorf("kata %s: %w", path, err)
	}

	return &k, nil
}

// Jobs lists the workflow's jobs in name order, so every walk is stable.
func (k *Kata) Jobs() []*Job {
	jobs := make([]*Job, emptyLen, len(k.Workflow))
	for _, name := range slices.Sorted(maps.Keys(k.Workflow)) {
		jobs = append(jobs, k.Workflow[name])
	}

	return jobs
}

func (k *Kata) validate() error {
	switch {
	case strings.TrimSpace(k.Name) == unset:
		return ErrName
	case len(k.Workflow) == emptyLen:
		return ErrNoJobs
	}

	if err := k.validateDecls(); err != nil {
		return err
	}

	if err := k.resolveFanOut(); err != nil {
		return err
	}

	for _, job := range k.Jobs() {
		if err := k.validateJob(job); err != nil {
			return fmt.Errorf(jobErrFmt, job.Name, err)
		}
	}

	return k.detectCycle()
}

// validateDecls holds the params and profiles to what they may declare.
// resolveFanOut marks every job a fan-out names as a template and holds each
// declaration to what the engine can schedule. It runs before the per-job
// pass, which needs the marks to resolve {{fanout:}} and {{item}}.
func (k *Kata) resolveFanOut() error {
	for _, job := range k.Jobs() {
		if job.FanOut == nil {
			continue
		}

		tmpl, err := k.fanOutTemplate(job)
		if err != nil {
			return fmt.Errorf(jobErrFmt, job.Name, err)
		}

		tmpl.Template = true
	}

	return k.validateTemplates()
}

// fanOutTemplate holds one fan-out declaration to a template the engine can
// instantiate, and returns it.
func (k *Kata) fanOutTemplate(job *Job) (*Job, error) {
	f := job.FanOut
	tmpl := k.Workflow[f.Job]

	switch {
	case !slices.Contains(job.Outputs, f.Items):
		return nil, fmt.Errorf(quotedFmt, ErrFanItems, f.Items)
	case f.MaxRounds < minRound:
		return nil, ErrFanRounds
	case f.MaxItems < minRound:
		return nil, ErrFanMaxItems
	case tmpl == nil:
		return nil, fmt.Errorf(quotedFmt, ErrFanJob, f.Job)
	case tmpl == job:
		return nil, ErrFanSelf
	case tmpl.Template:
		return nil, fmt.Errorf(quotedFmt, ErrFanShared, f.Job)
	}

	return tmpl, nil
}

// validateTemplates holds a template to a job the engine alone schedules:
// nothing needs it, it needs nothing, and it does not fan out in turn.
func (k *Kata) validateTemplates() error {
	for _, job := range k.Jobs() {
		for _, need := range job.Needs {
			if dep := k.Workflow[need]; dep != nil && dep.Template {
				return fmt.Errorf(jobErrFmt, job.Name,
					fmt.Errorf(quotedFmt, ErrFanNeeded, need))
			}
		}

		if !job.Template {
			continue
		}

		switch {
		case job.FanOut != nil:
			return fmt.Errorf(jobErrFmt, job.Name, ErrFanNested)
		case len(job.Needs) != emptyLen:
			return fmt.Errorf(jobErrFmt, job.Name, ErrFanNeeds)
		case slices.Contains(job.Outputs, ParkedNote):
			return fmt.Errorf(jobErrFmt, job.Name, ErrParkedOwned)
		}
	}

	return nil
}

func (k *Kata) validateDecls() error {
	for name := range k.Workflow {
		if !paramName.MatchString(name) {
			return fmt.Errorf(quotedFmt, ErrJobName, name)
		}
	}

	for name := range k.Params {
		if !paramName.MatchString(name) {
			return fmt.Errorf(quotedFmt, ErrParamName, name)
		}
	}

	for _, name := range slices.Sorted(maps.Keys(k.Agents)) {
		if err := validateProfile(k.Agents[name]); err != nil {
			return fmt.Errorf("profile %q: %w", name, err)
		}
	}

	return nil
}

func validateProfile(p Profile) error {
	switch {
	case p.Harness == unset:
		return ErrHarness
	case p.Model == unset:
		return ErrModel
	case p.TimeoutSeconds < minTimeout:
		return ErrTimeout
	}

	return validateAdapter(p.ACPAdapter)
}

func validateAdapter(adapter string) error {
	if adapter != unset && !acpAdapter.MatchString(adapter) {
		return fmt.Errorf(quotedFmt, ErrACPAdapter, adapter)
	}

	return nil
}

func (k *Kata) validateJob(job *Job) error {
	if err := validateShape(job); err != nil {
		return err
	}

	for _, need := range job.Needs {
		if k.Workflow[need] == nil {
			return fmt.Errorf(quotedFmt, ErrUnknownNeed, need)
		}
	}

	if err := k.resolveExecutor(job); err != nil {
		return err
	}

	return k.validateRefs(job)
}

// validateShape holds the job to the spec's structure: an agent exactly when
// there is a prompt, evidence to verify, and a final message only from an
// agent.
func validateShape(job *Job) error {
	if (job.Agent == nil) != (job.Prompt == unset) {
		return ErrAgentPrompt
	}

	if len(job.Outputs) == emptyLen && len(job.PostSteps) == emptyLen {
		return ErrNoEvidence
	}

	if job.Prompt == unset && slices.Contains(job.Outputs, MessageOutput) {
		return ErrMessageJob
	}

	return validateOutputs(job.Outputs)
}

func validateOutputs(outputs []string) error {
	seen := make(map[string]bool, len(outputs))

	for _, out := range outputs {
		if out == unset || out == "." || out == ".." ||
			strings.ContainsRune(out, filepath.Separator) {
			return fmt.Errorf(quotedFmt, ErrOutputName, out)
		}

		if seen[out] {
			return fmt.Errorf(quotedFmt, ErrDupOutput, out)
		}

		seen[out] = true
	}

	return nil
}

// resolveExecutor resolves the job's profile with its amendments merged.
func (k *Kata) resolveExecutor(job *Job) error {
	if job.Agent == nil {
		return nil
	}

	base, ok := k.Agents[job.Agent.Profile]
	if !ok {
		return fmt.Errorf(quotedFmt, ErrUnknownAgent, job.Agent.Profile)
	}

	if job.Agent.Amend.TimeoutSeconds < minTimeout {
		return ErrTimeout
	}

	if err := validateAdapter(job.Agent.Amend.ACPAdapter); err != nil {
		return err
	}

	merged := merge(base, job.Agent.Amend)
	job.Executor = &merged

	return nil
}

// merge applies an amendment to a profile: scalars replace, lists and the
// appended system prompt append, options keys win.
func merge(base, amend Profile) Profile {
	merged := Profile{
		Harness: base.Harness,
		Model:   cmp.Or(amend.Model, base.Model),
		TimeoutSeconds: cmp.Or(amend.TimeoutSeconds, base.TimeoutSeconds,
			DefaultTimeoutSeconds),
		Options:    map[string]string{},
		Skills:     slices.Concat(base.Skills, amend.Skills),
		MCPs:       slices.Concat(base.MCPs, amend.MCPs),
		ACPAdapter: cmp.Or(amend.ACPAdapter, base.ACPAdapter),
		AppendSystemPrompt: joinText(
			base.AppendSystemPrompt, amend.AppendSystemPrompt),
	}

	maps.Copy(merged.Options, base.Options)
	maps.Copy(merged.Options, amend.Options)

	return merged
}

// joinText appends amend to base as a paragraph of its own.
func joinText(base, amend string) string {
	if base == unset || amend == unset {
		return base + amend
	}

	return strings.TrimRight(base, "\n") + blankLine + amend
}

// texts is every string of the job placeholders may appear in.
func (job *Job) texts() []string {
	texts := []string{job.Prompt}
	if job.Executor != nil {
		texts = append(texts, job.Executor.AppendSystemPrompt)
	}
	for _, step := range slices.Concat(job.PreSteps, job.PostSteps) {
		texts = append(texts, step...)
	}

	return texts
}

// validateRefs holds every placeholder the job uses to what the kata declares.
func (k *Kata) validateRefs(job *Job) error {
	text := strings.Join(job.texts(), "\n")
	for _, groups := range placeholder.FindAllStringSubmatch(text, -1) {
		err := k.validateRef(job, groups[kindGroup], groups[refGroup])
		if err != nil {
			return err
		}
	}

	return nil
}

func (k *Kata) validateRef(job *Job, kind, ref string) error {
	switch kind {
	case kindParam:
		if _, ok := k.Params[ref]; !ok {
			return fmt.Errorf(quotedFmt, ErrUnknownParam, ref)
		}
	case kindOutput:
		if !slices.Contains(job.Outputs, ref) {
			return fmt.Errorf(quotedFmt, ErrUnknownOut, ref)
		}
	case kindArtif:
		return k.validateArtifactRef(ref)
	case kindFanOut:
		if target := k.Workflow[ref]; target == nil ||
			(target.FanOut == nil && !target.Template) {
			return fmt.Errorf(quotedFmt, ErrFanOutRef, ref)
		}
	case kindItem:
		if ref != unset || !job.Template {
			return ErrItemRef
		}
	default:
		return fmt.Errorf(quotedFmt, ErrPlaceholder, kind)
	}

	return nil
}

// validateArtifactRef holds <job>/<output> to a job that declares it.
func (k *Kata) validateArtifactRef(ref string) error {
	target, out, ok := strings.Cut(ref, refSep)
	if !ok || k.Workflow[target] == nil {
		return fmt.Errorf(quotedFmt, ErrArtifactRef, ref)
	}

	if k.Workflow[target].FanOut != nil || k.Workflow[target].Template {
		return fmt.Errorf(quotedFmt, ErrFanArtifact, ref)
	}

	if !slices.Contains(k.Workflow[target].Outputs, out) {
		return fmt.Errorf(quotedFmt, ErrUnknownOut, ref)
	}

	return nil
}

// visit marks a job's state during depth-first cycle detection.
type visit int

const (
	visiting visit = iota + 1 // the zero value means unvisited
	visited
)

func (k *Kata) detectCycle() error {
	state := make(map[string]visit, len(k.Workflow))

	for _, job := range k.Jobs() {
		if err := k.visit(job, state); err != nil {
			return err
		}
	}

	return nil
}

func (k *Kata) visit(job *Job, state map[string]visit) error {
	switch state[job.Name] {
	case visiting:
		return fmt.Errorf(jobErrFmt, job.Name, ErrCycle)
	case visited:
		return nil
	}

	state[job.Name] = visiting

	for _, need := range job.Needs {
		if err := k.visit(k.Workflow[need], state); err != nil {
			return err
		}
	}

	state[job.Name] = visited

	return nil
}

// Expand replaces every placeholder in s from job's point of view: params
// with their bound values, outputs, artifacts and fan-out trees with absolute
// paths under artifactsDir, and {{item}} with the instance's work item. It
// runs after validation, so every reference resolves.
func Expand(
	job *Job, s string, params map[string]string, artifactsDir string,
) string {
	return placeholder.ReplaceAllStringFunc(s, func(match string) string {
		groups := placeholder.FindStringSubmatch(match)
		ref := groups[refGroup]

		switch groups[kindGroup] {
		case kindParam:
			return params[ref]
		case kindOutput:
			return filepath.Join(artifactsDir, job.Name, ref)
		case kindItem:
			return job.Item
		default:
			return filepath.Join(artifactsDir, ref)
		}
	})
}

// ExpandAll expands every element of argv, so an expanded value with spaces
// stays one argument.
func ExpandAll(
	job *Job, argv []string, params map[string]string, artifactsDir string,
) []string {
	out := make([]string, len(argv))
	for i, arg := range argv {
		out[i] = Expand(job, arg, params, artifactsDir)
	}

	return out
}
