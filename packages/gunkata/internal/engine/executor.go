package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// killGrace is what a process group gets past its own timeout before the
// engine kills it. Tests lower it.
var killGrace = 60 * time.Second

const (
	acpxBin = "acpx"
	// checkTimeout bounds a gate check.
	checkTimeout = 60 * time.Second
	// waitDelay bounds the wait after a kill, so no run hangs on teardown.
	waitDelay = 5 * time.Second
	exitOK    = 0
	// noExit accompanies an error: the process produced no exit code.
	noExit = 0
)

var (
	errACPXMissing = errors.New("acpx not found on PATH")
	errEmptyCheck  = errors.New("check declares no command")
)

// Executor directories under the node's engine-owned HOME.
const (
	tmpDir       = "tmp"
	xdgConfigDir = ".config"
	xdgDataDir   = ".local/share"
	xdgStateDir  = ".local/state"
	xdgCacheDir  = ".cache"
	authDir      = ".gemini/antigravity-acp"
	serverDir    = ".local/lib/antigravity-acp"
)

// inherited is the whitelist that crosses the executor boundary. Skills,
// agent instruction files, MCP config and everything else stay outside.
var inherited = []string{"PATH", "LANG", "LC_ALL", "USER", "LOGNAME"}

// checkInherited is all a check needs: it is a command with an exit code.
var checkInherited = []string{"PATH", "LANG"}

// authFiles are the subscription credentials, the sole inheritance the
// executor contract allows.
var authFiles = []string{"settings.json", "acp_token.json"}

// execSpec is one acpx invocation.
type execSpec struct {
	agent   string
	model   string
	prompt  string
	timeout time.Duration
	home    string
	work    string
	logPath string
}

// runExecutor starts acpx bare - whitelisted environment, engine-owned HOME,
// its own process group - and reports its exit code. A non-zero code is an
// answer, not an error; an error means the executor never ran.
func runExecutor(ctx context.Context, spec execSpec) (int, error) {
	bin, err := exec.LookPath(acpxBin)
	if err != nil {
		return noExit, fmt.Errorf("%w: %w", errACPXMissing, err)
	}

	if prepErr := prepareHome(spec.home, spec.work); prepErr != nil {
		return noExit, prepErr
	}

	logFile, err := os.OpenFile(spec.logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePerm)
	if err != nil {
		return noExit, fmt.Errorf("open executor log: %w", err)
	}
	defer logFile.Close()

	cmdCtx, cancel := context.WithTimeout(ctx, spec.timeout+killGrace)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, bin, acpxArgs(spec)...)
	cmd.Dir = spec.work
	cmd.Env = executorEnv(spec.home)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	return runGrouped(cmd)
}

// runCheck executes a node's declared check from the run directory. The
// exit code is the verdict.
func runCheck(ctx context.Context, argv []string, runDir string) (int, error) {
	if len(argv) == emptyLen {
		return noExit, errEmptyCheck
	}

	cmdCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	// The check is a command the graph declared; running it is the point.
	//nolint:gosec // G204: argv comes from the graph, which is the input
	cmd := exec.CommandContext(cmdCtx, argv[argvHead], argv[argvTail:]...)
	cmd.Dir = runDir
	cmd.Env = env(nil, checkInherited)

	return runGrouped(cmd)
}

// runGrouped runs cmd as the leader of its own process group, so a timeout or
// a cancelled run takes the whole tree down with it (lock 5). An exit code of
// -1 means the engine, or a signal, killed it.
func runGrouped(cmd *exec.Cmd) (int, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = waitDelay
	cmd.Cancel = func() error { return killGroup(cmd.Process) }

	if err := cmd.Start(); err != nil {
		return noExit, fmt.Errorf("start %s: %w", cmd.Path, err)
	}

	waitErr := cmd.Wait()

	// The child is reaped; sweep anything it left behind in the group.
	_ = killGroup(cmd.Process)

	if cmd.ProcessState == nil {
		return noExit, fmt.Errorf("wait for %s: %w", cmd.Path, waitErr)
	}

	return cmd.ProcessState.ExitCode(), nil
}

func killGroup(p *os.Process) error {
	if p == nil {
		return nil
	}

	err := syscall.Kill(-p.Pid, syscall.SIGKILL)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("kill process group %d: %w", p.Pid, err)
	}

	return nil
}

func acpxArgs(spec execSpec) []string {
	seconds := strconv.Itoa(int(spec.timeout.Seconds()))

	return []string{
		"--agent", spec.agent,
		"--cwd", spec.work,
		"--model", spec.model,
		"--timeout", seconds,
		"--approve-all",
		"--format", "quiet",
		"exec", spec.prompt,
	}
}

// executorEnv is the whole environment an executor gets: the whitelist, plus
// the engine-owned home and the paths that hang off it.
func executorEnv(home string) []string {
	own := []string{
		"HOME=" + home,
		"TERM=dumb",
		"TMPDIR=" + filepath.Join(home, tmpDir),
		"XDG_CONFIG_HOME=" + filepath.Join(home, xdgConfigDir),
		"XDG_DATA_HOME=" + filepath.Join(home, xdgDataDir),
		"XDG_STATE_HOME=" + filepath.Join(home, xdgStateDir),
		"XDG_CACHE_HOME=" + filepath.Join(home, xdgCacheDir),
	}

	return env(own, inherited)
}

func env(own, keys []string) []string {
	out := make([]string, len(own), len(own)+len(keys))
	copy(out, own)

	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			out = append(out, key+"="+value)
		}
	}

	return out
}

// prepareHome builds the executor's directories and links in the credentials.
func prepareHome(home, work string) error {
	dirs := []string{
		work,
		filepath.Join(home, tmpDir),
		filepath.Join(home, xdgConfigDir),
		filepath.Join(home, xdgDataDir),
		filepath.Join(home, xdgStateDir),
		filepath.Join(home, xdgCacheDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("create executor dir: %w", err)
		}
	}

	return linkAuth(home)
}

// linkAuth symlinks the real user's ACP credentials into the node's home.
// The real home is resolved from the engine's own environment, which the
// executor's HOME override never touches.
func linkAuth(home string) error {
	realHome, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve real home: %w", err)
	}

	dst := filepath.Join(home, authDir)
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return fmt.Errorf("create auth dir: %w", err)
	}

	src := filepath.Join(realHome, authDir)

	for _, name := range authFiles {
		err := link(filepath.Join(src, name), filepath.Join(dst, name))
		if err != nil {
			return err
		}
	}

	// The agy wrapper resolves its .par through $HOME, so the server
	// binaries ride the same explicit inheritance as the credentials.
	libDir := filepath.Dir(filepath.Join(home, serverDir))
	if err := os.MkdirAll(libDir, dirPerm); err != nil {
		return fmt.Errorf("create server dir: %w", err)
	}

	src = filepath.Join(realHome, serverDir)

	return link(src, filepath.Join(home, serverDir))
}

// link points dst at src, skipping credentials the host does not have.
func link(src, dst string) error {
	if !exists(src) {
		return nil
	}

	if exists(dst) {
		if err := os.Remove(dst); err != nil {
			return fmt.Errorf("replace auth link: %w", err)
		}
	}

	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("link credential: %w", err)
	}

	return nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)

	return err == nil
}
