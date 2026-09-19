// Command gunkata runs a kata.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/tenequm/gunkata/internal/engine"
)

const (
	exitOK      = 0
	exitFailure = 1
	// exitParked is a run that parked.
	exitParked = 2
)

const (
	runUsage = "usage: gunkata run [--runs-root <dir>] <kata.yml> " +
		"[-p key=value]..."
	// runsRootFallback is used when XDG_STATE_HOME is unset.
	runsRootFallback = ".local/state"
	runsRootSuffix   = "gunkata/runs"
	unset            = ""
	// Argv positions and counts, named because the lint config forbids
	// bare literals.
	first  = 0
	rest   = 1
	empty  = 0
	oneArg = 1
)

// version is overridden at release time with -ldflags.
var version = "0.0.0-dev"

var (
	errParamSyntax   = errors.New("-p wants key=value")
	errParamRepeated = errors.New("-p binds the same param twice")
)

// paramBindings collects the repeatable -p flag. Each occurrence binds one
// declared param to one value.
type paramBindings map[string]string

func (paramBindings) String() string { return unset }

func (b paramBindings) Set(raw string) error {
	key, value, ok := strings.Cut(raw, "=")
	if !ok || key == unset {
		return fmt.Errorf("%w: %q", errParamSyntax, raw)
	}

	if _, dup := b[key]; dup {
		return fmt.Errorf("%w: %q", errParamRepeated, key)
	}

	b[key] = value

	return nil
}

func main() {
	os.Exit(dispatch(os.Args, os.Stdout, os.Stderr))
}

// dispatch wires the argument vector to a subcommand and returns the process
// exit code.
func dispatch(argv []string, out, errOut io.Writer) int {
	args := argv[rest:]
	if len(args) == empty {
		return usage(errOut)
	}

	switch args[first] {
	case engine.PrivateTmpInit:
		return engine.EnterPrivateTmp(args[rest:], errOut)
	case "run":
		return runKata(args[rest:], out, errOut)
	case "version":
		fmt.Fprintf(out, "gunkata %s\n", version)

		return exitOK
	default:
		return usage(errOut)
	}
}

func usage(errOut io.Writer) int {
	fmt.Fprintln(errOut, runUsage)
	fmt.Fprintln(errOut, "usage: gunkata version")

	return exitFailure
}

// runKata runs one kata. Stdout carries the run directory and nothing else;
// progress goes to stderr, and the exit code carries the outcome.
func runKata(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(errOut)
	runsRoot := flags.String("runs-root", defaultRunsRoot(),
		"directory that holds run directories")
	params := paramBindings{}
	flags.Var(params, "p", "bind a declared param: key=value (repeatable)")

	positional, err := parseInterleaved(flags, args)
	if err != nil {
		return exitFailure
	}

	if len(positional) != oneArg {
		fmt.Fprintln(errOut, runUsage)

		return exitFailure
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	res, err := engine.Run(ctx, engine.Options{
		KataPath: positional[first],
		RunsRoot: *runsRoot,
		Params:   params,
		Progress: errOut,
	})

	return report(res, err, out, errOut)
}

// parseInterleaved lets flags follow the kata path, as in
// `gunkata run k.kata.yml -p key=value`; the flag package alone stops at the
// first positional argument.
func parseInterleaved(flags *flag.FlagSet, args []string) ([]string, error) {
	var positional []string

	for {
		if err := flags.Parse(args); err != nil {
			return nil, fmt.Errorf("parse flags: %w", err)
		}

		if flags.NArg() == empty {
			return positional, nil
		}

		positional = append(positional, flags.Arg(first))
		args = flags.Args()[rest:]
	}
}

func report(res engine.Result, err error, out, errOut io.Writer) int {
	if res.RunDir != unset {
		fmt.Fprintln(out, res.RunDir)
	}

	if err != nil {
		fmt.Fprintln(errOut, "gunkata:", err)

		return exitFailure
	}

	if res.Outcome != engine.OutcomeSucceeded {
		return exitParked
	}

	return exitOK
}

// defaultRunsRoot keeps runs in the user's state directory, so a run never
// depends on the directory it was started from.
func defaultRunsRoot() string {
	if state := os.Getenv("XDG_STATE_HOME"); state != unset {
		return filepath.Join(state, runsRootSuffix)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return runsRootSuffix
	}

	return filepath.Join(home, runsRootFallback, runsRootSuffix)
}
