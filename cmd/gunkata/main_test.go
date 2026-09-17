package main

import (
	"strings"
	"testing"
)

func TestRunWritesBuildIdentity(t *testing.T) {
	t.Parallel()

	var out strings.Builder

	if err := run(&out); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	if got := out.String(); !strings.Contains(got, version) {
		t.Errorf("run() wrote %q, want it to contain %q", got, version)
	}
}
