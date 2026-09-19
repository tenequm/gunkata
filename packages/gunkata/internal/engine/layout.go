package engine

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// The run directory layout. Everything a run produces lives under it, so a
// reader needs the directory and nothing else.
const (
	artifactsDir = "artifacts"
	paramsDir    = "params"
	jobsDir      = "jobs"
	skillsDir    = "skills"
	homeDir      = "home"
	workDir      = "work"
	executorLog  = "executor.log"   // acpx stderr
	executorFeed = "executor.jsonl" // acpx stdout, the ACP event stream
	runLog       = "gunkata.log"
	stepsLog     = "steps.log"
	recordName   = "record.json"
	kataName     = "kata.yml"
)

const (
	dirPerm     = 0o700
	filePerm    = 0o600
	runIDLayout = "20060102T150405Z"
	// idBytes is two bytes, rendered as the four hex chars of a run id.
	idBytes = 2
	// idAttempts bounds the retries when two runs pick the same id.
	idAttempts = 8
)

var errRunID = errors.New("could not claim a free run directory")

// Outcome is a run's verdict.
type Outcome string

// A run succeeded only when every job did; anything else parks it.
const (
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeParked    Outcome = "parked"
)

// jobState is what the engine concluded about one job.
type jobState string

const (
	statePending jobState = "pending"
	stateDone    jobState = "done"
	stateParked  jobState = "parked"
)

// record is the run's final account of itself, written to record.json.
type record struct {
	RunID      string  `json:"run_id"`
	Kata       string  `json:"kata"`
	Outcome    Outcome `json:"outcome"`
	StartedAt  string  `json:"started_at"`
	FinishedAt string  `json:"finished_at"`
	// Params is every bound value; a file param states the source it was
	// snapshotted from.
	Params map[string]string `json:"params,omitempty"`
	// Skills maps each declared skill to the SHA or local path it was
	// snapshotted from.
	Skills map[string]string `json:"skills,omitempty"`
	// PrivateTmp states whether executors saw their own /tmp or the host's,
	// and TmpReason why not; absent in a run with no executor.
	PrivateTmp *bool                 `json:"private_tmp,omitempty"`
	TmpReason  string                `json:"private_tmp_reason,omitempty"`
	Jobs       map[string]*jobRecord `json:"jobs"`
}

// jobRecord holds one job's evidence. A nil field means that evidence never
// happened.
type jobRecord struct {
	State        jobState `json:"state"`
	StartedAt    *string  `json:"started_at"`
	FinishedAt   *string  `json:"finished_at"`
	ExecutorExit *int     `json:"executor_exit"`
	// Versions maps what the executor ran - acpx, its ACP agent and, for
	// claude, Claude Code - to the version it ran.
	Versions map[string]string `json:"versions,omitempty"`
	// Failure names the first piece of evidence that did not pass.
	Failure string `json:"failure,omitempty"`
	// SkippedMCPs names the optional MCP servers left out for an unset
	// variable.
	SkippedMCPs []string `json:"skipped_mcps,omitempty"`
}

// layout is one run's directory and the paths inside it.
type layout struct {
	id  string
	dir string
}

// newLayout creates a fresh run directory under runsRoot.
func newLayout(runsRoot string) (*layout, error) {
	root, err := filepath.Abs(runsRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve runs root: %w", err)
	}

	if err := os.MkdirAll(root, dirPerm); err != nil {
		return nil, fmt.Errorf("create runs root: %w", err)
	}

	for range idAttempts {
		l, err := claimRunDir(root)
		if err != nil {
			return nil, err
		}

		if l != nil {
			return l, nil
		}
	}

	return nil, errRunID
}

// claimRunDir creates one candidate run directory, returning a nil layout
// when another run already holds that id.
func claimRunDir(root string) (*layout, error) {
	dir := filepath.Join(root, newRunID(time.Now()))

	err := os.Mkdir(dir, dirPerm)
	if errors.Is(err, fs.ErrExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("create run dir: %w", err)
	}

	return &layout{id: filepath.Base(dir), dir: dir}, nil
}

// newRunID is a UTC timestamp plus four hex chars, so two runs started in the
// same second still get their own directory.
func newRunID(now time.Time) string {
	suffix := make([]byte, idBytes)
	// crypto/rand.Read cannot fail; it panics rather than return an error.
	_, _ = rand.Read(suffix)

	return now.UTC().Format(runIDLayout) + hex.EncodeToString(suffix)
}

func (l *layout) artifacts() string {
	return filepath.Join(l.dir, artifactsDir)
}

func (l *layout) skills() string {
	return filepath.Join(l.dir, skillsDir)
}

func (l *layout) jobDir(name string) string {
	return filepath.Join(l.dir, jobsDir, name)
}

func (l *layout) home(name string) string {
	return filepath.Join(l.jobDir(name), homeDir)
}

// work sits inside home: Claude Code searches ancestors of its cwd for
// skills and stops at HOME, so beside it the search would reach the real one.
func (l *layout) work(name string) string {
	return filepath.Join(l.home(name), workDir)
}

// ensureJob creates the job's output and working directories. A job that
// never starts never calls it, so it leaves no trace on disk.
func (l *layout) ensureJob(name string) error {
	dirs := []string{filepath.Join(l.artifacts(), name), l.work(name)}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("create job dir: %w", err)
		}
	}

	return nil
}

// copyKata stores the kata verbatim beside the record.
func (l *layout) copyKata(src string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read kata: %w", err)
	}

	//nolint:gosec // the kata path is the run's input, and dst is the run dir
	err = os.WriteFile(filepath.Join(l.dir, kataName), raw, filePerm)
	if err != nil {
		return fmt.Errorf("copy kata: %w", err)
	}

	return nil
}

// snapshotParams copies every param whose value is an existing regular file
// into the run dir, and returns the values placeholders expand to.
func (l *layout) snapshotParams(bound map[string]string) (
	map[string]string, error,
) {
	values := make(map[string]string, len(bound))

	for key, value := range bound {
		info, err := os.Stat(value)
		if err != nil || !info.Mode().IsRegular() {
			values[key] = value

			continue
		}

		dst := filepath.Join(l.dir, paramsDir, key, filepath.Base(value))
		if err := copyFile(value, dst); err != nil {
			return nil, fmt.Errorf("snapshot param %q: %w", key, err)
		}

		values[key] = dst
	}

	return values, nil
}

func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), dirPerm); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dst), err)
	}

	//nolint:gosec // G703: dst is the run dir plus a validated param name
	if err := os.WriteFile(dst, raw, filePerm); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}

	return nil
}

// writeRecord writes record.json through a temp file and a rename, so a
// reader either sees the previous state or the final one.
func (l *layout) writeRecord(rec *record) error {
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("encode record: %w", err)
	}

	return replaceFile(filepath.Join(l.dir, recordName), append(raw, '\n'))
}

func writeAndClose(file *os.File, raw []byte) error {
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()

		return fmt.Errorf("write %s: %w", file.Name(), err)
	}

	if err := file.Chmod(filePerm); err != nil {
		_ = file.Close()

		return fmt.Errorf("chmod %s: %w", file.Name(), err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", file.Name(), err)
	}

	return nil
}

// stamp formats a time the way the record states it.
func stamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func stampPtr(t time.Time) *string {
	s := stamp(t)

	return &s
}
