package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// killGrace is what a process group gets past its own timeout before the
// engine kills it. Tests lower it.
var killGrace = 60 * time.Second

// stepTimeout bounds one pre- or post-step. Tests lower it.
var stepTimeout = 10 * time.Minute

const (
	acpxBin       = "acpx"
	harnessClaude = "claude"
	// waitDelay bounds the wait after a kill, so no run hangs on teardown.
	waitDelay = 5 * time.Second
	exitOK    = 0
	// noExit accompanies an error: the process produced no exit code.
	noExit = 0
	// mcpStdin is where acpx reads the MCP config from, so the expanded
	// config never touches disk.
	mcpStdin = "/dev/stdin"
	mcpType  = "http"
	// domainLabels is a registrable domain's label count: name plus TLD.
	domainLabels = 2
	firstLabel   = 0
	varGroup     = 1
	allMatches   = -1
	firstDup     = 2
	nameDupFmt   = "%s-%d"
)

var (
	errACPXMissing = errors.New("acpx not found on PATH")
	errMCPVar      = errors.New("MCP URL references an unset variable")
)

// mcpVar matches a ${VAR} placeholder in an MCP URL.
var mcpVar = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Executor directories under the job's engine-owned HOME.
const (
	tmpDir       = "tmp"
	xdgConfigDir = ".config"
	xdgDataDir   = ".local/share"
	xdgStateDir  = ".local/state"
	xdgCacheDir  = ".cache"
)

// inherited is the whitelist that crosses the executor boundary. Skills,
// agent instruction files, MCP config and everything else stay outside.
var inherited = []string{"PATH", "LANG", "LC_ALL", "USER", "LOGNAME"}

// harnessAuth is the subscription credentials one harness inherits, the sole
// inheritance the executor contract allows, as paths that hold the same place
// under the real home and under the executor's.
var harnessAuth = map[string][]string{
	harnessClaude: {".claude/.credentials.json"},
	"codex":       {".codex/auth.json"},
	// pi reads its provider list, and the key with it, from this one file.
	"pi": {".pi/agent/models.json"},
}

// harnessEnv is what a harness needs set to start bare. Claude Code would
// otherwise load the claude.ai connectors tied to the subscription login,
// ancestor CLAUDE.md files, its bundled skills and auto memory; acpx would
// skip the user scope, where the job's skills are copied.
var harnessEnv = map[string][]string{
	harnessClaude: {
		"ACPX_CLAUDE_INCLUDE_USER_SETTINGS=1",
		"ENABLE_CLAUDEAI_MCP_SERVERS=false",
		"CLAUDE_CODE_DISABLE_CLAUDE_MDS=1",
		"CLAUDE_CODE_DISABLE_BUNDLED_SKILLS=1",
		"CLAUDE_CODE_DISABLE_AUTO_MEMORY=1",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
		"DISABLE_AUTOUPDATER=1",
	},
}

// githubAuth is the host's own gh and git access, inherited by every harness:
// a gh login on a workstation, or a credential-injecting gateway (.onecli)
// on a sandbox. A path the host lacks is skipped.
var githubAuth = []string{".config/gh", ".config/git", ".onecli"}

// execSpec is one acpx invocation.
type execSpec struct {
	harness string
	model   string
	prompt  string
	options map[string]string
	skills  []string // snapshot dirs
	mcps    []string // URLs, ${VAR} unexpanded
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

	if prepErr := prepareHome(spec); prepErr != nil {
		return noExit, prepErr
	}

	mcpConfig, err := mcpConfigJSON(spec.mcps)
	if err != nil {
		return noExit, err
	}

	logFile, err := openLog(spec.logPath)
	if err != nil {
		return noExit, err
	}
	defer logFile.Close()

	cmdCtx, cancel := context.WithTimeout(ctx, spec.timeout+killGrace)
	defer cancel()

	var mcpFlags []string
	if mcpConfig != nil {
		mcpFlags = []string{"--mcp-config", mcpStdin}
	}

	cmd := exec.CommandContext(cmdCtx, bin, acpxArgs(spec, mcpFlags)...)
	cmd.Dir = spec.work
	cmd.Env = append(executorEnv(spec.home), harnessEnv[spec.harness]...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if mcpConfig != nil {
		cmd.Stdin = bytes.NewReader(mcpConfig)
	}

	return runGrouped(cmd)
}

func openLog(path string) (*os.File, error) {
	const flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND

	file, err := os.OpenFile(path, flags, filePerm)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}

	return file, nil
}

// runStep executes one declared step engine-side: full engine environment,
// cwd the job's working directory, output appended to log. The exit code is
// the verdict.
func runStep(
	ctx context.Context, argv []string, work string, log io.Writer,
) (int, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, stepTimeout)
	defer cancel()

	fmt.Fprintf(log, "$ %s\n", strings.Join(argv, " "))

	// The step is a command the kata declared; running it is the point.
	//nolint:gosec // G204: argv comes from the kata, which is the input
	cmd := exec.CommandContext(cmdCtx, argv[argvHead], argv[argvTail:]...)
	cmd.Dir = work
	cmd.Stdout = log
	cmd.Stderr = log

	return runGrouped(cmd)
}

