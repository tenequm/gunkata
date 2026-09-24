package engine

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fanOutKata is a fan-out head whose items the stub writes in round 1 only,
// a template that derives from its item, and a collector that waits for the
// whole loop and reads the tree by {{fanout:}}.
const fanOutKata = `
name: stub-fanout
params:
  count: {}
  rounds: {default: "3"}
agents:
  stub:
    harness: claude
    model: stub-model
    timeout_seconds: 7
workflow:
  escalate:
    agent: stub
    prompt: |
      ACTION=items
      COUNT={{param:count}}
      TARGET={{output:items}}
    outputs: [items]
    fan-out:
      items: items
      job: probe
      max_rounds: 3
      max_items: 5
  probe:
    agent: stub
    prompt: |
      ACTION=derive
      SOURCE={{item}}
      TARGET={{output:out.txt}}
    outputs: [out.txt]
  collect:
    needs: [escalate]
    post-steps:
      - [test, -d, "{{fanout:escalate}}"]
`

func artifact(t *testing.T, runDir string, parts ...string) string {
	t.Helper()

	return readFile(t, filepath.Join(
		append([]string{runDir, artifactsDir}, parts...)...))
}

func TestFanOutRunsRoundsUntilAnEmptyOne(t *testing.T) {
	stubACPX(t)

	res := mustRun(t, fanOutKata, map[string]string{"count": "2"})
	if res.Outcome != OutcomeSucceeded {
		t.Fatalf(outcomeFmt, res.Outcome, OutcomeSucceeded)
	}

	rec := readRecord(t, res.RunDir)

	loop := rec.FanOut["escalate"]
	if loop == nil {
		t.Fatal("record holds no fan-out summary for escalate")
	}

	if loop.Rounds != 2 || loop.Capped || !slices.Equal(loop.Items, []int{2, 0}) {
		t.Errorf("fan-out = %+v, want rounds 2, items [2 0], not capped", loop)
	}

	// Both rounds' heads and both of round 1's items ran, each under its own
	// key, so the record names every job the run actually carried out.
	for _, name := range []string{
		"escalate/round-1", "escalate/round-2",
		"probe/round-1/item-1", "probe/round-1/item-2", "collect",
	} {
		job := rec.Jobs[name]
		if job == nil || job.State != stateDone {
			t.Errorf("job %q = %+v, want done", name, job)
		}
	}

	if _, ok := rec.Jobs["probe/round-2/item-1"]; ok {
		t.Error("an empty round still ran an item")
	}

	// {{item}} reached the instance: each probe derived from its own item.
	for i, want := range []string{"work 1 derived", "work 2 derived"} {
		key := "probe/round-1/item-" + string(rune('1'+i))
		if got := artifact(t, res.RunDir, key, "out.txt"); !strings.Contains(got, want) {
			t.Errorf("%s out.txt = %q, want %q", key, got, want)
		}
	}

	if item := rec.Jobs["probe/round-1/item-2"].Item; !strings.HasSuffix(item, "item-2.md") {
		t.Errorf("recorded item = %q, want the round's second item file", item)
	}
}

func TestFanOutEndsOnARoundWithNoItems(t *testing.T) {
	stubACPX(t)

	res := mustRun(t, fanOutKata, map[string]string{"count": "0"})
	if res.Outcome != OutcomeSucceeded {
		t.Fatalf(outcomeFmt, res.Outcome, OutcomeSucceeded)
	}

	rec := readRecord(t, res.RunDir)

	loop := rec.FanOut["escalate"]
	if loop.Rounds != 1 || loop.Capped || !slices.Equal(loop.Items, []int{0}) {
		t.Errorf("fan-out = %+v, want one round of zero items", loop)
	}

	for name := range rec.Jobs {
		if strings.HasPrefix(name, "probe/") {
			t.Errorf("job %q ran although the round produced no items", name)
		}
	}

	// The head's empty items directory is evidence, not a missing output.
	if rec.Jobs["escalate/round-1"].State != stateDone {
		t.Error("an empty items directory failed the head's output check")
	}
}

// cappedKata never runs dry, so only max_rounds can stop it.
const cappedKata = `
name: stub-fanout-cap
agents:
  stub:
    harness: claude
    model: stub-model
    timeout_seconds: 7
workflow:
  escalate:
    agent: stub
    prompt: |
      ACTION=loop
      TARGET={{output:items}}
    outputs: [items]
    fan-out:
      items: items
      job: probe
      max_rounds: 2
      max_items: 4
  probe:
    agent: stub
    prompt: |
      ACTION=derive
      SOURCE={{item}}
      TARGET={{output:out.txt}}
    outputs: [out.txt]
`

