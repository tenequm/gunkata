package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tenequm/gunkata/internal/kata"
)

const (
	githubTree = "https://github.com/"
	treeMarker = "tree"
	// A tree URL's path after the host: owner, repo, "tree", ref, then the
	// skill's path inside the repo.
	treeOwner   = 0
	treeRepo    = 1
	treeKeyword = 2
	treeRef     = 3
	treePath    = 4
	treeParts   = 5
	skillErrFmt = "skill %q: %w"
	quotedFmt   = "%w: %q"
)

var (
	errSkillURL = errors.New(
		"skill URL is not https://github.com/<o>/<r>/tree/<ref>/<path>")
	errSkillName = errors.New("two skills share a directory name")
	errSkillDirs = errors.New("harness has no known skills directory")
)

// harnessSkillDirs is where each harness loads skills from, relative to its
// HOME. A harness absent here cannot take skills yet.
var harnessSkillDirs = map[string]string{
	harnessClaude: ".claude/skills",
	// The Gemini home's global skills. agy also reads
	// .gemini/antigravity-cli/skills, and .gemini/skills and .agents/skills
	// under its cwd - none of them present in a bare HOME.
	harnessAgy:   ".gemini/config/skills",
	harnessCodex: ".agents/skills",
}

// treeURL is a parsed GitHub tree URL.
type treeURL struct {
	repo string // clone URL
	ref  string
	path string
}

func parseTreeURL(raw string) (treeURL, error) {
	rest, ok := strings.CutPrefix(raw, githubTree)
	if !ok {
		return treeURL{}, fmt.Errorf(quotedFmt, errSkillURL, raw)
	}

	parts := strings.SplitN(rest, "/", treeParts)
	if len(parts) != treeParts || parts[treeKeyword] != treeMarker {
		return treeURL{}, fmt.Errorf(quotedFmt, errSkillURL, raw)
	}

	return treeURL{
		repo: githubTree + parts[treeOwner] + "/" + parts[treeRepo],
		ref:  parts[treeRef],
		path: parts[treePath],
	}, nil
}

func isRemoteSkill(entry string) bool {
	return strings.Contains(entry, "://")
}

// checkSkills rejects, before anything runs, skills a job's harness cannot
// load and malformed skill URLs.
func checkSkills(k *kata.Kata) error {
	for _, job := range k.Jobs() {
		if job.Executor == nil {
			continue
		}

		if err := checkJobSkills(job.Executor); err != nil {
			return fmt.Errorf(jobFmt, job.Name, err)
		}
	}

	return nil
}

func checkJobSkills(p *kata.Profile) error {
	if len(p.Skills) == emptyLen {
		return nil
	}

	if _, ok := harnessSkillDirs[p.Harness]; !ok {
		return fmt.Errorf(quotedFmt, errSkillDirs, p.Harness)
	}

	for _, entry := range p.Skills {
		if !isRemoteSkill(entry) {
			continue
		}

		if _, err := parseTreeURL(entry); err != nil {
			return err
		}
	}

	return nil
}

// skillFetcher snapshots every declared skill into dst/<dir name> once,
// resolving each remote ref to a SHA at run start.
type skillFetcher struct {
	log     *slog.Logger
	kataDir string
	dst     string
	// snapshots maps each entry to its snapshot dir; sources, for the
	// record, to the SHA or local path it came from.
	snapshots map[string]string
	sources   map[string]string
	owner     map[string]string // snapshot dir name -> entry that took it
}

func fetchSkills(
	ctx context.Context, k *kata.Kata, dst string, log *slog.Logger,
) (*skillFetcher, error) {
	f := &skillFetcher{
		log:       log,
		kataDir:   k.Dir,
		dst:       dst,
		snapshots: map[string]string{},
		sources:   map[string]string{},
		owner:     map[string]string{},
	}

	for _, job := range k.Jobs() {
		if job.Executor == nil {
			continue
		}

		if err := f.fetchAll(ctx, job.Executor.Skills); err != nil {
			return nil, err
		}
	}

	return f, nil
}

func (f *skillFetcher) fetchAll(ctx context.Context, entries []string) error {
	for _, entry := range entries {
		if _, done := f.snapshots[entry]; done {
			continue
		}

		if err := f.fetch(ctx, entry); err != nil {
			return fmt.Errorf(skillErrFmt, entry, err)
		}
	}

	return nil
}

func (f *skillFetcher) fetch(ctx context.Context, entry string) error {
	began := time.Now()

	src, err := f.resolve(ctx, entry)
	if err != nil {
		return err
	}
	defer src.cleanup()

	name := filepath.Base(src.dir)
	if prior, taken := f.owner[name]; taken {
		return fmt.Errorf(quotedFmt, errSkillName, prior)
	}

	snapshot := filepath.Join(f.dst, name)
	if err := os.CopyFS(snapshot, os.DirFS(src.dir)); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	f.owner[name] = entry
	f.snapshots[entry], f.sources[entry] = snapshot, src.source
	f.log.Info("skill fetched", "entry", entry, "source", src.source,
		durSince(began))

	return nil
}

// skillSource is a skill directory on disk and where it came from.
type skillSource struct {
	dir     string
	source  string // SHA or local path
	cleanup func()
}

// resolve finds the skill's directory on disk: a local path as is, a tree URL
// via a shallow clone that cleanup removes.
func (f *skillFetcher) resolve(ctx context.Context, entry string) (
	skillSource, error,
) {
	if !isRemoteSkill(entry) {
		dir := entry
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(f.kataDir, dir)
		}

		return skillSource{dir: dir, source: dir, cleanup: func() {}}, nil
	}

	tree, err := parseTreeURL(entry)
	if err != nil {
		return skillSource{}, err
	}

	clone, err := os.MkdirTemp(unset, "gunkata-skill-")
	if err != nil {
		return skillSource{}, fmt.Errorf("create clone dir: %w", err)
	}

	cleanup := func() { _ = os.RemoveAll(clone) }

	sha, err := shallowClone(ctx, tree, clone)
	if err != nil {
		cleanup()

		return skillSource{}, err
	}

	return skillSource{
		dir: filepath.Join(clone, tree.path), source: sha, cleanup: cleanup,
	}, nil
}

// shallowClone fetches one ref engine-side, with the engine's own
// credentials, and reports the SHA it resolved to.
func shallowClone(ctx context.Context, tree treeURL, dir string) (
	string, error,
) {
	//nolint:gosec // G204: the repo and ref come from the kata, the input
	clone := exec.CommandContext(ctx, "git", "clone", "--quiet",
		"--depth", "1", "--branch", tree.ref, tree.repo, dir)
	if out, err := clone.CombinedOutput(); err != nil {
		return unset, fmt.Errorf("clone %s@%s: %w: %s",
			tree.repo, tree.ref, err, out)
	}

	head := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD")

	out, err := head.Output()
	if err != nil {
		return unset, fmt.Errorf("resolve %s@%s: %w", tree.repo, tree.ref, err)
	}

	return strings.TrimSpace(string(out)), nil
}

// materializeSkills copies the job's skill snapshots to where its harness
// loads skills from. A copy, not a link, so the executor cannot alter the
// run's evidence.
func materializeSkills(home, harness string, skills []string) error {
	root := filepath.Join(home, harnessSkillDirs[harness])

	for _, snapshot := range skills {
		name := filepath.Base(snapshot)

		err := os.CopyFS(filepath.Join(root, name), os.DirFS(snapshot))
		if err != nil {
			return fmt.Errorf("materialize skill %s: %w", name, err)
		}
	}

	return nil
}
