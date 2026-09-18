package main

import (
	"flag"
	"maps"
	"path/filepath"
	"slices"
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
		"no arguments":       {cmdName},
		"unknown command":    {cmdName, "orchestrate"},
		"run without a kata": {cmdName, "run"},
		"run with two katas": {cmdName, "run", "a.kata.yml", "b.kata.yml"},
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

func TestParamBindings(t *testing.T) {
	t.Parallel()

	bindings := paramBindings{}
	if err := bindings.Set("pr=https://github.com/o/r/pull/1"); err != nil {
		t.Fatalf("Set() returned error: %v", err)
	}

	if err := bindings.Set("expr=a=b"); err != nil {
		t.Fatalf("Set() returned error: %v", err)
	}

	if err := bindings.Set("blank="); err != nil {
		t.Fatalf("Set() returned error: %v", err)
	}

	want := paramBindings{"pr": "https://github.com/o/r/pull/1", "expr": "a=b", "blank": ""}
	if !maps.Equal(bindings, want) {
		t.Errorf("bindings = %v, want %v", bindings, want)
	}

	if err := bindings.Set("pr=again"); err == nil {
		t.Error("Set() accepted the same param twice, want an error")
	}

	for _, raw := range []string{"pr", "=value"} {
		if err := (paramBindings{}).Set(raw); err == nil {
			t.Errorf("Set(%q) accepted it, want an error", raw)
		}
	}
}

// TestParseInterleaved pins the spec's invocation shape: params may follow
// the kata path.
func TestParseInterleaved(t *testing.T) {
	t.Parallel()

	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	root := flags.String("runs-root", unset, unset)
	params := paramBindings{}
	flags.Var(params, "p", unset)

	positional, err := parseInterleaved(flags,
		[]string{"--runs-root", "/r", "k.kata.yml", "-p", "a=1", "-p", "b=2"})
	if err != nil {
		t.Fatalf("parseInterleaved() returned error: %v", err)
	}

	if !slices.Equal(positional, []string{"k.kata.yml"}) {
		t.Errorf("positional = %v, want [k.kata.yml]", positional)
	}

	if *root != "/r" || !maps.Equal(params, paramBindings{"a": "1", "b": "2"}) {
		t.Errorf("runs-root = %q, params = %v", *root, params)
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
