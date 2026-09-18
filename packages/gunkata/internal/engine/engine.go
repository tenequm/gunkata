// Package engine runs a gunkata graph. It lays out the run directory, starts
// one generic executor per node that carries a prompt, releases a node's
// dependents only against verified evidence, and records what happened.
package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tenequm/gunkata/internal/graph"
)

// The lint config forbids bare literals, so the ones this package repeats are
// named here.
const (
	unset      = ""
	emptyLen   = 0
	nodeLogFmt = "%s: %v"
	argvHead   = 0 // the command in a check's argv
	argvTail   = 1 // its arguments
	// attemptOnce is every attempt count this engine can record: a failure
	// parks the node, because nothing classifies it yet (lock 4).
	attemptOnce = 1
)

// Options configures one run.
type Options struct {
	GraphPath string
	RunsRoot  string
	// Inputs binds each input the graph declares to the file the run copies
	// in. A graph that declares none takes none.
	Inputs map[string]string
	// Progress receives one human-readable line per node transition. It is
	// never the run's result; that is the record.
	Progress io.Writer
}

// Result is where the run landed and what it concluded.
type Result struct {
	RunDir  string
	Outcome Outcome
}

// Run schedules the graph at opts.GraphPath and writes its record. A parked
// run is a Result, not an error; an error means the run could not be carried
// out at all. Result.RunDir is set as soon as the directory exists, so a
// caller can point at it even when Run fails.
func Run(ctx context.Context, opts Options) (Result, error) {
	g, err := graph.Load(opts.GraphPath)
	if err != nil {
		return Result{}, fmt.Errorf("load graph: %w", err)
	}

	l, err := newLayout(opts.RunsRoot)
	if err != nil {
		return Result{}, err
	}

	res := Result{RunDir: l.dir}

	if err := l.copyGraph(opts.GraphPath); err != nil {
		return res, err
	}

	inputs, bindErr := l.bindInputs(g.Inputs, opts.Inputs)
	if bindErr != nil {
		return res, fmt.Errorf("bind inputs: %w", bindErr)
	}

	s := newScheduler(g, l, opts.Progress)
	started := time.Now()

	s.execute(ctx)

	rec := s.record(started, time.Now())
	rec.Inputs = inputs

	if err := l.writeRecord(rec); err != nil {
		return res, err
	}

	res.Outcome = rec.Outcome

	if ctx.Err() != nil {
		return res, fmt.Errorf("run interrupted: %w", ctx.Err())
	}

	return res, nil
}

// scheduler holds the state of one run in flight.
type scheduler struct {
	graph    *graph.Graph
	layout   *layout
	progress io.Writer
	mu       sync.Mutex // serialises progress writes
	// nodes holds one record per node. Each is written only by that node's
	// own goroutine, and read by its dependents once it has settled.
	nodes map[string]*nodeRecord
	// settled is closed per node once its record is final.
	settled map[string]chan struct{}
}

func newScheduler(g *graph.Graph, l *layout, progress io.Writer) *scheduler {
	s := &scheduler{
		graph:    g,
		layout:   l,
		progress: progress,
		nodes:    make(map[string]*nodeRecord, len(g.Nodes)),
		settled:  make(map[string]chan struct{}, len(g.Nodes)),
	}

	for i := range g.Nodes {
		n := &g.Nodes[i]
		s.nodes[n.Name] = &nodeRecord{
			State:    statePending,
			Artifact: artifactRef(n),
		}
		s.settled[n.Name] = make(chan struct{})
	}

	return s
}

// execute runs every node whose needs are met, as soon as they are met, and
// returns once no node can make progress. It does not return while a process
// group is still alive, so a cancelled run is torn down before Run returns.
func (s *scheduler) execute(ctx context.Context) {
	var wg sync.WaitGroup

	for i := range s.graph.Nodes {
		n := &s.graph.Nodes[i]

		wg.Go(func() {
			defer close(s.settled[n.Name])

			s.runNode(ctx, n)
		})
	}

	wg.Wait()
}

