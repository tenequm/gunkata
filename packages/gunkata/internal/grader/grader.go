// Package grader scores one starter-corpus run without a model: it reads the
// engine's record.json plus the files the run left on disk, and reports every
// check the run violates.
package grader

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Variant selects which half of the starter case is being graded.
type Variant string

// The two starter-case variants.
const (
	VariantPass Variant = "pass"
	VariantFail Variant = "fail"
)

// Finding is one violated check.
type Finding struct{ Check, Detail string }

// Grade reports every check the run in runDir violates. An empty slice is a
// clean pass. It errors only when the record cannot be read or parsed, or
// when the variant is unknown.
func Grade(runDir string, variant Variant) ([]Finding, error) {
	if variant != VariantPass && variant != VariantFail {
		return nil, fmt.Errorf("%w: %q", errUnknownVariant, variant)
	}

	rec, err := readRecord(runDir)
	if err != nil {
		return nil, err
	}

	out := checkRecord(rec)
	out = append(out, checkDisk(runDir, rec)...)

	if variant == VariantPass {
		return append(out, checkPassVariant(runDir, rec)...), nil
	}

	return append(out, checkFailVariant(runDir, rec)...), nil
}

var errUnknownVariant = errors.New("unknown grader variant")

type state string

const (
	statePending state = "pending"
	stateDone    state = "done"
	stateParked  state = "parked"
)

type outcome string

const (
	outcomeSucceeded outcome = "succeeded"
	outcomeParked    outcome = "parked"
)

// Starter-case node names are fixed by the case, not read from the graph.
const (
	nodeProduce = "produce"
	nodeGate    = "gate"
	nodeConsume = "consume"
)

const (
	noAttempts    = 0 // a node that never started
	singleAttempt = 1 // lock 4: a node runs once, then parks
	exitOK        = 0
	emptyArtifact = 0 // an empty file is no evidence
)

// Check names. Every Finding carries one of these.
const (
	checkStateEnum       = "record_state_enum"
	checkAttempts        = "record_attempts"
	checkRetried         = "node_retried"
	checkOutcome         = "record_outcome"
	checkTimestamps      = "record_timestamps"
	checkArtifactPath    = "record_artifact_path"
	checkPendingRan      = "pending_node_ran"
	checkPendingDir      = "pending_node_dir"
	checkPendingArtifact = "pending_node_artifact"
	checkDoneArtifact    = "done_node_artifact"
	checkDoneExit        = "done_node_exit"
	checkPassOutcome     = "variant_pass_outcome"
	checkPassNodes       = "variant_pass_nodes"
	checkPassArtifacts   = "variant_pass_artifacts"
	checkPassGateExit    = "variant_pass_gate_exit"
	checkFailOutcome     = "variant_fail_outcome"
	checkFailConsume     = "variant_fail_consume"
	checkFailParked      = "variant_fail_parked"
)

const (
	recordName      = "record.json"
	nodesDir        = "nodes"
	artifactsDir    = "artifacts"
	artifactsPrefix = artifactsDir + string(filepath.Separator)
)

type record struct {
	Outcome    outcome         `json:"outcome"`
	StartedAt  *time.Time      `json:"started_at"`
	FinishedAt *time.Time      `json:"finished_at"`
	Nodes      map[string]node `json:"nodes"`
}

