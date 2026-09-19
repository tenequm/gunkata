package engine

import (
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

// itemFailureKata has a template that writes nothing, so its instance parks.
const itemFailureKata = `
name: stub-fanout-parked-item
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
      COUNT=1
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
      ACTION=none
      TARGET={{output:out.txt}}
    outputs: [out.txt]
  collect:
    needs: [escalate]
    post-steps:
      - [test, -d, "{{fanout:escalate}}"]
`

func TestFanOutParksWhenAnItemParks(t *testing.T) {
	stubACPX(t)

	res := mustRun(t, itemFailureKata, nil)
	if res.Outcome != OutcomeParked {
		t.Fatalf(outcomeFmt, res.Outcome, OutcomeParked)
	}

	rec := readRecord(t, res.RunDir)

	if state := rec.Jobs["probe/round-1/item-1"].State; state != stateParked {
		t.Errorf("item = %q, want %q", state, stateParked)
	}

	if failure := rec.Jobs["escalate"].Failure; !strings.Contains(failure, "item 1") {
		t.Errorf("escalate failure = %q, want it to name the item", failure)
	}

	if _, ok := rec.Jobs["escalate/round-2"]; ok {
		t.Error("the loop ran another round after an item parked")
	}

	if state := rec.Jobs["collect"].State; state != statePending {
		t.Errorf("collect = %q, want %q", state, statePending)
	}
}
