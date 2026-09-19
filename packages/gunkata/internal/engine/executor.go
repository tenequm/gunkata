package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// killGrace is what a process group gets past its own timeout before the
// engine kills it. Tests lower it.
var killGrace = 60 * time.Second

// stepTimeout bounds one pre- or post-step. Tests lower it.
var stepTimeout = 10 * time.Minute

// PrivateTmpInit is the hidden gunkata subcommand an executor starts
// through, to get its own /tmp before it becomes acpx.
const PrivateTmpInit = "__private-tmp"

const (
	acpxBin       = "acpx"
	harnessClaude = "claude"
	harnessAgy    = "agy"
	harnessCodex  = "codex"
	// claudeLogin is Claude Code's subscription login under a HOME.
	claudeLogin = ".claude/.credentials.json"
	// claudeTranscripts matches Claude Code's main session transcripts under
	// a HOME; subagents' sit deeper.
	claudeTranscripts = ".claude/projects/*/*.jsonl"
	claudeCodeKey     = "claude-code"
	// loginPoll is how often a claude executor's login link is checked for
	// a refresh that replaced it.
	loginPoll = time.Second
	// relinkSuffix names the link staged to restore a replaced one.
	relinkSuffix = ".relink"
	noExpiry     = 0
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
	errHostLogin   = errors.New("host claude login is not a JSON object")
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

// npmCacheDir is the engine-owned npm cache under the host's cache dir.
// acpx 0.17 runs the claude and codex adapters through npm exec, which
// would otherwise download and install hundreds of MB into each bare HOME.
// It holds package code, never config, and is no new write capability: an
// executor runs as the engine's uid on an unsandboxed filesystem.
const npmCacheDir = "gunkata/npm"

// inherited is the whitelist that crosses the executor boundary. Skills,
// agent instruction files, MCP config and everything else stay outside. On
// NixOS the last one tells a shell its environment is already set up;
// without it a harness whose shell is the login shell, such as Codex's zsh,
// rebuilds PATH from the executor's HOME and loses the inherited one.
var inherited = []string{
	"PATH", "LANG", "LC_ALL", "USER", "LOGNAME", "__NIXOS_SET_ENVIRONMENT_DONE",
}

// harnessAuth is the subscription credentials one harness inherits, the sole
// inheritance the executor contract allows, as paths that hold the same place
// under the real home and under the executor's.
var harnessAuth = map[string][]string{
	harnessClaude: {claudeLogin},
	harnessCodex:  {".codex/auth.json"},
	// pi reads its provider list, and the key with it, from this one file.
	"pi": {".pi/agent/models.json"},
	// settings.json selects the Google-account login; the lib dir is the ACP
	// server itself, which the host's wrapper finds under HOME.
	harnessAgy: {
		".gemini/antigravity-acp/settings.json",
		".gemini/antigravity-acp/acp_token.json",
		".local/lib/antigravity-acp",
	},
}

// harnessAgent is the acpx agent argument for a harness acpx has no built-in
// agent for; any other harness is acpx's positional agent name. acpx 0.17
// lacks Antigravity, so agy runs the host's wrapper around its ACP server.
var harnessAgent = map[string][]string{
	harnessAgy: {agentFlag, "agy-acp-server"},
}

// agentFlag names acpx's custom agent command; npxRun is how that command
// runs a pinned adapter package, from the executor's shared npm cache.
const (
	agentFlag = "--agent"
	npxRun    = "npx -y "
)

// harnessEnv is what a harness needs set to start bare. Claude Code would
// otherwise load the claude.ai connectors tied to the subscription login,
// ancestor CLAUDE.md files, its bundled skills and auto memory; acpx would
// skip the user scope, where the job's skills are copied. The Codex adapter
// would otherwise start in its sandbox with the network off, which cuts the
// inherited GitHub access, and send actions to its reviewer model; a kata
// picks another mode with options: {mode: ...}.
var harnessEnv = map[string][]string{
	harnessCodex: {"INITIAL_AGENT_MODE=agent-full-access"},
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

// harnessFiles is config a harness needs written into its HOME to start bare,
// keyed by path under that HOME. It never holds secrets.
var harnessFiles = map[string]map[string]string{
	harnessCodex: {".codex/config.toml": codexConfig},
}

// codexConfig keeps Codex bare. Without it Codex would load the connectors
// and plugins tied to the subscription login, install its bundled system
// skills, read a cwd AGENTS.md, walk from the cwd up to the nearest .git for
// .agents/skills - out of the job's HOME whenever the runs root sits in a
// repository - and look for MCP OAuth tokens in the host's keyring. The zero
// grace makes Codex wait for the declared MCP servers, which it would
// otherwise leave out of the model's tools.
const codexConfig = `project_doc_max_bytes = 0
project_root_markers = []
mcp_optional_startup_grace_ms = 0
mcp_oauth_credentials_store = "file"

[features]
apps = false
plugins = false
remote_plugin = false

[skills.bundled]
enabled = false
`

// githubAuth is the host's own gh and git access, inherited by every harness:
// a gh login on a workstation, or a credential-injecting gateway (.onecli)
// on a sandbox. A path the host lacks is skipped.
var githubAuth = []string{".config/gh", ".config/git", ".onecli"}

// execSpec is one acpx invocation.
type execSpec struct {
	harness string
	adapter string // npm package spec; unset keeps acpx's built-in
	model   string
	prompt  string
	options map[string]string
	skills  []string // snapshot dirs
	mcps    []string // URLs, ${VAR} unexpanded
	timeout time.Duration
	home    string
	work    string
	jobDir  string // holds the executor's stream and stderr
	// privateTmp mounts home/tmp over the executor's /tmp.
	privateTmp bool
	npmCache   string
	log        *slog.Logger
}

// runExecutor starts acpx bare - whitelisted environment, engine-owned HOME,
// its own process group - and reports its exit code and the versions it ran.
// A non-zero code is an answer, not an error; an error means the executor
// never ran. Its stdout, the ACP event stream, is kept verbatim and
// summarised into the log as it arrives.
func runExecutor(
	ctx context.Context, spec execSpec,
) (int, map[string]string, error) {
	bin, err := exec.LookPath(acpxBin)
	if err != nil {
		return noExit, nil, fmt.Errorf("%w: %w", errACPXMissing, err)
	}

	defer dropCredentials(spec)

	if prepErr := prepareHome(spec); prepErr != nil {
		return noExit, nil, prepErr
	}

	stopLoginWatch := watchClaudeLogin(spec)
	defer stopLoginWatch()

	spec.npmCache, err = sharedNPMCache()
	if err != nil {
		return noExit, nil, err
	}

	mcpConfig, err := mcpConfigJSON(spec.mcps)
	if err != nil {
		return noExit, nil, err
	}

	feed, err := openLog(filepath.Join(spec.jobDir, executorFeed))
	if err != nil {
		return noExit, nil, err
	}
	defer feed.Close()

	stderr, err := openLog(filepath.Join(spec.jobDir, executorLog))
	if err != nil {
		return noExit, nil, err
	}
	defer stderr.Close()

	cmdCtx, cancel := context.WithTimeout(ctx, spec.timeout+killGrace)
	defer cancel()

	events := &eventStream{
		log: spec.log, tools: map[string]*toolCall{},
		versions: map[string]string{},
	}
	cmd := executorCmd(cmdCtx, bin, spec, mcpConfig)

	if confineErr := maybeConfine(cmd, spec); confineErr != nil {
		return noExit, nil, confineErr
	}

	// The file comes first: a line is on disk before it is parsed.
	cmd.Stdout = io.MultiWriter(feed, events)
	cmd.Stderr = stderr

	code, err := superviseExecutor(cmdCtx, cmd, spec, events)

	return code, executorVersions(spec, events.versions), err
}

// executorVersions is what the executor ran: acpx and its ACP agent as the
// stream's handshake names them, plus Claude Code for a claude executor,
// whose adapter does not say which one it bundles.
func executorVersions(
	spec execSpec, versions map[string]string,
) map[string]string {
	if spec.harness == harnessClaude {
		if v := claudeCodeVersion(spec.home); v != unset {
			versions[claudeCodeKey] = v
		}
	}

	if len(versions) == emptyLen {
		return nil
	}

	return versions
}

// claudeCodeVersion is the version Claude Code stamps on its session
// transcripts under home; unset when there is none.
func claudeCodeVersion(home string) string {
	paths, err := filepath.Glob(filepath.Join(home, claudeTranscripts))
	if err != nil {
		return unset
	}

	for _, path := range paths {
		if v := transcriptVersion(path); v != unset {
			return v
		}
	}

	return unset
}

// transcriptVersion is the first version a transcript's entries carry. The
// decoder streams, so an entry of any size is read whole.
func transcriptVersion(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return unset
	}
	defer file.Close()

	dec := json.NewDecoder(file)

	for {
		var entry struct {
			Version string `json:"version"`
		}

		if dec.Decode(&entry) != nil {
			return unset
		}

		if entry.Version != unset {
			return entry.Version
		}
	}
}

// maybeConfine gives the executor its own /tmp when the run has one.
func maybeConfine(cmd *exec.Cmd, spec execSpec) error {
	if !spec.privateTmp {
		return nil
	}

	return confinePrivateTmp(cmd, filepath.Join(spec.home, tmpDir))
}

func executorCmd(
	ctx context.Context, bin string, spec execSpec, mcpConfig []byte,
) *exec.Cmd {
	var mcpFlags []string
	if mcpConfig != nil {
		mcpFlags = []string{"--mcp-config", mcpStdin}
	}

	//nolint:gosec // G204: bin is acpx on PATH; the args are the kata's
	cmd := exec.CommandContext(ctx, bin, acpxArgs(spec, mcpFlags)...)
	cmd.Dir = spec.work
	cmd.Env = append(executorEnv(spec.home, spec.npmCache),
		harnessEnv[spec.harness]...)

	if mcpConfig != nil {
		cmd.Stdin = bytes.NewReader(mcpConfig)
	}

	return cmd
}

// superviseExecutor runs the executor to its end and logs its lifecycle. The
// prompt is logged by length and MCP servers by name, since the text and the
// expanded URLs may carry secrets.
func superviseExecutor(
	ctx context.Context, cmd *exec.Cmd, spec execSpec, events *eventStream,
) (int, error) {
	began := time.Now()

	if err := startGrouped(cmd); err != nil {
		return noExit, err
	}

	spec.log.Info("executor start", "harness", spec.harness,
		"model", spec.model, "pid", cmd.Process.Pid,
		"timeout_s", int(spec.timeout.Seconds()),
		"mcps", mapped(spec.mcps, mcpName),
		"skills", mapped(spec.skills, filepath.Base),
		"prompt_len", len(spec.prompt))

	code, err := waitGrouped(cmd)

	events.flush()

	if ctx.Err() != nil {
		spec.log.Warn("executor killed", "reason", ctx.Err().Error())
	}

	spec.log.Log(ctx, levelFor[err == nil && code == exitOK],
		"executor exit", keyExit, code, durSince(began))

	return code, err
}

func mapped(in []string, fn func(string) string) []string {
	out := make([]string, emptyLen, len(in))
	for _, s := range in {
		out = append(out, fn(s))
	}

	return out
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
	if err := startGrouped(cmd); err != nil {
		return noExit, err
	}

	return waitGrouped(cmd)
}

func startGrouped(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.Setpgid = true
	cmd.WaitDelay = waitDelay
	cmd.Cancel = func() error { return killGroup(cmd.Process) }

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", cmd.Path, err)
	}

	return nil
}

