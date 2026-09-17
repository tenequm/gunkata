// Command gunkata runs a work graph and grades a finished run.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/tenequm/gunkata/internal/engine"
	"github.com/tenequm/gunkata/internal/grader"
)

const (
	exitOK      = 0
	exitFailure = 1
	// exitParked is a run that parked. gunkata grade reuses it for a usage
	// or read error, where no verdict was reached either.
	exitParked = 2
	exitUsage  = exitParked
)

const (
	runUsage   = "usage: gunkata run [--runs-root <dir>] <graph.yaml>"
	gradeUsage = "usage: gunkata grade --variant pass|fail <runDir>"
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
	case "run":
		return runGraph(args[rest:], out, errOut)
	case "grade":
		return gradeRun(args[rest:], errOut)
	case "version":
		fmt.Fprintf(out, "gunkata %s\n", version)

		return exitOK
	default:
		return usage(errOut)
	}
}

func usage(errOut io.Writer) int {
	fmt.Fprintln(errOut, runUsage)
	fmt.Fprintln(errOut, gradeUsage)
	fmt.Fprintln(errOut, "usage: gunkata version")

	return exitFailure
}

// runGraph runs one graph. Stdout carries the run directory and nothing else;
// progress goes to stderr, and the exit code carries the outcome.
func runGraph(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(errOut)
	runsRoot := flags.String("runs-root", defaultRunsRoot(),
		"directory that holds run directories")

	if err := flags.Parse(args); err != nil {
		return exitFailure
	}

	if flags.NArg() != oneArg {
		fmt.Fprintln(errOut, runUsage)

		return exitFailure
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	res, err := engine.Run(ctx, engine.Options{
		GraphPath: flags.Arg(first),
		RunsRoot:  *runsRoot,
		Progress:  errOut,
	})

	return report(res, err, out, errOut)
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

func gradeRun(args []string, errOut io.Writer) int {
	flags := flag.NewFlagSet("grade", flag.ContinueOnError)
	flags.SetOutput(errOut)
	variant := flags.String("variant", unset, "corpus variant: pass or fail")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	if flags.NArg() != oneArg || *variant == unset {
		fmt.Fprintln(errOut, gradeUsage)

		return exitUsage
	}

	findings, err := grader.Grade(flags.Arg(first), grader.Variant(*variant))
	if err != nil {
		fmt.Fprintln(errOut, "gunkata:", err)

		return exitUsage
	}

	for _, finding := range findings {
		fmt.Fprintf(errOut, "%s: %s\n", finding.Check, finding.Detail)
	}

	if len(findings) != empty {
		return exitFailure
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
