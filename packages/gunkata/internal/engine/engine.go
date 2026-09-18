// Package engine runs a gunkata kata. It lays out the run directory, runs each
// job's steps and executor, releases a job's dependents only against verified
// evidence, and records what happened.
package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tenequm/gunkata/internal/kata"
)

// The lint config forbids bare literals, so the ones this package repeats are
// named here.
const (
	unset    = ""
	emptyLen = 0
	argvHead = 0 // the command in a step's argv
	argvTail = 1 // its arguments
	stepNum  = 1 // steps are numbered from one in failures
)

var (
	errUnknownParam = errors.New("no such param is declared by the kata")
	errUnboundParam = errors.New("required param is not bound")
)

// Options configures one run.
type Options struct {
	KataPath string
	RunsRoot string
	// Params binds declared params by key. Unbound params take their
	// default; a required param must be bound.
	Params map[string]string
	// Progress receives one human-readable line per job transition. It is
	// never the run's result; that is the record.
	Progress io.Writer
}

// Result is where the run landed and what it concluded.
type Result struct {
	RunDir  string
	Outcome Outcome
}

// Run executes the kata at opts.KataPath and writes its record. A parked run
// is a Result, not an error; an error means the run could not be carried out
// at all. Result.RunDir is set as soon as the directory exists.
func Run(ctx context.Context, opts Options) (Result, error) {
	k, err := kata.Load(opts.KataPath)
	if err != nil {
		return Result{}, fmt.Errorf("load kata: %w", err)
	}

	bound, err := bindParams(k, opts.Params)
	if err != nil {
		return Result{}, err
	}

	if checkErr := preflight(k); checkErr != nil {
		return Result{}, checkErr
	}

	l, err := newLayout(opts.RunsRoot)
	if err != nil {
		return Result{}, err
	}

	res := Result{RunDir: l.dir}

	s, err := setup(ctx, k, l, bound, opts)
	if err != nil {
		return res, err
	}

	started := time.Now()

	s.execute(ctx)

	rec := s.record(started, time.Now())
	rec.Params = bound

	if err := l.writeRecord(rec); err != nil {
		return res, err
	}

	res.Outcome = rec.Outcome

	if ctx.Err() != nil {
		return res, fmt.Errorf("run interrupted: %w", ctx.Err())
	}

	return res, nil
}

// bindParams holds the bindings to the declared params: none undeclared,
// every required one bound, defaults filling the rest.
func bindParams(k *kata.Kata, given map[string]string) (
	map[string]string, error,
) {
	for key := range given {
		if _, ok := k.Params[key]; !ok {
			return nil, fmt.Errorf(quotedFmt, errUnknownParam, key)
		}
	}

	bound := make(map[string]string, len(k.Params))

	for key, param := range k.Params {
		value, ok := given[key]

		switch {
		case ok:
			bound[key] = value
		case param.Default != nil:
			bound[key] = *param.Default
		default:
			return nil, fmt.Errorf(quotedFmt, errUnboundParam, key)
		}
	}

	return bound, nil
}

// preflight rejects what would fail mid-run: skills a harness cannot load,
// MCP variables the environment lacks.
func preflight(k *kata.Kata) error {
	if err := checkSkills(k); err != nil {
		return err
	}

	for _, job := range k.Jobs() {
		if job.Executor == nil {
			continue
		}

		if _, err := mcpConfigJSON(job.Executor.MCPs); err != nil {
			return fmt.Errorf("job %q: %w", job.Name, err)
		}
	}

	return nil
}

// setup fills the run dir with what the run is given - the kata, param
// snapshots, skill snapshots - before any job starts.
func setup(
	ctx context.Context, k *kata.Kata, l *layout, bound map[string]string,
	opts Options,
) (*scheduler, error) {
	if err := l.copyKata(opts.KataPath); err != nil {
		return nil, err
	}

	params, err := l.snapshotParams(bound)
	if err != nil {
		return nil, err
	}

	skills, err := fetchSkills(ctx, k, l.skills())
	if err != nil {
		return nil, err
	}

	s := newScheduler(k, l, params, skills.snapshots, opts.Progress)
	s.skillSources = skills.sources

	return s, nil
}

// scheduler holds the state of one run in flight.
type scheduler struct {
	kata     *kata.Kata
	layout   *layout
	params   map[string]string // placeholder values
	skills   map[string]string // skill entry -> snapshot dir
	progress io.Writer
	mu       sync.Mutex // serialises progress writes
	// jobs holds one record per job. Each is written only by that job's own
	// goroutine, and read by its dependents once it has settled.
	jobs map[string]*jobRecord
	// settled is closed per job once its record is final.
	settled      map[string]chan struct{}
	skillSources map[string]string
}

func newScheduler(
	k *kata.Kata, l *layout, params, skills map[string]string,
	progress io.Writer,
) *scheduler {
	s := &scheduler{
		kata:     k,
		layout:   l,
		params:   params,
		skills:   skills,
		progress: progress,
		jobs:     make(map[string]*jobRecord, len(k.Workflow)),
		settled:  make(map[string]chan struct{}, len(k.Workflow)),
	}

	for name := range k.Workflow {
		s.jobs[name] = &jobRecord{State: statePending}
		s.settled[name] = make(chan struct{})
	}

	return s
}