func waitGrouped(cmd *exec.Cmd) (int, error) {
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

// acpxArgs builds the invocation: global flags, the harness's acpx agent,
// then exec with its config options and the prompt.
func acpxArgs(spec execSpec, mcpFlags []string) []string {
	args := []string{
		"--cwd", spec.work,
		"--model", spec.model,
		"--timeout", strconv.Itoa(int(spec.timeout.Seconds())),
		"--approve-all",
		"--format", "json", "--json-strict",
	}

	args = append(args, mcpFlags...)
	args = append(args, acpxAgent(spec)...)
	args = append(args, "exec")

	for _, key := range slices.Sorted(maps.Keys(spec.options)) {
		args = append(args,
			"--config-option", key+"="+spec.options[key])
	}

	return append(args, spec.prompt)
}

// acpxAgent is the acpx agent argument: a pinned adapter run through npx,
// the harness's own agent command, or acpx's built-in agent by name.
func acpxAgent(spec execSpec) []string {
	if spec.adapter != unset {
		return []string{agentFlag, npxRun + spec.adapter}
	}

	if agent, ok := harnessAgent[spec.harness]; ok {
		return agent
	}

	return []string{spec.harness}
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
// the engine-owned home and the paths that hang off it, and the shared npm
// cache.
func executorEnv(home, npmCache string) []string {
	env := []string{
		"HOME=" + home,
		"npm_config_cache=" + npmCache,
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

// sharedNPMCache creates the npm cache every executor shares. npm keeps it
// safe for concurrent use: content-addressed writes, and a lock around each
// npm exec install.
func sharedNPMCache() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return unset, fmt.Errorf("resolve cache dir: %w", err)
	}

	dir := filepath.Join(cache, npmCacheDir)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return unset, fmt.Errorf("create npm cache: %w", err)
	}

	return dir, nil
}

// prepareHome builds the executor's directories, links in the credentials
// its harness needs and the host's GitHub access, writes the harness's own
// config, and copies in its skills.
func prepareHome(spec execSpec) error {
	if err := makeExecutorDirs(spec.home); err != nil {
		return err
	}

	realHome, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve real home: %w", err)
	}

	for _, rel := range credentialPaths(spec.harness) {
		err := link(filepath.Join(realHome, rel), filepath.Join(spec.home, rel))
		if err != nil {
			return err
		}
	}

	if err := writeHarnessFiles(spec.home, spec.harness); err != nil {
		return err
	}

	return materializeSkills(spec.home, spec.harness, spec.skills)
}