func TestFanOutStopsAtMaxRounds(t *testing.T) {
	stubACPX(t)

	res := mustRun(t, cappedKata, nil)
	if res.Outcome != OutcomeSucceeded {
		t.Fatalf(outcomeFmt, res.Outcome, OutcomeSucceeded)
	}

	rec := readRecord(t, res.RunDir)

	loop := rec.FanOut["escalate"]
	if loop.Rounds != 2 || !loop.Capped || !slices.Equal(loop.Items, []int{1, 1}) {
		t.Errorf("fan-out = %+v, want two capped rounds of one item", loop)
	}

	if _, ok := rec.Jobs["escalate/round-3"]; ok {
		t.Error("the loop ran past max_rounds")
	}
}

// overCapKata asks for three items where two are allowed.
const overCapKata = `
name: stub-fanout-items
agents:
  stub:
    harness: claude
    model: stub-model
    timeout_seconds: 7
workflow:
  escalate:
    agent: stub
    prompt: |
      ACTION=items
      COUNT=3
      TARGET={{output:items}}
    outputs: [items]
    fan-out:
      items: items
      job: probe
      max_rounds: 2
      max_items: 2
  probe:
    agent: stub
    prompt: |
      ACTION=derive
      SOURCE={{item}}
      TARGET={{output:out.txt}}
    outputs: [out.txt]
  collect:
    needs: [escalate]
    post-steps:
      - [test, -d, "{{fanout:escalate}}"]
`

func TestFanOutParksOverMaxItems(t *testing.T) {
	stubACPX(t)

	res := mustRun(t, overCapKata, nil)
	if res.Outcome != OutcomeParked {
		t.Fatalf(outcomeFmt, res.Outcome, OutcomeParked)
	}

	rec := readRecord(t, res.RunDir)

	if failure := rec.Jobs["escalate"].Failure; !strings.Contains(failure, "max_items") {
		t.Errorf("escalate failure = %q, want it to name max_items", failure)
	}

	if state := rec.Jobs["collect"].State; state != statePending {
		t.Errorf("collect = %q, want %q: nothing releases past a parked head", state, statePending)
	}

	for name := range rec.Jobs {
		if strings.HasPrefix(name, "probe/") {
			t.Errorf("job %q ran although the round broke the cap", name)
		}
	}
}

// parkedItemKata has a template that writes nothing, so every instance parks.
// Its head emits two items in round 1 and one in round 2, so the loop has to
// survive both to reach the empty round 3.
const parkedItemKata = `
name: stub-fanout-parked-item
params:
  count: {}
agents:
  stub:
    harness: claude
    model: stub-model
    timeout_seconds: 7
workflow:
  escalate:
    agent: stub
    prompt: |
      ACTION=rounds
      COUNT={{param:count}}
      TARGET={{output:items}}
    outputs: [items]
    fan-out:
      items: items
      job: probe
      max_rounds: 4
      max_items: 4
  probe:
    agent: stub
    prompt: |
      ACTION=none
      TARGET={{output:out.txt}}
    outputs: [out.txt]
  collect:
    needs: [escalate]
    post-steps:
      - [test, -d, "{{fanout:escalate}}"]
`

// A parked instance is a sample that did not come back, not a dependency
// failure: the round carries on, the loop reaches its own end, and the head's
// dependents release.
func TestFanOutCarriesOnPastAParkedItem(t *testing.T) {
	stubACPX(t)

	res := mustRun(t, parkedItemKata, map[string]string{"count": "2"})

	// The run still records the park; what changed is that it no longer ends
	// the loop.
	if res.Outcome != OutcomeParked {
		t.Fatalf(outcomeFmt, res.Outcome, OutcomeParked)
	}

	rec := readRecord(t, res.RunDir)

	loop := rec.FanOut["escalate"]
	if loop.Rounds != 3 || loop.Capped ||
		!slices.Equal(loop.Items, []int{2, 1, 0}) ||
		!slices.Equal(loop.Parked, []int{2, 1, 0}) {
		t.Errorf("fan-out = %+v, want 3 rounds, items [2 1 0], parked [2 1 0]", loop)
	}

	// The head reached its own terminating answer and released its dependents.
	if state := rec.Jobs["escalate"].State; state != stateDone {
		t.Errorf("escalate = %q, want %q: a missing sample is not a failed need",
			state, stateDone)
	}

	if failure := rec.Jobs["escalate"].Failure; failure != "" {
		t.Errorf("escalate failure = %q, want none", failure)
	}

	if state := rec.Jobs["collect"].State; state != stateDone {
		t.Errorf("collect = %q, want %q", state, stateDone)
	}

	// Each instance is still held to its own evidence and recorded parked.
	for _, name := range []string{
		"probe/round-1/item-1", "probe/round-1/item-2", "probe/round-2/item-1",
	} {
		if state := rec.Jobs[name].State; state != stateParked {
			t.Errorf("%s = %q, want %q", name, state, stateParked)
		}
	}
}

