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
	"slices"
	"time"
)

// The run directory layout. Everything a run produces lives under it, so a
// reader needs the directory and nothing else.
const (
	artifactsDir = "artifacts"
	inputsDir    = "inputs"
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

var (
	errRunID        = errors.New("could not claim a free run directory")
	errUnknownInput = errors.New("no such input is declared by the graph")
	errUnboundInput = errors.New("declared input has no --input binding")
	errInputSource  = errors.New("input source is not a non-empty regular file")
)

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
	RunID      string  `json:"run_id"`
	Graph      string  `json:"graph"`
	Outcome    Outcome `json:"outcome"`
	StartedAt  string  `json:"started_at"`
	FinishedAt string  `json:"finished_at"`
	// Inputs names each declared input and the source file it was bound to.
	// A graph that declares none records none.
	Inputs map[string]string      `json:"inputs,omitempty"`
	Nodes  map[string]*nodeRecord `json:"nodes"`
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

func (l *layout) inputs() string {
	return filepath.Join(l.dir, inputsDir)
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

// bindInputs copies every file bound to a declared input into the run's
// inputs dir and reports the absolute source each name was bound to. It runs
// before any node does, so a misbound run fails outright instead of parking.
func (l *layout) bindInputs(
	declared []string, bound map[string]string,
) (map[string]string, error) {
	for name := range bound {
		if !slices.Contains(declared, name) {
			return nil, fmt.Errorf("%w: %q", errUnknownInput, name)
		}
	}

	if len(declared) == emptyLen {
		return nil, nil
	}

	return l.copyInputs(declared, bound)
}

// copyInputs copies one file per declared input, in declaration order.
func (l *layout) copyInputs(
	declared []string, bound map[string]string,
) (map[string]string, error) {
	sources := make(map[string]string, len(declared))

	for _, name := range declared {
		src, ok := bound[name]
		if !ok {
			return nil, fmt.Errorf("%w: %q", errUnboundInput, name)
		}

		abs, err := l.copyInput(name, src)
		if err != nil {
			return nil, err
		}

		sources[name] = abs
	}

	return sources, nil
}

// copyInput copies the bound file's bytes into the run's inputs dir and
// reports the absolute source it came from, so the run holds its own copy of
// what it was given rather than a link to a file someone else owns.
func (l *layout) copyInput(name, src string) (string, error) {
	abs, err := filepath.Abs(src)
	if err != nil {
		return unset, fmt.Errorf("resolve input %q: %w", name, err)
	}

	raw, err := readInputSource(abs)
	if err != nil {
		return unset, fmt.Errorf("input %q: %w", name, err)
	}

	dst := filepath.Join(l.inputs(), name)
	if err := os.MkdirAll(filepath.Dir(dst), dirPerm); err != nil {
		return unset, fmt.Errorf("create inputs dir: %w", err)
	}

	if err := os.WriteFile(dst, raw, filePerm); err != nil {
		return unset, fmt.Errorf("write input %q: %w", name, err)
	}

	return abs, nil
}

// readInputSource holds a binding to what can be evidence: a regular file
// with something in it.
func readInputSource(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errInputSource, err)
	}

	switch {
	case !info.Mode().IsRegular():
		return nil, fmt.Errorf("%w: %s is not a regular file",
			errInputSource, path)
	case info.Size() == emptyLen:
		return nil, fmt.Errorf("%w: %s is empty", errInputSource, path)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errInputSource, err)
	}

	return raw, nil
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