func makeExecutorDirs(home string) error {
	dirs := []string{tmpDir, xdgConfigDir, xdgDataDir, xdgStateDir, xdgCacheDir}
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(home, dir), dirPerm)
		if err != nil {
			return fmt.Errorf("create executor dir: %w", err)
		}
	}

	return nil
}

func writeHarnessFiles(home, harness string) error {
	for rel, body := range harnessFiles[harness] {
		path := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
			return fmt.Errorf("create harness config dir: %w", err)
		}

		if err := os.WriteFile(path, []byte(body), filePerm); err != nil {
			return fmt.Errorf("write harness config: %w", err)
		}
	}

	return nil
}

// credentialPaths is everything prepareHome links in for a harness.
func credentialPaths(harness string) []string {
	return slices.Concat(harnessAuth[harness], githubAuth)
}

// dropCredentials removes, once the executor is gone, every credential it was
// given and any copy it wrote in a link's place: agy refreshes its token by
// replacing the link with a regular file, via a temp file beside it. Nothing
// secret may outlive the job in the run dir.
func dropCredentials(spec execSpec) {
	for _, rel := range credentialPaths(spec.harness) {
		if err := dropCredential(filepath.Join(spec.home, rel)); err != nil {
			spec.log.Error("credential left in the run dir", keyErr, err)
		}
	}
}