type node struct {
	State        state      `json:"state"`
	Attempts     int        `json:"attempts"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	ExecutorExit *int       `json:"executor_exit"`
	GateExit     *int       `json:"gate_exit"`
	Artifact     *string    `json:"artifact"`
}

func readRecord(runDir string) (record, error) {
	path := filepath.Join(runDir, recordName)

	raw, err := os.ReadFile(path)
	if err != nil {
		return record{}, fmt.Errorf("read record: %w", err)
	}

	var rec record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return record{}, fmt.Errorf("parse %s: %w", path, err)
	}

	return rec, nil
}

type findings []Finding

func (f *findings) add(check, detail string) {
	*f = append(*f, Finding{Check: check, Detail: detail})
}

func sortedNames(rec record) []string {
	return slices.Sorted(maps.Keys(rec.Nodes))
}

// checkRecord validates the record against itself: enums, attempt counts,
// outcome consistency and timestamp order.
func checkRecord(rec record) findings {
	out := checkOutcomeConsistency(rec)

	if reversed(rec.StartedAt, rec.FinishedAt) {
		out.add(checkTimestamps, "run finished before it started")
	}

	for _, name := range sortedNames(rec) {
		out = append(out, checkNode(name, rec.Nodes[name])...)
	}

	return out
}

func checkOutcomeConsistency(rec record) findings {
	out := findings{}
	allDone := allNodesDone(rec)

	switch rec.Outcome {
	case outcomeSucceeded:
		if !allDone {
			out.add(checkOutcome,
				`outcome "succeeded" with a node that is not done`)
		}
	case outcomeParked:
		if allDone {
			out.add(checkOutcome, `outcome "parked" with every node done`)
		}
	default:
		out.add(checkOutcome, fmt.Sprintf("unknown outcome %q", rec.Outcome))
	}

	return out
}

// allNodesDone is false for an empty record: nothing done is not all done.
func allNodesDone(rec record) bool {
	done := false

	for _, n := range rec.Nodes {
		if n.State != stateDone {
			return false
		}

		done = true
	}

	return done
}

func checkNode(name string, n node) findings {
	out := checkAttemptCount(name, n)
	out = append(out, checkNodeTrace(name, n)...)

	if reversed(n.StartedAt, n.FinishedAt) {
		out.add(checkTimestamps, "node "+name+" finished before it started")
	}

	return out
}

// checkAttemptCount ties the attempt count to the state: a node that never
// started has none, a node that reached a verdict has exactly one.
func checkAttemptCount(name string, n node) findings {
	out := findings{}

	if n.Attempts > singleAttempt {
		out.add(checkRetried, fmt.Sprintf(
			"node %s has attempts %d; a parked node is never retried",
			name, n.Attempts))
	}

	if n.State == statePending && n.Attempts != noAttempts {
		out.add(checkAttempts, fmt.Sprintf(
			"pending node %s has attempts %d, want 0", name, n.Attempts))
	}

	if n.State != statePending && n.Attempts == noAttempts {
		out.add(checkAttempts, fmt.Sprintf(
			"node %s is %q with attempts 0, want 1", name, n.State))
	}

	return out
}

func checkNodeTrace(name string, n node) findings {
	switch n.State {
	case statePending:
		return checkPendingNotRun(name, n)
	case stateDone:
		return checkDoneExits(name, n)
	case stateParked:
		return nil
	default:
		out := findings{}
		out.add(checkStateEnum,
			fmt.Sprintf("node %s has state %q", name, n.State))

		return out
	}
}

// checkPendingNotRun asserts the record shows no trace of a run for a node
// the engine says never started.
func checkPendingNotRun(name string, n node) findings {
	out := findings{}

	if n.StartedAt != nil {
		out.add(checkPendingRan, "pending node "+name+" records started_at")
	}

	if n.FinishedAt != nil {
		out.add(checkPendingRan, "pending node "+name+" records finished_at")
	}

	if n.ExecutorExit != nil {
		out.add(checkPendingRan, "pending node "+name+" records executor_exit")
	}

	if n.GateExit != nil {
		out.add(checkPendingRan, "pending node "+name+" records gate_exit")
	}

	return out
}

// checkDoneExits enforces the completion semantics: a done node's evidence
// passed, so any exit code it recorded is zero.
func checkDoneExits(name string, n node) findings {
	out := findings{}

	if n.ExecutorExit != nil && *n.ExecutorExit != exitOK {
		out.add(checkDoneExit, fmt.Sprintf(
			"done node %s has executor_exit %d, want 0",
			name, *n.ExecutorExit))
	}

	if n.GateExit != nil && *n.GateExit != exitOK {
		out.add(checkDoneExit, fmt.Sprintf(
			"done node %s has gate_exit %d, want 0", name, *n.GateExit))
	}

	return out
}

// checkDisk matches the record against the run directory.
func checkDisk(runDir string, rec record) findings {
	out := checkPendingNodeDirs(runDir, rec)

	for _, name := range sortedNames(rec) {
		out = append(out, checkArtifactOnDisk(runDir, name, rec.Nodes[name])...)
	}

	return out
}

// checkPendingNodeDirs asserts a node the engine never started left no home.
func checkPendingNodeDirs(runDir string, rec record) findings {
	out := findings{}

	for _, name := range sortedNames(rec) {
		n := rec.Nodes[name]

		if n.State == statePending && isDir(nodeHome(runDir, name)) {
			out.add(checkPendingDir,
				"pending node "+name+" has a "+nodesDir+"/ directory")
		}
	}

	return out
}

func checkArtifactOnDisk(runDir, name string, n node) findings {
	out := findings{}

	if n.Artifact == nil {
		return out
	}

	path, valid := artifactPath(runDir, *n.Artifact)
	if !valid {
		out.add(checkArtifactPath, fmt.Sprintf(
			"node %s records artifact path %q, want one under %s",
			name, *n.Artifact, artifactsPrefix))

		return out
	}

	return checkArtifactState(name, n.State, path, *n.Artifact)
}

// checkArtifactState asserts only what the state promises. A parked node's
// artifact may be absent, empty or present but rejected by its check, so it
// is not checked here at all.
func checkArtifactState(name string, s state, path, rel string) findings {
	out := findings{}

	if s == stateDone {
		if problem, bad := artifactProblem(path); bad {
			out.add(checkDoneArtifact, "done node "+name+": "+problem)
		}
	}

	if s == statePending && exists(path) {
		out.add(checkPendingArtifact,
			"pending node "+name+" has artifact "+rel+" on disk")
	}

	return out
}

// checkPassVariant holds the pass half to its expectations: every node done
// on one attempt, both artifacts on disk, every declared check green.
func checkPassVariant(runDir string, rec record) findings {
	out := findings{}

	if rec.Outcome != outcomeSucceeded {
		out.add(checkPassOutcome, fmt.Sprintf(
			"outcome is %q, want %q", rec.Outcome, outcomeSucceeded))
	}

	out = append(out, checkPassNodeStates(rec)...)
	out = append(out, checkPassEvidence(runDir, rec)...)

	return append(out, checkPassChecks(rec)...)
}

// checkPassNodeStates reads a missing node as its zero value, so an absent
// node shows up as an empty state instead of needing its own check.
func checkPassNodeStates(rec record) findings {
	out := findings{}

	for _, name := range []string{nodeProduce, nodeGate, nodeConsume} {
		n := rec.Nodes[name]

		if n.State != stateDone {
			out.add(checkPassNodes, fmt.Sprintf(
				"node %s is %q, want %q", name, n.State, stateDone))
		}

		if n.Attempts != singleAttempt {
			out.add(checkPassNodes, fmt.Sprintf(
				"node %s has attempts %d, want 1", name, n.Attempts))
		}
	}

	return out
}

// checkPassEvidence covers the two nodes carrying a prompt and an artifact.
func checkPassEvidence(runDir string, rec record) findings {
	out := checkPassExecutors(rec)

	for _, name := range []string{nodeProduce, nodeConsume} {
		n := rec.Nodes[name]
		out = append(out, checkArtifactEvidence(runDir, name, n)...)
	}

	return out
}

func checkPassExecutors(rec record) findings {
	out := findings{}

	for _, name := range []string{nodeProduce, nodeConsume} {
		if rec.Nodes[name].ExecutorExit == nil {
			out.add(checkPassNodes,
				"node "+name+" records no executor_exit; it never ran")
		}
	}

	return out
}

func checkArtifactEvidence(runDir, name string, n node) findings {
	out := findings{}

	if n.Artifact == nil {
		out.add(checkPassArtifacts, "node "+name+" records no artifact")

		return out
	}

	path, valid := artifactPath(runDir, *n.Artifact)
	if !valid {
		return out // checkNodeOnDisk reports the bad path
	}

	if problem, bad := artifactProblem(path); bad {
		out.add(checkPassArtifacts, "node "+name+": "+problem)
	}

	return out
}

func checkPassChecks(rec record) findings {
	out := findings{}

	for _, name := range sortedNames(rec) {
		if n := rec.Nodes[name]; n.GateExit != nil && *n.GateExit != exitOK {
			out.add(checkPassGateExit, fmt.Sprintf(
				"node %s has gate_exit %d, want 0", name, *n.GateExit))
		}
	}

	for _, name := range []string{nodeGate, nodeConsume} {
		if n := rec.Nodes[name]; n.GateExit == nil {
			out.add(checkPassGateExit,
				"node "+name+" records no gate_exit; its check never ran")
		}
	}

	return out
}

// checkFailVariant holds the fail half to its expectations: the run parked,
// consume never started, and nothing retried.
func checkFailVariant(runDir string, rec record) findings {
	out := checkFailConsumeUntouched(runDir, rec)

	if rec.Outcome != outcomeParked {
		out.add(checkFailOutcome, fmt.Sprintf(
			"outcome is %q, want %q", rec.Outcome, outcomeParked))
	}

	produce, gate := rec.Nodes[nodeProduce], rec.Nodes[nodeGate]
	if produce.State != stateParked && gate.State != stateParked {
		out.add(checkFailParked, "neither "+nodeProduce+" nor "+nodeGate+
			" is parked; the node whose evidence failed must park")
	}

	return out
}

func checkFailConsumeUntouched(runDir string, rec record) findings {
	out := findings{}
	n := rec.Nodes[nodeConsume]

	if n.State != statePending {
		out.add(checkFailConsume, fmt.Sprintf(
			"node %s is %q, want %q", nodeConsume, n.State, statePending))
	}

	if n.Attempts != noAttempts {
		out.add(checkFailConsume, fmt.Sprintf(
			"node %s has attempts %d, want 0", nodeConsume, n.Attempts))
	}

	if n.StartedAt != nil {
		out.add(checkFailConsume,
			"node "+nodeConsume+" records started_at; it must never start")
	}

	if n.Artifact != nil && artifactExists(runDir, *n.Artifact) {
		out.add(checkFailConsume,
			"node "+nodeConsume+" artifact "+*n.Artifact+" exists on disk")
	}

	return out
}

// artifactPath resolves a recorded artifact path. The record states paths
// relative to the run directory and under artifacts/; anything else is a
// record defect rather than a missing file.
func artifactPath(runDir, rel string) (string, bool) {
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) || !strings.HasPrefix(clean, artifactsPrefix) {
		return "", false
	}

	return filepath.Join(runDir, clean), true
}

// artifactProblem says why an artifact fails to count as evidence.
func artifactProblem(path string) (string, bool) {
	info, err := os.Stat(path)

	switch {
	case errors.Is(err, os.ErrNotExist):
		return "artifact missing on disk", true
	case err != nil:
		return "artifact unreadable: " + err.Error(), true
	case !info.Mode().IsRegular():
		return "artifact is not a regular file", true
	case info.Size() == emptyArtifact:
		return "artifact is empty", true
	}

	return "", false
}

func artifactExists(runDir, rel string) bool {
	path, valid := artifactPath(runDir, rel)

	return valid && exists(path)
}

func nodeHome(runDir, name string) string {
	return filepath.Join(runDir, nodesDir, name)
}

func exists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}

func reversed(start, finish *time.Time) bool {
	return start != nil && finish != nil && finish.Before(*start)
}
