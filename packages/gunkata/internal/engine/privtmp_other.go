//go:build !linux

package engine

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// initFailed is the helper's exit code when it could not reach acpx.
const initFailed = 125

// errNoPrivateTmp is why executors share the host's /tmp here: a private
// one takes Linux user namespaces.
var errNoPrivateTmp = errors.New(
	"cannot give executors a private /tmp - that needs Linux")

func probePrivateTmp() error { return errNoPrivateTmp }

func confinePrivateTmp(*exec.Cmd, string) error { return errNoPrivateTmp }

// EnterPrivateTmp is never reached: the probe fails first.
func EnterPrivateTmp(_ []string, errOut io.Writer) int {
	fmt.Fprintln(errOut, errNoPrivateTmp)

	return initFailed
}