// runNode waits for the node's needs and, if they were verified, attempts it
// exactly once.
func (s *scheduler) runNode(ctx context.Context, n *graph.Node) {
	if !s.needsDone(ctx, n) {
		return // stays pending; nothing of it runs and nothing is created
	}

	rec := s.nodes[n.Name]
	rec.Attempts = attemptOnce
	rec.StartedAt = stampPtr(time.Now())

	s.logf("%s: start", n.Name)

	rec.State = s.evaluate(ctx, n, rec)
	rec.FinishedAt = stampPtr(time.Now())

	s.logf("%s: %s", n.Name, rec.State)
}

// needsDone reports whether every node this one needs reached done. A need
// that parked, or a cancelled run, leaves the node pending.
func (s *scheduler) needsDone(ctx context.Context, n *graph.Node) bool {
	for _, need := range n.Needs {
		select {
		case <-s.settled[need]:
		case <-ctx.Done():
			return false
		}

		if s.nodes[need].State != stateDone {
			s.logf("%s: pending, %s did not complete", n.Name, need)

			return false
		}
	}

	return ctx.Err() == nil
}

// evaluate walks the evidence the node declared, in the order the contract
// states, and stops at the first piece that does not pass. Nothing an
// executor reported about itself is consulted.
func (s *scheduler) evaluate(
	ctx context.Context, n *graph.Node, rec *nodeRecord,
) nodeState {
	if n.Prompt != unset && !s.runPrompt(ctx, n, rec) {
		return stateParked
	}

	if n.Artifact != unset && !s.artifactPresent(n) {
		return stateParked
	}

	if len(n.Check) != emptyLen && !s.checkPasses(ctx, n, rec) {
		return stateParked
	}

	return stateDone
}

func (s *scheduler) runPrompt(
	ctx context.Context, n *graph.Node, rec *nodeRecord,
) bool {
	if err := s.layout.ensureNode(n.Name); err != nil {
		s.logf(nodeLogFmt, n.Name, err)

		return false
	}

	artifacts, inputs := s.layout.artifacts(), s.layout.inputs()

	code, err := runExecutor(ctx, execSpec{
		agent:         n.Agent,
		model:         n.Model,
		prompt:        s.graph.Expand(n, n.Prompt, artifacts, inputs),
		configOptions: n.ConfigOptions,
		timeout:       time.Duration(n.TimeoutSeconds) * time.Second,
		home:          s.layout.home(n.Name),
		work:          s.layout.work(n.Name),
		logPath:       s.layout.logPath(n.Name),
	})
	if err != nil {
		s.logf(nodeLogFmt, n.Name, err)

		return false
	}

	rec.ExecutorExit = &code

	return code == exitOK
}

// artifactPresent holds the artifact to what the engine can see on disk: a
// regular file with something in it.
func (s *scheduler) artifactPresent(n *graph.Node) bool {
	path := filepath.Join(s.layout.artifacts(), n.Artifact)

	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == emptyLen {
		s.logf("%s: artifact %s is not evidence", n.Name, n.Artifact)

		return false
	}

	return true
}

func (s *scheduler) checkPasses(
	ctx context.Context, n *graph.Node, rec *nodeRecord,
) bool {
	argv := s.graph.ExpandAll(n, n.Check,
		s.layout.artifacts(), s.layout.inputs())

	code, err := runCheck(ctx, argv, s.layout.dir)
	if err != nil {
		s.logf(nodeLogFmt, n.Name, err)

		return false
	}

	rec.GateExit = &code

	return code == exitOK
}

// record assembles the run's account of itself.
func (s *scheduler) record(started, finished time.Time) *record {
	rec := &record{
		RunID:      s.layout.id,
		Graph:      s.graph.Name,
		Outcome:    OutcomeSucceeded,
		StartedAt:  stamp(started),
		FinishedAt: stamp(finished),
		Nodes:      s.nodes,
	}

	for _, n := range s.nodes {
		if n.State != stateDone {
			rec.Outcome = OutcomeParked
		}
	}

	return rec
}

func (s *scheduler) logf(format string, args ...any) {
	if s.progress == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	fmt.Fprintf(s.progress, format+"\n", args...)
}

// artifactRef states the node's declared artifact relative to the run dir, so
// a reader of the record can find it without the graph.
func artifactRef(n *graph.Node) *string {
	if n.Artifact == unset {
		return nil
	}

	ref := filepath.Join(artifactsDir, n.Artifact)

	return &ref
}