// watchClaudeLogin hands every token a claude executor refreshes back to the
// host while it runs, and once more when the returned func stops it. Claude
// Code writes its login by temp file and rename, so a refresh replaces the
// link with a file whose refresh token has rotated: the host's is spent, and
// dropping that file would log the host out.
func watchClaudeLogin(spec execSpec) func() {
	realHome, err := os.UserHomeDir()
	if spec.harness != harnessClaude || err != nil {
		return func() {}
	}

	host := filepath.Join(realHome, claudeLogin)
	job := filepath.Join(spec.home, claudeLogin)
	done := make(chan struct{})

	var watch sync.WaitGroup

	watch.Go(func() { pollLogin(host, job, spec.log, done) })

	return func() {
		close(done)
		watch.Wait()
		returnLogin(host, job, spec.log)
	}
}

func pollLogin(host, job string, log *slog.Logger, done <-chan struct{}) {
	ticker := time.NewTicker(loginPoll)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			returnLogin(host, job, log)
		}
	}
}

func returnLogin(host, job string, log *slog.Logger) {
	if err := syncLogin(host, job); err != nil {
		log.Error("claude login not returned to the host", keyErr, err)
	}
}

// syncLogin, once the executor has replaced its link to host with a file,
// writes that file over the host's login if its token expires later - a
// login Claude Code cleared as dead expires at 0, so it never does - then
// restores the link, so the executor reads what the host holds from then on.
func syncLogin(host, job string) error {
	info, err := os.Lstat(job)
	if err != nil || !info.Mode().IsRegular() {
		return nil //nolint:nilerr // still the link, or never linked
	}

	if err := keepNewerLogin(host, job); err != nil {
		return err
	}

	return relink(host, job)
}

