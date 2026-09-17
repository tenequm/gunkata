package grader

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Fixtures are miniature run directories, one per case.
const fixtureRoot = "../../testdata/grader"

func TestGrade(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dir     string
		variant Variant
		want    []string
	}{
		{"clean pass", "pass-clean", VariantPass, nil},
		{"clean fail, gate parked", "fail-gate-parked", VariantFail, nil},
		{
			"clean fail, produce parked before gate ran",
			"fail-produce-parked", VariantFail, nil,
		},
		{
			"parked node retried",
			"fail-retried", VariantFail,
			[]string{checkRetried},
		},
		{
			"consume started in the fail variant",
			"fail-consume-started", VariantFail,
			[]string{checkPendingRan, checkPendingDir, checkFailConsume},
		},
		{
			"pending consume artifact on disk",
			"fail-consume-artifact-present", VariantFail,
			[]string{checkPendingArtifact, checkFailConsume},
		},
		{
			"parked node claims no attempt",
			"fail-attempts-zero", VariantFail,
			[]string{checkAttempts},
		},
		{
			"state outside the enum",
			"fail-unknown-state", VariantFail,
			[]string{checkStateEnum, checkFailParked},
		},
		{
			"done node whose artifact is absent",
			"pass-artifact-missing", VariantPass,
			[]string{checkDoneArtifact, checkPassArtifacts},
		},
		{
			"outcome inconsistent with node states",
			"pass-outcome-mismatch", VariantPass,
			[]string{checkOutcome, checkPassNodes, checkPassGateExit},
		},
		{
			"node finished before it started",
			"pass-timestamps-reversed", VariantPass,
			[]string{checkTimestamps},
		},
		{
			"done node with a failing check",
			"pass-done-nonzero-exit", VariantPass,
			[]string{checkDoneExit, checkPassGateExit},
		},
		{
			"artifact path outside artifacts/",
			"pass-artifact-escape", VariantPass,
			[]string{checkArtifactPath},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertFindings(t, tt.dir, tt.variant, tt.want)
		})
	}
}

func assertFindings(t *testing.T, dir string, variant Variant, want []string) {
	t.Helper()

	got, err := Grade(filepath.Join(fixtureRoot, dir), variant)
	if err != nil {
		t.Fatalf("Grade(%s) error = %v, want nil", dir, err)
	}

	if got == nil {
		t.Fatalf("Grade(%s) returned a nil slice, want a slice", dir)
	}

	checks := make([]string, len(got))

	for i, finding := range got {
		if finding.Detail == "" {
			t.Errorf("finding %q carries no detail", finding.Check)
		}

		checks[i] = finding.Check
	}

	slices.Sort(checks)

	if !slices.Equal(checks, slices.Sorted(slices.Values(want))) {
		t.Errorf("Grade(%s) findings = %v, want %v (full: %+v)",
			dir, checks, want, got)
	}
}

func TestGradeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dir     string
		variant Variant
		wantErr error
	}{
		{
			"record missing from the run directory",
			"record-missing", VariantPass, os.ErrNotExist,
		},
		{"run directory missing", "no-such-run", VariantFail, os.ErrNotExist},
		{"unknown variant", "pass-clean", Variant("review"), errUnknownVariant},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertError(t, tt.dir, tt.variant, tt.wantErr)
		})
	}
}

func assertError(t *testing.T, dir string, variant Variant, wantErr error) {
	t.Helper()

	got, err := Grade(filepath.Join(fixtureRoot, dir), variant)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Grade(%s) error = %v, want %v", dir, err, wantErr)
	}

	if got != nil {
		t.Errorf("Grade(%s) findings = %v, want nil", dir, got)
	}
}

func TestGradeCorruptRecord(t *testing.T) {
	t.Parallel()

	_, err := Grade(filepath.Join(fixtureRoot, "record-corrupt"), VariantPass)

	if _, ok := errors.AsType[*json.SyntaxError](err); !ok {
		t.Fatalf("Grade() error = %v, want a JSON syntax error", err)
	}
}
