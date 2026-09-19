package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

// Kernel constants the syscall package does not name.
const (
	capSysAdmin          = 21 // CAP_SYS_ADMIN, which mount needs
	prCapAmbient         = 47 // PR_CAP_AMBIENT
	prCapAmbientClearAll = 4  // PR_CAP_AMBIENT_CLEAR_ALL
	prUnused             = 0
	errnoNone            = 0
	// initFailed is the helper's exit code when it could not reach acpx.
	initFailed = 125
	// probeArgs is a helper invocation that mounts and execs nothing.
	probeArgs = 1
	argTmp    = 0
	argBin    = 1
	idSpan    = 1
)

var errNoPrivateTmp = errors.New("cannot give executors a private " +
	sharedTmp + " - this host refuses unprivileged user namespaces")

// namespaceAttr puts the helper in a user and mount namespace of its own,
// as the same uid and gid, holding CAP_SYS_ADMIN across its exec only so it
// can mount. There is no PID namespace: the engine still kills the executor
// by process group, as for any other command.
func namespaceAttr() *syscall.SysProcAttr {
	uid, gid := os.Getuid(), os.Getgid()

	return &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: uid, HostID: uid, Size: idSpan},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: gid, HostID: gid, Size: idSpan},
		},
		AmbientCaps: []uintptr{capSysAdmin},
	}
}

// confinePrivateTmp rewrites cmd to start through the engine's own binary,
// which mounts tmp over /tmp in a fresh namespace and then becomes the
// original command in place, keeping its pid.
func confinePrivateTmp(cmd *exec.Cmd, tmp string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve gunkata binary: %w", err)
	}

	cmd.Args = append([]string{self, PrivateTmpInit, tmp, cmd.Path},
		cmd.Args[argBin:]...)
	cmd.Path = self
	cmd.SysProcAttr = namespaceAttr()

	return nil
}

// probePrivateTmp proves the host lets the helper mount a private /tmp.
func probePrivateTmp() error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve gunkata binary: %w", err)
	}

	// Mount /tmp over itself and exec nothing.
	cmd := exec.CommandContext(context.Background(),
		self, PrivateTmpInit, sharedTmp)
	cmd.SysProcAttr = namespaceAttr()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %w: %s", errNoPrivateTmp, err,
			strings.TrimSpace(stderr.String()))
	}

	return nil
}

// EnterPrivateTmp is the helper side of confinePrivateTmp, run by the
// gunkata binary as its first act. args is the private tmp dir and the
// command to become; with no command it only proves the mount works.
func EnterPrivateTmp(args []string, errOut io.Writer) int {
	if len(args) < probeArgs {
		fmt.Fprintln(errOut, "private tmp: no directory given")

		return initFailed
	}

	err := syscall.Mount(args[argTmp], sharedTmp, "", syscall.MS_BIND, "")
	if err != nil {
		fmt.Fprintf(errOut, "private tmp: bind %s: %v\n", sharedTmp, err)

		return initFailed
	}

	if len(args) == probeArgs {
		return exitOK
	}

	fmt.Fprintf(errOut, "private tmp: %v\n", becomeCommand(args[argBin:]))

	return initFailed
}

// becomeCommand execs argv without the capability the mount needed. Ambient
// capabilities are per thread, so the drop and the exec share one; it
// returns only when the exec fails.
func becomeCommand(argv []string) error {
	runtime.LockOSThread()

	_, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL,
		prCapAmbient, prCapAmbientClearAll, prUnused)
	if errno != errnoNone {
		return fmt.Errorf("drop capabilities: %w", errno)
	}

	//nolint:gosec // G204: argv is acpx and its args, built by the engine
	err := syscall.Exec(argv[argTmp], argv, os.Environ())

	return fmt.Errorf("exec %s: %w", argv[argTmp], err)
}