// execute runs every job whose needs are met, as soon as they are met, and
// returns once no job can make progress. It does not return while a process
// group is still alive, so a cancelled run is torn down before Run returns.
func (s *scheduler) execute(ctx context.Context) {
	var wg sync.WaitGroup

	for _, job := range s.kata.Jobs() {
		wg.Go(func() {
			defer close(s.settled[job.Name])

			s.runJob(ctx, job)
		})
	}

	wg.Wait()
}

// runJob waits for the job's needs and, if they were verified, attempts it
// exactly once.
func (s *scheduler) runJob(ctx context.Context, job *kata.Job) {
	if !s.needsDone(ctx, job) {
		return // stays pending; nothing of it runs and nothing is created
	}

	rec := s.jobs[job.Name]
	rec.StartedAt = stampPtr(time.Now())

	s.logf("%s: start", job.Name)

	rec.State = stateDone
	if failure := s.evaluate(ctx, job, rec); failure != unset {
		rec.State, rec.Failure = stateParked, failure
	}

	rec.FinishedAt = stampPtr(time.Now())

	if rec.Failure != unset {
		s.logf("%s: %s (%s)", job.Name, rec.State, rec.Failure)
	} else {
		s.logf("%s: %s", job.Name, rec.State)
	}
}

// needsDone reports whether every job this one needs reached done. A need
// that parked, or a cancelled run, leaves the job pending.
func (s *scheduler) needsDone(ctx context.Context, job *kata.Job) bool {
	for _, need := range job.Needs {
		select {
		case <-s.settled[need]:
		case <-ctx.Done():
			return false
		}

		if s.jobs[need].State != stateDone {
			s.logf("%s: pending, %s did not complete", job.Name, need)

			return false
		}
	}

	return ctx.Err() == nil
}

// evaluate walks the job in the order the spec states - pre-steps, prompt,
// outputs, post-steps - and returns the first thing that did not pass, or
// unset when all did. Nothing an executor reported about itself is consulted.
func (s *scheduler) evaluate(
	ctx context.Context, job *kata.Job, rec *jobRecord,
) string {
	if err := s.layout.ensureJob(job.Name); err != nil {
		return err.Error()
	}

	failure := s.runSteps(ctx, job, "pre-step", job.PreSteps)
	if failure == unset {
		failure = s.runPrompt(ctx, job, rec)
	}

	if failure == unset {
		failure = s.checkOutputs(job)
	}

	if failure == unset {
		failure = s.runSteps(ctx, job, "post-step", job.PostSteps)
	}

	return failure
}

func (s *scheduler) checkOutputs(job *kata.Job) string {
	for _, out := range job.Outputs {
		if !present(filepath.Join(s.layout.artifacts(), job.Name, out)) {
			return fmt.Sprintf("output %s is missing or empty", out)
		}
	}

	return unset
}

func (s *scheduler) runSteps(
	ctx context.Context, job *kata.Job, kind string, steps []kata.Step,
) string {
	if len(steps) == emptyLen {
		return unset
	}

	logPath := filepath.Join(s.layout.jobDir(job.Name), stepsLog)

	log, err := openLog(logPath)
	if err != nil {
		return fmt.Sprintf("open steps log: %v", err)
	}
	defer log.Close()

	for i, step := range steps {
		argv := kata.ExpandAll(job, step, s.params, s.layout.artifacts())

		code, err := runStep(ctx, argv, s.layout.work(job.Name), log)
		if err != nil {
			return fmt.Sprintf("%s %d: %v", kind, i+stepNum, err)
		}

		if code != exitOK {
			return fmt.Sprintf("%s %d exited %d", kind, i+stepNum, code)
		}
	}

	return unset
}

// runPrompt runs the job's executor once; a job without a prompt passes.
func (s *scheduler) runPrompt(
	ctx context.Context, job *kata.Job, rec *jobRecord,
) string {
	p := job.Executor
	if p == nil {
		return unset
	}

	skills := make([]string, emptyLen, len(p.Skills))
	for _, entry := range p.Skills {
		skills = append(skills, s.skills[entry])
	}

	code, err := runExecutor(ctx, execSpec{
		harness: p.Harness,
		model:   p.Model,
		prompt:  kata.Expand(job, job.Prompt, s.params, s.layout.artifacts()),
		options: p.Options,
		skills:  skills,
		mcps:    p.MCPs,
		timeout: time.Duration(p.TimeoutSeconds) * time.Second,
		home:    s.layout.home(job.Name),
		work:    s.layout.work(job.Name),
		logPath: filepath.Join(s.layout.jobDir(job.Name), executorLog),
	})
	if err != nil {
		return fmt.Sprintf("executor: %v", err)
	}

	rec.ExecutorExit = &code

	if code != exitOK {
		return fmt.Sprintf("executor exited %d", code)
	}

	return unset
}

// present holds an output to what the engine can see on disk: a non-empty
// file, or a directory with at least one entry.
func present(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if info.IsDir() {
		entries, err := os.ReadDir(path)

		return err == nil && len(entries) != emptyLen
	}

	return info.Mode().IsRegular() && info.Size() != emptyLen
}

// record assembles the run's account of itself.
func (s *scheduler) record(started, finished time.Time) *record {
	rec := &record{
		RunID:      s.layout.id,
		Kata:       s.kata.Name,
		Outcome:    OutcomeSucceeded,
		StartedAt:  stamp(started),
		FinishedAt: stamp(finished),
		Skills:     s.skillSources,
		Jobs:       s.jobs,
	}

	for _, job := range s.jobs {
		if job.State != stateDone {
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