// keepNewerLogin folds job's login into host's, entry by entry.
func keepNewerLogin(host, job string) error {
	refreshed, err := os.ReadFile(job)
	if err != nil {
		return fmt.Errorf("read refreshed login: %w", err)
	}

	target, err := filepath.EvalSymlinks(host)
	if err != nil {
		return fmt.Errorf("resolve host login: %w", err)
	}

	current, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("read host login: %w", err)
	}

	merged, err := mergeLogins(current, refreshed)
	if err != nil || merged == nil {
		return err
	}

	return replaceFile(target, merged)
}

// loginFile is a Claude Code login file, or its mcpOAuth map, with every
// entry left opaque: the engine reads expiresAt and nothing else.
type loginFile map[string]json.RawMessage

const (
	keyClaudeOAuth = "claudeAiOauth"
	keyMCPOAuth    = "mcpOAuth"
)

// mergeLogins is host with each entry of job that expires later - the
// subscription login and each MCP server's, since both rotate - or nil when
// none does. Whatever else host holds stays, so a login the host gained
// while the job ran survives.
func mergeLogins(host, job []byte) ([]byte, error) {
	hostFile := parseLogins(host)
	if hostFile == nil {
		return nil, errHostLogin
	}

	jobFile := parseLogins(job)
	if jobFile == nil {
		return nil, nil // an unreadable login is never returned
	}

	changed := keepNewer(hostFile, jobFile, keyClaudeOAuth)

	mcpChanged, err := mergeMCPLogins(hostFile, jobFile)
	if err != nil || !changed && !mcpChanged {
		return nil, err
	}

	raw, err := json.Marshal(hostFile)
	if err != nil {
		return nil, fmt.Errorf("encode login: %w", err)
	}

	return append(raw, lineEnd...), nil
}

func mergeMCPLogins(host, job loginFile) (bool, error) {
	hostMCP, err := hostMCPLogins(host)
	if err != nil {
		return false, err
	}

	jobMCP := parseLogins(job[keyMCPOAuth])

	changed := false
	for name := range jobMCP {
		changed = keepNewer(hostMCP, jobMCP, name) || changed
	}

	if !changed {
		return false, nil
	}

	raw, err := json.Marshal(hostMCP)
	if err != nil {
		return false, fmt.Errorf("encode MCP logins: %w", err)
	}

	host[keyMCPOAuth] = raw

	return true, nil
}

// hostMCPLogins is the host's MCP logins, empty when it has none.
func hostMCPLogins(host loginFile) (loginFile, error) {
	raw, ok := host[keyMCPOAuth]
	if !ok {
		return loginFile{}, nil
	}

	logins := parseLogins(raw)
	if logins == nil {
		return nil, errHostLogin
	}

	return logins, nil
}

// parseLogins is raw's entries, or nil when raw is not a JSON object.
func parseLogins(raw []byte) loginFile {
	var logins loginFile
	if json.Unmarshal(raw, &logins) != nil {
		return nil
	}

	return logins
}

