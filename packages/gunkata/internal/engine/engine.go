// Package engine runs a gunkata kata. It lays out the run directory, runs each
// job's steps and executor, releases a job's dependents only against verified
// evidence, and records what happened.
package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// Log attribute keys, shared so every event reads the same way.
const (
	keyJob   = "job"
	keyErr   = "err"
	keyDur   = "dur_ms"
	keyExit  = "exit"
	keyState = "state"
	keyTitle = "title"
	keyKind  = "kind"
	keyTool  = "tool"
)

var (
	errUnknownParam = errors.New("no such param is declared by the kata")
	errUnboundParam = errors.New("required param is not bound")
)

// levelFor maps whether a thing passed to its log level: what did not pass is
// a warning.
var levelFor = map[bool]slog.Level{true: slog.LevelInfo, false: slog.LevelWarn}

// Options configures one run.
type Options struct {
	KataPath string
	RunsRoot string
	// Params binds declared params by key. Unbound params take their
	// default; a required param must be bound.
	Params map[string]string
	// Progress receives the run's log as human-readable text: the events
	// gunkata.log holds as JSON lines. It is never the run's result; that is
	// the record.
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
	k, bound, err := prepare(opts)
	if err != nil {
		return Result{}, err
	}

	l, err := newLayout(opts.RunsRoot)
	if err != nil {
		return Result{}, err
	}

	res := Result{RunDir: l.dir}

	log, logFile, err := openRunLog(l.dir, opts.Progress)
	if err != nil {
		return res, err
	}
	defer logFile.Close()

	res.Outcome, err = carryOut(ctx, k, l, bound, opts.KataPath, log)
	if err != nil {
		log.Error("run failed", keyErr, err)
	}

	return res, err
}

// prepare loads the kata and binds its params, rejecting what would fail
// mid-run before anything is created.
func prepare(opts Options) (*kata.Kata, map[string]string, error) {
	k, err := kata.Load(opts.KataPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load kata: %w", err)
	}

	bound, err := bindParams(k, opts.Params)
	if err != nil {
		return nil, nil, err
	}

	if checkErr := preflight(k); checkErr != nil {
		return nil, nil, checkErr
	}

	return k, bound, nil
}

// openRunLog logs the run as JSON lines to gunkata.log in runDir and, when
// progress is set, as text to progress too.
func openRunLog(runDir string, progress io.Writer) (
	*slog.Logger, *os.File, error,
) {
	file, err := openLog(filepath.Join(runDir, runLog))
	if err != nil {
		return nil, nil, err
	}

	handler := slog.Handler(slog.NewJSONHandler(file, nil))
	if progress != nil {
		handler = slog.NewMultiHandler(handler,
			slog.NewTextHandler(progress, nil))
	}

	return slog.New(handler), file, nil
}

