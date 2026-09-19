package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/tenequm/gunkata/internal/kata"
)

// roundSeg and itemSeg name an instance's directories under the declared
// job's, so a run dir reads as <job>/round-2/item-3 and nothing else can
// spell it: a kata's job names hold no separator.
const (
	roundSeg = "/round-"
	itemSeg  = "/item-"
)

var errItemCap = errors.New("round produced more items than max_items")

// runRounds runs a fan-out head: round after round, each head instance
// followed by one template instance per item it produced, until a round
// produces none or max_rounds is reached. The head's own record is the loop's
// verdict, so its dependents release on the whole loop, not on one round.
func (s *scheduler) runRounds(
	ctx context.Context, head *kata.Job, log *slog.Logger,
) {
	rec := s.jobs[head.Name]
	began := time.Now()
	rec.StartedAt = stampPtr(began)

	log.Info("fan-out start", "template", head.FanOut.Job,
		"max_rounds", head.FanOut.MaxRounds, "max_items", head.FanOut.MaxItems)

	summary := &fanOutRecord{}
	s.putFanOut(head.Name, summary)

	rec.State = stateDone
	if failure := s.rounds(ctx, head, summary, log); failure != unset {
		rec.State, rec.Failure = stateParked, failure
	}

	rec.FinishedAt = stampPtr(time.Now())

	log.Log(ctx, levelFor[rec.State == stateDone], "fan-out end",
		keyState, rec.State, "rounds", summary.Rounds,
		"capped", summary.Capped, durSince(began))
}

// rounds is the loop itself, returning the first failure or unset.
func (s *scheduler) rounds(
	ctx context.Context, head *kata.Job, summary *fanOutRecord,
	log *slog.Logger,
) string {
	for round := firstRound; round <= head.FanOut.MaxRounds; round++ {
		summary.Rounds = round

		items, failure := s.runHead(ctx, head, round)
		if failure != unset {
			return failure
		}

		summary.Items = append(summary.Items, len(items))

		log.Info("fan-out round", keyRound, round, keyItems, len(items))

		if len(items) == emptyLen {
			return unset // a round that found nothing new ends the loop
		}

		// A cancelled run needs no check here: the next instance the loop
		// starts refuses on the dead context and parks the head.
		if failure := s.runItems(ctx, head, round, items); failure != unset {
			return failure
		}
	}

	summary.Capped = true

	return unset
}

// runHead runs one round's head instance and returns the work items it left
// in its items directory, in name order.
func (s *scheduler) runHead(
	ctx context.Context, head *kata.Job, round int,
) ([]string, string) {
	inst := *head
	inst.Name = head.Name + roundSeg + strconv.Itoa(round)

	itemsDir := filepath.Join(
		s.layout.artifacts(), inst.Name, head.FanOut.Items)
	if err := os.MkdirAll(itemsDir, dirPerm); err != nil {
		return nil, fmt.Sprintf("create items dir: %v", err)
	}

	if failure := s.attemptInstance(ctx, &inst); failure != unset {
		return nil, failure
	}

	entries, err := os.ReadDir(itemsDir)
	if err != nil {
		return nil, fmt.Sprintf("read items dir: %v", err)
	}

	if len(entries) > head.FanOut.MaxItems {
		return nil, fmt.Sprintf("%s: %d > %d",
			errItemCap.Error(), len(entries), head.FanOut.MaxItems)
	}

	items := make([]string, emptyLen, len(entries))
	for _, entry := range entries {
		items = append(items, filepath.Join(itemsDir, entry.Name()))
	}

	return items, unset
}

// runItems runs the template once per item, in parallel, and returns the
// first failure in item order: one parked instance parks the whole fan-out.
func (s *scheduler) runItems(
	ctx context.Context, head *kata.Job, round int, items []string,
) string {
	tmpl := s.kata.Workflow[head.FanOut.Job]
	failures := make([]string, len(items))

	var wg sync.WaitGroup

	for i, item := range items {
		wg.Go(func() {
			inst := *tmpl
			inst.Name = tmpl.Name + roundSeg + strconv.Itoa(round) +
				itemSeg + strconv.Itoa(i+firstRound)
			inst.Item = item

			failures[i] = s.attemptInstance(ctx, &inst)
		})
	}

	wg.Wait()

	for i, failure := range failures {
		if failure != unset {
			return fmt.Sprintf("item %d: %s", i+firstRound, failure)
		}
	}

	return unset
}

// attemptInstance runs one instance the engine derived and records it under
// its own key, so the run log and record.json name every job that ran.
func (s *scheduler) attemptInstance(
	ctx context.Context, inst *kata.Job,
) string {
	if ctx.Err() != nil {
		return fmt.Sprintf("run interrupted: %v", ctx.Err())
	}

	rec := &jobRecord{State: statePending, Item: inst.Item}
	s.putDynamic(inst.Name, rec)
	s.attempt(ctx, inst, rec, s.log.With(keyJob, inst.Name))

	return rec.Failure
}

func (s *scheduler) putDynamic(name string, rec *jobRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.dynamic[name] = rec
}

func (s *scheduler) putFanOut(name string, summary *fanOutRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.fanOuts[name] = summary
}