// keepNewer copies job's entry key into host when it expires later.
func keepNewer(host, job loginFile, key string) bool {
	entry, ok := job[key]
	if !ok || entryExpiry(entry) <= entryExpiry(host[key]) {
		return false
	}

	host[key] = entry

	return true
}

// entryExpiry is when a login entry's access token expires, in unix ms;
// noExpiry when there is none to read.
func entryExpiry(raw json.RawMessage) int64 {
	var entry struct {
		ExpiresAt int64 `json:"expiresAt"`
	}

	if json.Unmarshal(raw, &entry) != nil {
		return noExpiry
	}

	return entry.ExpiresAt
}

// replaceFile writes raw over path atomically, owner-only.
func replaceFile(path string, raw []byte) error {
	dir, base := filepath.Split(path)

	tmp, err := os.CreateTemp(dir, base+".gunkata-*")
	if err != nil {
		return fmt.Errorf("create login temp file: %w", err)
	}

	if err := writeAndClose(tmp, raw); err != nil {
		_ = os.Remove(tmp.Name())

		return err
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())

		return fmt.Errorf("return login to the host: %w", err)
	}

	return nil
}

// relink puts the link to src back at dst in one rename, so a reader never
// finds dst missing.
func relink(src, dst string) error {
	staged := dst + relinkSuffix
	_ = os.Remove(staged)

	if err := os.Symlink(src, staged); err != nil {
		return fmt.Errorf("stage credential link: %w", err)
	}

	if err := os.Rename(staged, dst); err != nil {
		_ = os.Remove(staged)

		return fmt.Errorf("restore credential link: %w", err)
	}

	return nil
}

// dropCredential removes path and its siblings named path.*, the temp files
// an atomic rewrite leaves when it is cut short.
func dropCredential(path string) error {
	dir, base := filepath.Split(path)

	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("list %s: %w", dir, err)
	}

	for _, entry := range entries {
		if !isCredentialCopy(entry.Name(), base) {
			continue
		}

		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return fmt.Errorf("remove credential: %w", err)
		}
	}

	return nil
}