// carryOut runs a prepared kata in its run dir and writes the record.
func carryOut(
	ctx context.Context, k *kata.Kata, l *layout, bound map[string]string,
	kataPath string, log *slog.Logger,
) (Outcome, error) {
	began := time.Now()
	log.Info("run start", "run_id", l.id, "kata", k.Name,
		"run_dir", l.dir, "params", bound)

	s, err := setup(ctx, k, l, bound, kataPath, log)
	if err != nil {
		return unset, err
	}

	started := time.Now()

	s.execute(ctx)

	rec := s.record(started, time.Now())
	rec.Params = bound

	if err := l.writeRecord(rec); err != nil {
		return unset, err
	}

	log.Info("record written", "path", filepath.Join(l.dir, recordName))
	log.Info("run end", "outcome", rec.Outcome, durSince(began))

	if ctx.Err() != nil {
		return rec.Outcome, fmt.Errorf("run interrupted: %w", ctx.Err())
	}

	return rec.Outcome, nil
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
// variables a required MCP server lacks.
func preflight(k *kata.Kata) error {
	if err := checkSkills(k); err != nil {
		return err
	}

	for _, job := range k.Jobs() {
		if err := checkMCPs(job); err != nil {
			return fmt.Errorf("job %q: %w", job.Name, err)
		}
	}

	return nil
}

func hasExecutor(job *kata.Job) bool { return job.Executor != nil }

// sharedTmp is what each executor sees replaced by its own HOME/tmp, since
// agents write there whatever TMPDIR says.
const sharedTmp = "/tmp"

var errRunsUnderTmp = errors.New("runs root is under " + sharedTmp +
	", which each executor sees replaced by its own")

// checkPrivateTmp says why the run's executors cannot get a private /tmp,
// or nil when they can: the run dir must not lie under what that /tmp
// hides, and the host must allow the namespace.
func checkPrivateTmp(runDir string) error {
	rel, err := filepath.Rel(sharedTmp, runDir)
	if err == nil && !strings.HasPrefix(rel, "..") {
		return fmt.Errorf("%w: %s", errRunsUnderTmp, runDir)
	}

	return probePrivateTmp()
}

// choosePrivateTmp probes once per run with an executor job. Where the host
// cannot give executors a private /tmp they run with the shared one, which
// the log warns about and the record states.
func (s *scheduler) choosePrivateTmp() {
	if !slices.ContainsFunc(s.kata.Jobs(), hasExecutor) {
		return
	}

	err := checkPrivateTmp(s.layout.dir)
	active := err == nil
	s.privateTmp = &active

	if err != nil {
		s.tmpReason = err.Error()
		s.log.Warn("executors share the host /tmp", keyErr, err)
	}
}

// checkMCPs refuses a required MCP server whose URL references an unset
// variable; an optional one is skipped at spawn instead.
func checkMCPs(job *kata.Job) error {
	if job.Executor == nil {
		return nil
	}

	for _, mcp := range job.Executor.MCPs {
		if name := unsetVar(mcp.URL); mcp.Required && name != unset {
			return fmt.Errorf("%w: %s", errMCPVar, name)
		}
	}

	return nil
}

// setup fills the run dir with what the run is given - the kata, param
// snapshots, skill snapshots - before any job starts.
func setup(
	ctx context.Context, k *kata.Kata, l *layout, bound map[string]string,
	kataPath string, log *slog.Logger,
) (*scheduler, error) {
	if err := l.copyKata(kataPath); err != nil {
		return nil, err
	}

	params, err := l.snapshotParams(bound)
	if err != nil {
		return nil, err
	}

	skills, err := fetchSkills(ctx, k, l.skills(), log)
	if err != nil {
		return nil, err
	}

	s := newScheduler(k, l, params, skills.snapshots, log)
	s.skillSources = skills.sources
	s.choosePrivateTmp()

	return s, nil
}

// scheduler holds the state of one run in flight.
type scheduler struct {
	kata   *kata.Kata
	layout *layout
	params map[string]string // placeholder values
	skills map[string]string // skill entry -> snapshot dir
	log    *slog.Logger
	// jobs holds one record per job. Each is written only by that job's own
	// goroutine, and read by its dependents once it has settled.
	jobs map[string]*jobRecord
	// settled is closed per job once its record is final.
	settled      map[string]chan struct{}
	skillSources map[string]string
	// privateTmp is whether executors get their own /tmp; nil when the run
	// has no executor. tmpReason says why not.
	privateTmp *bool
	tmpReason  string
}

func newScheduler(
	k *kata.Kata, l *layout, params, skills map[string]string,
	log *slog.Logger,
) *scheduler {
	s := &scheduler{
		kata:    k,
		layout:  l,
		params:  params,
		skills:  skills,
		log:     log,
		jobs:    make(map[string]*jobRecord, len(k.Workflow)),
		settled: make(map[string]chan struct{}, len(k.Workflow)),
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
	log := s.log.With(keyJob, job.Name)
	if !s.needsDone(ctx, job, log) {
		return // stays pending; nothing of it runs and nothing is created
	}

	rec := s.jobs[job.Name]
	began := time.Now()
	rec.StartedAt = stampPtr(began)

	log.Info("job start")

	rec.State = stateDone
	if failure := s.evaluate(ctx, job, rec, log); failure != unset {
		rec.State, rec.Failure = stateParked, failure
	}

	rec.FinishedAt = stampPtr(time.Now())

	attrs := []any{keyState, rec.State, durSince(began)}
	if rec.Failure != unset {
		attrs = append(attrs, "failure", rec.Failure)
	}

	log.Log(ctx, levelFor[rec.State == stateDone], "job end", attrs...)
}

// needsDone reports whether every job this one needs reached done. A need
// that parked, or a cancelled run, leaves the job pending.
func (s *scheduler) needsDone(
	ctx context.Context, job *kata.Job, log *slog.Logger,
) bool {
	for _, need := range job.Needs {
		select {
		case <-s.settled[need]:
		case <-ctx.Done():
			return false
		}

		if state := s.jobs[need].State; state != stateDone {
			log.Warn("job pending", "need", need, keyState, state)

			return false
		}
	}

	return ctx.Err() == nil
}

// evaluate walks the job in the order the spec states - pre-steps, prompt,
// outputs, post-steps - and returns the first thing that did not pass, or
// unset when all did. Nothing an executor reported about itself is consulted.
func (s *scheduler) evaluate(
	ctx context.Context, job *kata.Job, rec *jobRecord, log *slog.Logger,
) string {
	if err := s.layout.ensureJob(job.Name); err != nil {
		return err.Error()
	}

	failure := s.runSteps(ctx, job, "pre-step", job.PreSteps, log)
	if failure == unset {
		failure = s.runPrompt(ctx, job, rec, log)
	}

	if failure == unset {
		failure = s.checkOutputs(job, log)
	}

	if failure == unset {
		failure = s.runSteps(ctx, job, "post-step", job.PostSteps, log)
	}

	return failure
}

func (s *scheduler) checkOutputs(job *kata.Job, log *slog.Logger) string {
	for _, out := range job.Outputs {
		ok := present(filepath.Join(s.layout.artifacts(), job.Name, out))
		log.Log(context.Background(), levelFor[ok], "output",
			"output", out, "present", ok)

		if !ok {
			return fmt.Sprintf("output %s is missing or empty", out)
		}
	}

	return unset
}

func (s *scheduler) runSteps(
	ctx context.Context, job *kata.Job, kind string, steps []kata.Step,
	log *slog.Logger,
) string {
	if len(steps) == emptyLen {
		return unset
	}

	logPath := filepath.Join(s.layout.jobDir(job.Name), stepsLog)

	out, err := openLog(logPath)
	if err != nil {
		return fmt.Sprintf("open steps log: %v", err)
	}
	defer out.Close()

	for i, step := range steps {
		argv := kata.ExpandAll(job, step, s.params, s.layout.artifacts())

		began := time.Now()
		code, err := runStep(ctx, argv, s.layout.work(job.Name), out)
		log.Log(ctx, levelFor[err == nil && code == exitOK], "step",
			keyKind, kind, "index", i+stepNum, "argv", argv,
			keyExit, code, durSince(began))

		if failure := stepFailure(kind, i, code, err); failure != unset {
			return failure
		}
	}

	return unset
}

func stepFailure(kind string, i, code int, err error) string {
	if err != nil {
		return fmt.Sprintf("%s %d: %v", kind, i+stepNum, err)
	}

	if code != exitOK {
		return fmt.Sprintf("%s %d exited %d", kind, i+stepNum, code)
	}

	return unset
}

// runPrompt runs the job's executor once; a job without a prompt passes.
func (s *scheduler) runPrompt(
	ctx context.Context, job *kata.Job, rec *jobRecord, log *slog.Logger,
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
		mcps:    usableMCPs(job, rec, log),
		timeout: time.Duration(p.TimeoutSeconds) * time.Second,
		home:    s.layout.home(job.Name),
		work:    s.layout.work(job.Name),
		jobDir:  s.layout.jobDir(job.Name),
		// set only for a run with an executor job, which this is
		privateTmp: *s.privateTmp,
		log:        log,
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

// usableMCPs is the job's MCP URLs minus the optional servers whose
// variables are unset, each skip warned about and recorded.
func usableMCPs(job *kata.Job, rec *jobRecord, log *slog.Logger) []string {
	urls := make([]string, emptyLen, len(job.Executor.MCPs))

	for _, mcp := range job.Executor.MCPs {
		name := unsetVar(mcp.URL)
		if name == unset || mcp.Required {
			urls = append(urls, mcp.URL)

			continue
		}

		server := mcpName(mcp.URL)
		rec.SkippedMCPs = append(rec.SkippedMCPs, server)
		log.Warn("skipping MCP: variable unset", "server", server, "var", name)
	}

	return urls
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
		PrivateTmp: s.privateTmp,
		TmpReason:  s.tmpReason,
		Jobs:       s.jobs,
	}

	for _, job := range s.jobs {
		if job.State != stateDone {
			rec.Outcome = OutcomeParked
		}
	}

	return rec
}

func durSince(began time.Time) slog.Attr {
	return slog.Int64(keyDur, time.Since(began).Milliseconds())
}
