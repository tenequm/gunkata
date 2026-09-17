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
	nodesDir     = "nodes"
	homeDir      = "home"
	workDir      = "work"
	logName      = "executor.log"
	recordName   = "record.json"
	graphName    = "graph.yaml"
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

// A run succeeded only when every node did; anything else parks it.
const (
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeParked    Outcome = "parked"
)

// nodeState is what the engine concluded about one node.
type nodeState string

const (
	statePending nodeState = "pending"
	stateDone    nodeState = "done"
	stateParked  nodeState = "parked"
)

// record is the run's final account of itself, written to record.json.
type record struct {
	RunID      string                 `json:"run_id"`
	Graph      string                 `json:"graph"`
	Outcome    Outcome                `json:"outcome"`
	StartedAt  string                 `json:"started_at"`
	FinishedAt string                 `json:"finished_at"`
	Nodes      map[string]*nodeRecord `json:"nodes"`
}

// nodeRecord holds one node's evidence: the exit codes it produced and the
// artifact it declared. A nil field means that evidence never happened.
type nodeRecord struct {
	State        nodeState `json:"state"`
	Attempts     int       `json:"attempts"`
	StartedAt    *string   `json:"started_at"`
	FinishedAt   *string   `json:"finished_at"`
	ExecutorExit *int      `json:"executor_exit"`
	GateExit     *int      `json:"gate_exit"`
	Artifact     *string   `json:"artifact"`
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

	l := &layout{id: filepath.Base(dir), dir: dir}
	if err := os.MkdirAll(l.artifacts(), dirPerm); err != nil {
		return nil, fmt.Errorf("create artifacts dir: %w", err)
	}

	return l, nil
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

func (l *layout) nodeDir(name string) string {
	return filepath.Join(l.dir, nodesDir, name)
}

func (l *layout) home(name string) string {
	return filepath.Join(l.nodeDir(name), homeDir)
}

func (l *layout) work(name string) string {
	return filepath.Join(l.nodeDir(name), workDir)
}

func (l *layout) logPath(name string) string {
	return filepath.Join(l.nodeDir(name), logName)
}

// ensureNode creates the node's executor directories. A node without a
// prompt never calls it, so a check-only node leaves no nodes/ entry.
func (l *layout) ensureNode(name string) error {
	for _, dir := range []string{l.home(name), l.work(name)} {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("create node dir: %w", err)
		}
	}

	return nil
}

// copyGraph stores the input graph verbatim beside the record.
func (l *layout) copyGraph(src string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read graph: %w", err)
	}

	dst := filepath.Join(l.dir, graphName)
	//nolint:gosec // the graph path is the run's input, and dst is the run dir
	if err := os.WriteFile(dst, raw, filePerm); err != nil {
		return fmt.Errorf("copy graph: %w", err)
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

	tmp, err := os.CreateTemp(l.dir, recordName+".*")
	if err != nil {
		return fmt.Errorf("create record temp file: %w", err)
	}

	if err := writeAndClose(tmp, append(raw, '\n')); err != nil {
		_ = os.Remove(tmp.Name())

		return err
	}

	dst := filepath.Join(l.dir, recordName)
	if err := os.Rename(tmp.Name(), dst); err != nil {
		_ = os.Remove(tmp.Name())

		return fmt.Errorf("install record: %w", err)
	}

	return nil
}

func writeAndClose(file *os.File, raw []byte) error {
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()

		return fmt.Errorf("write record: %w", err)
	}

	if err := file.Chmod(filePerm); err != nil {
		_ = file.Close()

		return fmt.Errorf("chmod record: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close record: %w", err)
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