func isCredentialCopy(name, base string) bool {
	return name == base || strings.HasPrefix(name, base+".")
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

// ACP names the event stream is summarised by. Everything else - message and
// thought chunks, command lists, session info - stays in the stream only.
const (
	methodUpdate     = "session/update"
	methodPermission = "session/request_permission"
	updateTool       = "tool_call"
	updateToolUpdate = "tool_call_update"
	updateUsage      = "usage_update"
	toolCompleted    = "completed"
	toolFailed       = "failed"
)

var lineEnd = []byte{'\n'}

// acpMessage is the part of one ACP JSON-RPC message the log summarises.
type acpMessage struct {
	Method string    `json:"method"`
	Params acpParams `json:"params"`
	Result acpResult `json:"result"`
}

type acpParams struct {
	Update   acpUpdate `json:"update"`
	ToolCall acpTool   `json:"toolCall"`
	// ClientInfo is acpx naming itself in the initialize request.
	ClientInfo *acpInfo `json:"clientInfo"`
}

type acpResult struct {
	StopReason string           `json:"stopReason"`
	Usage      map[string]int64 `json:"usage"`
	// AgentInfo is the ACP agent naming itself in its initialize response.
	AgentInfo *acpInfo `json:"agentInfo"`
}

type acpInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type acpUpdate struct {
	acpTool

	SessionUpdate string   `json:"sessionUpdate"`
	Status        *string  `json:"status"`
	Cost          *acpCost `json:"cost"`
}

// acpTool is a tool call as updates carry it: any field but the id may be
// null, and a later update fills in what an earlier one lacked.
type acpTool struct {
	ID    string  `json:"toolCallId"`
	Title *string `json:"title"`
	Kind  *string `json:"kind"`
}

type acpCost struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// toolCall is what the stream has told so far about one tool call.
type toolCall struct {
	title, kind string
	began       time.Time
}

// eventStream splits the executor's stdout into lines and logs the events
// worth watching. It only reads what passes through and never fails a write,
// so a line it cannot parse never cuts the stream on disk short.
type eventStream struct {
	log     *slog.Logger
	partial []byte
	tools   map[string]*toolCall
	cost    *acpCost
	garbled bool // a line that is not JSON has been warned about
	// versions maps each party of the handshake, by name, to its version.
	versions map[string]string
}

func (e *eventStream) Write(p []byte) (int, error) {
	e.partial = append(e.partial, p...)

	for {
		line, rest, found := bytes.Cut(e.partial, lineEnd)
		if !found {
			return len(p), nil
		}

		e.handle(line)
		e.partial = rest
	}
}

// flush handles a last line the executor did not terminate.
func (e *eventStream) flush() {
	e.handle(e.partial)
	e.partial = nil
}

func (e *eventStream) handle(line []byte) {
	if len(bytes.TrimSpace(line)) == emptyLen {
		return
	}

	var msg acpMessage

	// A field of an unexpected type is still JSON; skip it, not the line.
	var typeErr *json.UnmarshalTypeError
	if err := json.Unmarshal(line, &msg); err != nil &&
		!errors.As(err, &typeErr) {
		e.warnGarbled(err)

		return
	}

	switch {
	case msg.Method == methodUpdate:
		e.update(msg.Params.Update)
	case msg.Method == methodPermission:
		tool := msg.Params.ToolCall
		e.log.Info("permission request", keyTool, tool.ID,
			keyTitle, deref(tool.Title), keyKind, deref(tool.Kind))
	case msg.Result.StopReason != unset:
		e.turnEnd(msg.Result)
	case msg.Params.ClientInfo != nil:
		e.version(msg.Params.ClientInfo)
	case msg.Result.AgentInfo != nil:
		e.version(msg.Result.AgentInfo)
	default: // the rest stays in the stream only
	}
}

func (e *eventStream) version(info *acpInfo) {
	if info.Name != unset && info.Version != unset {
		e.versions[info.Name] = info.Version
	}
}

// warnGarbled warns once per executor, however many lines are not JSON.
func (e *eventStream) warnGarbled(err error) {
	if e.garbled {
		return
	}

	e.garbled = true
	e.log.Warn("executor stream has a line that is not JSON", keyErr, err)
}

func (e *eventStream) update(u acpUpdate) {
	switch u.SessionUpdate {
	case updateTool:
		t := e.track(u)
		e.log.Info("tool start", keyTool, u.ID, keyTitle, t.title,
			keyKind, t.kind)
		e.maybeEnd(u, t)
	case updateToolUpdate:
		e.maybeEnd(u, e.track(u))
	case updateUsage:
		if u.Cost != nil {
			e.cost = u.Cost
		}
	default: // chunks and session notices stay in the stream only
	}
}

// track folds an update into what is known of its tool call, keeping the
// latest title and kind that were not null.
func (e *eventStream) track(u acpUpdate) *toolCall {
	t, ok := e.tools[u.ID]
	if !ok {
		t = &toolCall{began: time.Now()}
		e.tools[u.ID] = t
	}

	if u.Title != nil {
		t.title = *u.Title
	}

	if u.Kind != nil {
		t.kind = *u.Kind
	}

	return t
}

func (e *eventStream) maybeEnd(u acpUpdate, t *toolCall) {
	status := deref(u.Status)
	if status != toolCompleted && status != toolFailed {
		return
	}

	delete(e.tools, u.ID)
	e.log.Log(context.Background(), levelFor[status == toolCompleted],
		"tool end", keyTool, u.ID, keyTitle, t.title, keyKind, t.kind,
		"status", status, durSince(t.began))
}

func (e *eventStream) turnEnd(r acpResult) {
	attrs := []any{"stop_reason", r.StopReason, "usage", r.Usage}
	if e.cost != nil {
		attrs = append(attrs,
			"cost", e.cost.Amount, "currency", e.cost.Currency)
	}

	e.log.Info("turn end", attrs...)
}

func deref(s *string) string {
	if s == nil {
		return unset
	}

	return *s
}
