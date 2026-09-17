package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// cmdName is argv[0] as the shell would pass it.
const cmdName = "gunkata"

func TestDispatchVersion(t *testing.T) {
	t.Parallel()

	var out, errOut strings.Builder

	code := dispatch([]string{cmdName, "version"}, &out, &errOut)
	if code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}

	if !strings.Contains(out.String(), version) {
		t.Errorf("stdout = %q, want it to contain %q", out.String(), version)
	}
}

func TestDispatchUsage(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"no arguments":            {cmdName},
		"unknown command":         {cmdName, "orchestrate"},
		"run without a graph":     {cmdName, "run"},
		"grade without a variant": {cmdName, "grade", "/tmp/run"},
		"grade without a run dir": {cmdName, "grade", "--variant", "pass"},
	}

	for name, argv := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertUsage(t, argv)
		})
	}
}

// assertUsage insists a misuse costs the caller nothing but a usage line on
// stderr: stdout stays clean, because a script reads the run dir from it.
func assertUsage(t *testing.T, argv []string) {
	t.Helper()

	var out, errOut strings.Builder

	if code := dispatch(argv, &out, &errOut); code == exitOK {
		t.Errorf("exit code = %d, want a failure", code)
	}

	if out.Len() != empty {
		t.Errorf("stdout = %q, want nothing on stdout", out.String())
	}

	if !strings.Contains(errOut.String(), "usage:") {
		t.Errorf("stderr = %q, want usage", errOut.String())
	}
}

func TestDefaultRunsRoot(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	if got, want := defaultRunsRoot(),
		filepath.Join(state, runsRootSuffix); got != want {
		t.Errorf("defaultRunsRoot() = %q, want %q", got, want)
	}

	t.Setenv("XDG_STATE_HOME", unset)

	if got := defaultRunsRoot(); !strings.HasSuffix(got, runsRootSuffix) {
		t.Errorf("defaultRunsRoot() = %q, want it to end in %q",
			got, runsRootSuffix)
	}
}
