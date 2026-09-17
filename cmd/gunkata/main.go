// Command gunkata is the entry point for the gunkata DAG orchestrator.
package main

import (
	"fmt"
	"io"
	"os"
)

const exitFailure = 1

// version is overridden at release time with -ldflags.
var version = "0.0.0-dev"

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "gunkata:", err)
		os.Exit(exitFailure)
	}
}

// run reports the build identity. The engine is still in design; see docs/.
func run(out io.Writer) error {
	if _, err := fmt.Fprintf(out, "gunkata %s\n", version); err != nil {
		return fmt.Errorf("write banner: %w", err)
	}

	return nil
}