// runGrouped runs cmd as the leader of its own process group, so a timeout or
// a cancelled run takes the whole tree down with it. An exit code of -1 means
// the engine, or a signal, killed it.
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

// acpxArgs builds the invocation: global flags, the harness as acpx's
// positional agent, then exec with its config options and the prompt.
func acpxArgs(spec execSpec, mcpFlags []string) []string {
	args := []string{
		"--cwd", spec.work,
		"--model", spec.model,
		"--timeout", strconv.Itoa(int(spec.timeout.Seconds())),
		"--approve-all",
		"--format", "quiet",
	}

	args = append(args, mcpFlags...)
	args = append(args, spec.harness, "exec")

	for _, key := range slices.Sorted(maps.Keys(spec.options)) {
		args = append(args,
			"--config-option", key+"="+spec.options[key])
	}

	return append(args, spec.prompt)
}

// mcpServer is one entry of acpx's mcpServers array.
type mcpServer struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	URL     string `json:"url"`
	Headers []any  `json:"headers"`
}

// mcpConfigJSON expands the MCP URLs from the engine's environment into the
// config acpx reads on stdin; nil when the job declares none. Each server is
// named after its host's registrable label: glim for glim.sh, deepwiki for
// mcp.deepwiki.com.
func mcpConfigJSON(urls []string) ([]byte, error) {
	if len(urls) == emptyLen {
		return nil, nil
	}

	servers := make([]mcpServer, emptyLen, len(urls))
	seen := map[string]int{}

	for _, raw := range urls {
		expanded, err := expandMCP(raw)
		if err != nil {
			return nil, err
		}

		name := mcpName(raw)
		seen[name]++

		if seen[name] >= firstDup {
			name = fmt.Sprintf(nameDupFmt, name, seen[name])
		}

		servers = append(servers, mcpServer{
			Name: name, Type: mcpType, URL: expanded, Headers: []any{},
		})
	}

	raw, err := json.Marshal(map[string][]mcpServer{"mcpServers": servers})
	if err != nil {
		return nil, fmt.Errorf("encode MCP config: %w", err)
	}

	return raw, nil
}

func mcpName(raw string) string {
	u, err := url.Parse(mcpVar.ReplaceAllString(raw, unset))
	if err != nil || u.Hostname() == unset {
		return "mcp"
	}

	labels := strings.Split(u.Hostname(), ".")

	return labels[max(len(labels)-domainLabels, firstLabel)]
}

// unsetVar names the first ${VAR} in raw the engine's environment lacks, or
// is unset when there is none.
func unsetVar(raw string) string {
	for _, groups := range mcpVar.FindAllStringSubmatch(raw, allMatches) {
		if _, ok := os.LookupEnv(groups[varGroup]); !ok {
			return groups[varGroup]
		}
	}

	return unset
}

// expandMCP substitutes each ${VAR} from the engine's environment.
func expandMCP(raw string) (string, error) {
	var missing error

	out := mcpVar.ReplaceAllStringFunc(raw, func(match string) string {
		name := mcpVar.FindStringSubmatch(match)[varGroup]

		value, ok := os.LookupEnv(name)
		if !ok {
			missing = fmt.Errorf("%w: %s", errMCPVar, name)
		}

		return value
	})

	return out, missing
}

// executorEnv is the whole environment an executor gets: the whitelist, plus
// the engine-owned home and the paths that hang off it.
func executorEnv(home string) []string {
	env := []string{
		"HOME=" + home,
		"TERM=dumb",
		"TMPDIR=" + filepath.Join(home, tmpDir),
		"XDG_CONFIG_HOME=" + filepath.Join(home, xdgConfigDir),
		"XDG_DATA_HOME=" + filepath.Join(home, xdgDataDir),
		"XDG_STATE_HOME=" + filepath.Join(home, xdgStateDir),
		"XDG_CACHE_HOME=" + filepath.Join(home, xdgCacheDir),
	}

	for _, key := range inherited {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}

	return env
}

// prepareHome builds the executor's directories, links in the credentials
// its harness needs and the host's GitHub access, and copies in its skills.
func prepareHome(spec execSpec) error {
	dirs := []string{tmpDir, xdgConfigDir, xdgDataDir, xdgStateDir, xdgCacheDir}
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(spec.home, dir), dirPerm)
		if err != nil {
			return fmt.Errorf("create executor dir: %w", err)
		}
	}

	realHome, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve real home: %w", err)
	}

	for _, rel := range slices.Concat(harnessAuth[spec.harness], githubAuth) {
		err := link(filepath.Join(realHome, rel), filepath.Join(spec.home, rel))
		if err != nil {
			return err
		}
	}

	return materializeSkills(spec.home, spec.harness, spec.skills)
}

// link points dst at src, skipping credentials the host does not have.
func link(src, dst string) error {
	if _, err := os.Lstat(src); err != nil {
		return nil //nolint:nilerr // a credential the host lacks is skipped
	}

	if err := os.MkdirAll(filepath.Dir(dst), dirPerm); err != nil {
		return fmt.Errorf("create credential dir: %w", err)
	}

	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("link credential: %w", err)
	}

	return nil
}