// The note is what stops the next head reading silence as "never asked".
func TestFanOutWritesEvidenceForAParkedItem(t *testing.T) {
	stubACPX(t)

	res := mustRun(t, parkedItemKata, map[string]string{"count": "2"})

	note := artifact(t, res.RunDir, "probe/round-1/item-2", "parked.md")
	for _, want := range []string{
		"did not complete", "item-2.md", "out.txt is missing or empty", "open",
	} {
		if !strings.Contains(note, want) {
			t.Errorf("parked.md = %q, want it to mention %q", note, want)
		}
	}

	// It is reachable exactly where a head looks: under {{fanout:probe}}.
	path := filepath.Join(res.RunDir, artifactsDir, "probe", "round-1", "item-2", "parked.md")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("parked note not under the fan-out tree: %v", err)
	}
}

// A round in which every sample fails is still a round: the loop does not
// mistake a total miss for a reason to stop.
func TestFanOutCarriesOnWhenEveryItemParks(t *testing.T) {
	stubACPX(t)

	res := mustRun(t, parkedItemKata, map[string]string{"count": "4"})
	rec := readRecord(t, res.RunDir)

	loop := rec.FanOut["escalate"]
	if !slices.Equal(loop.Items, loop.Parked) {
		t.Errorf("fan-out = %+v, want every item of every round parked", loop)
	}

	if loop.Rounds != 3 || !slices.Equal(loop.Items, []int{4, 1, 0}) {
		t.Errorf("fan-out = %+v, want 3 rounds of [4 1 0]", loop)
	}

	if state := rec.Jobs["escalate/round-2"].State; state != stateDone {
		t.Errorf("round 2 head = %q, want %q: round 1 lost every sample and the "+
			"head still ran", state, stateDone)
	}

	if state := rec.Jobs["collect"].State; state != stateDone {
		t.Errorf("collect = %q, want %q", state, stateDone)
	}
}

// What still parks the run: a head that fails its own evidence.
const parkedHeadKata = `
name: stub-fanout-parked-head
agents:
  stub:
    harness: claude
    model: stub-model
    timeout_seconds: 7
workflow:
  escalate:
    agent: stub
    prompt: |
      ACTION=none
      TARGET={{output:ledger.md}}
    outputs: [items, ledger.md]
    fan-out:
      items: items
      job: probe
      max_rounds: 2
      max_items: 2
  probe:
    agent: stub
    prompt: |
      ACTION=derive
      SOURCE={{item}}
      TARGET={{output:out.txt}}
    outputs: [out.txt]
  collect:
    needs: [escalate]
    post-steps:
      - [test, -d, "{{fanout:escalate}}"]
`

func TestFanOutParksOnAParkedHead(t *testing.T) {
	stubACPX(t)

	res := mustRun(t, parkedHeadKata, nil)
	if res.Outcome != OutcomeParked {
		t.Fatalf(outcomeFmt, res.Outcome, OutcomeParked)
	}

	rec := readRecord(t, res.RunDir)

	if failure := rec.Jobs["escalate"].Failure; !strings.Contains(failure, "ledger.md") {
		t.Errorf("escalate failure = %q, want the head's own missing output", failure)
	}

	if _, ok := rec.Jobs["escalate/round-2"]; ok {
		t.Error("the loop ran on past a parked head")
	}

	if state := rec.Jobs["collect"].State; state != statePending {
		t.Errorf("collect = %q, want %q: a parked head still holds its dependents",
			state, statePending)
	}
}
