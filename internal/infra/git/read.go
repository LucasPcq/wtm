package git

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/rules/worktree"
)

// Reader is what a command observes of git.
type Reader interface {
	Worktrees(ctx context.Context) ([]Worktree, error)
	MainWorktree(ctx context.Context) (Worktree, error)
	Toplevel(ctx context.Context) (string, error)
	GitCommonDir(ctx context.Context) (string, error)
	Locate(ctx context.Context, path string) (Location, error)
	Branches(ctx context.Context) (Branches, error)
	RefExists(ctx context.Context, ref Ref) (bool, error)
	Tip(ctx context.Context, ref Ref) (string, error)
	AheadBehind(ctx context.Context, params AheadBehindParams) (AheadBehind, error)
}

// Worktree is one entry of `git worktree list`; the first is the main one.
type Worktree struct {
	Path     string
	Head     string
	Main     bool
	Locked   bool
	Prunable bool
	State    State
}

//go-sumtype:decl State

// State is what a worktree has checked out.
type State interface{ state() }

type OnBranch struct {
	Branch string
}

// Rebasing is a HEAD detached by a rebase in progress, on the branch the
// rebase will move.
type Rebasing struct {
	Branch string
}

type Detached struct{}

type Bare struct{}

func (OnBranch) state() {}
func (Rebasing) state() {}
func (Detached) state() {}
func (Bare) state()     {}

// Location is a path as git writes it, its symlinks resolved, and whether
// anything is there.
type Location struct {
	Path   string
	Exists bool
}

// Branches are named without their prefix: Remote holds `feature`, not
// `origin/feature`.
type Branches struct {
	Local  []string
	Remote []string
}

type AheadBehindParams struct {
	Of      Ref
	Against Ref
}

// AheadBehind counts the commits Of has that Against lacks, and the reverse.
type AheadBehind struct {
	Ahead  int
	Behind int
}

const remoteHead = "HEAD"

var errNoWorktree = errors.New("git worktree list returned no worktree")

func (r Repo) Worktrees(ctx context.Context) ([]Worktree, error) {
	out, err := r.git(ctx, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	worktrees := parseWorktrees(string(out))
	for i, worktree := range worktrees {
		if _, detached := worktree.State.(Detached); !detached {
			continue
		}
		state, err := detachedState(worktree.Path)
		if err != nil {
			return nil, err
		}
		worktrees[i].State = state
	}
	return worktrees, nil
}

func (r Repo) MainWorktree(ctx context.Context) (Worktree, error) {
	worktrees, err := r.Worktrees(ctx)
	if err != nil {
		return Worktree{}, err
	}
	if len(worktrees) == 0 {
		return Worktree{}, errNoWorktree
	}
	return worktrees[0], nil
}

// Toplevel is the root of the worktree Dir is in, as git writes it.
func (r Repo) Toplevel(ctx context.Context) (string, error) {
	out, err := r.git(ctx, "rev-parse", "--show-toplevel")
	return strings.TrimSpace(string(out)), err
}

// GitCommonDir is the .git every worktree of the repository shares.
func (r Repo) GitCommonDir(ctx context.Context) (string, error) {
	out, err := r.git(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	return strings.TrimSpace(string(out)), err
}

// Locate reads a relative path from Dir, as `git -C` does.
func (r Repo) Locate(_ context.Context, path string) (Location, error) {
	absolute := path
	if !filepath.IsAbs(path) {
		joined, err := filepath.Abs(filepath.Join(r.Dir, path))
		if err != nil {
			return Location{}, err
		}
		absolute = joined
	}
	resolved, err := RealPath(absolute)
	if err != nil {
		return Location{}, err
	}
	_, err = os.Lstat(resolved)
	if errors.Is(err, fs.ErrNotExist) {
		return Location{Path: resolved}, nil
	}
	if err != nil {
		return Location{}, err
	}
	return Location{Path: resolved, Exists: true}, nil
}

// RealPath resolves an absolute path the way git records a worktree's: the
// symlinks of the part that exists resolved, the rest kept as written.
func RealPath(path string) (string, error) {
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	parent := filepath.Dir(clean)
	if parent == clean {
		return clean, nil
	}
	resolvedParent, err := RealPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, filepath.Base(clean)), nil
}

func (r Repo) Branches(ctx context.Context) (Branches, error) {
	remotePrefix := r.remotePrefix()
	out, err := r.git(ctx, "for-each-ref", "--format=%(refname)", strings.TrimSuffix(headsPrefix, "/"), strings.TrimSuffix(remotePrefix, "/"))
	if err != nil {
		return Branches{}, err
	}
	var branches Branches
	for line := range strings.Lines(string(out)) {
		refname := strings.TrimSpace(line)
		if local, ok := strings.CutPrefix(refname, headsPrefix); ok {
			branches.Local = append(branches.Local, local)
			continue
		}
		if remote, ok := strings.CutPrefix(refname, remotePrefix); ok && remote != remoteHead {
			branches.Remote = append(branches.Remote, remote)
		}
	}
	return branches, nil
}

// RefExists reads git's exit status 1 as an absent ref; any other failure is
// an error, never an absence.
func (r Repo) RefExists(ctx context.Context, ref Ref) (bool, error) {
	name, err := r.refName(ref)
	if err != nil {
		return false, err
	}
	_, err = r.git(ctx, "show-ref", "--verify", "--quiet", name)
	var gitErr *Error
	if errors.As(err, &gitErr) && gitErr.Exit == 1 {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Tip is the commit a ref points at; an absent ref is an error.
func (r Repo) Tip(ctx context.Context, ref Ref) (string, error) {
	name, err := r.refName(ref)
	if err != nil {
		return "", err
	}
	out, err := r.git(ctx, "rev-parse", "--verify", name)
	return strings.TrimSpace(string(out)), err
}

func (r Repo) AheadBehind(ctx context.Context, params AheadBehindParams) (AheadBehind, error) {
	of, err := r.refName(params.Of)
	if err != nil {
		return AheadBehind{}, err
	}
	against, err := r.refName(params.Against)
	if err != nil {
		return AheadBehind{}, err
	}
	out, err := r.git(ctx, "rev-list", "--left-right", "--count", of+"..."+against)
	if err != nil {
		return AheadBehind{}, err
	}
	return parseAheadBehind(string(out))
}

func parseAheadBehind(out string) (AheadBehind, error) {
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return AheadBehind{}, fmt.Errorf("git rev-list --left-right --count: unexpected output %q", out)
	}
	ahead, err := strconv.Atoi(fields[0])
	if err != nil {
		return AheadBehind{}, err
	}
	behind, err := strconv.Atoi(fields[1])
	if err != nil {
		return AheadBehind{}, err
	}
	return AheadBehind{Ahead: ahead, Behind: behind}, nil
}

// parseWorktrees reads -z porcelain: one attribute per NUL-terminated field, an
// empty field closing each worktree, so a path holds any byte but NUL.
func parseWorktrees(out string) []Worktree {
	var worktrees []Worktree
	var current *Worktree
	for _, field := range strings.Split(out, "\x00") {
		key, value, _ := strings.Cut(field, " ")
		switch {
		case key == "worktree":
			worktrees = append(worktrees, Worktree{Path: value, Main: len(worktrees) == 0, State: Detached{}})
			current = &worktrees[len(worktrees)-1]
		case current == nil:
		case key == "HEAD":
			current.Head = value
		case key == "branch":
			current.State = OnBranch{Branch: strings.TrimPrefix(value, headsPrefix)}
		case key == "bare":
			current.State = Bare{}
		case key == "locked":
			current.Locked = true
		case key == "prunable":
			current.Prunable = true
		case field == "":
			current = nil
		}
	}
	return worktrees
}

// detachedState reads the worktree's admin directory: a rebase in progress
// leaves the branch it moves in head-name, which git worktree list never shows.
func detachedState(path string) (State, error) {
	adminDir, err := adminDirOf(path)
	if errors.Is(err, fs.ErrNotExist) {
		return stateOf(worktree.RebasingParams{DirGone: true}), nil
	}
	if err != nil {
		return nil, err
	}
	headNames, err := rebaseHeadNames(adminDir)
	if err != nil {
		return nil, err
	}
	return stateOf(worktree.RebasingParams{HeadNames: headNames}), nil
}

func rebaseHeadNames(adminDir string) ([]string, error) {
	var headNames []string
	for _, rebase := range []string{"rebase-merge", "rebase-apply"} {
		headName, err := os.ReadFile(filepath.Join(adminDir, rebase, "head-name"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		headNames = append(headNames, string(headName))
	}
	return headNames, nil
}

func stateOf(params worktree.RebasingParams) State {
	if branch := worktree.RebasingBranch(params); branch != "" {
		return Rebasing{Branch: branch}
	}
	return Detached{}
}

// adminDirOf follows the worktree's .git: the directory itself in the main
// worktree, a `gitdir:` file, relative or absolute, in a linked one.
func adminDirOf(path string) (string, error) {
	dotGit := filepath.Join(path, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return dotGit, nil
	}
	content, err := os.ReadFile(dotGit)
	if err != nil {
		return "", err
	}
	adminDir, ok := strings.CutPrefix(strings.TrimSpace(string(content)), "gitdir: ")
	if !ok {
		return "", errors.New("not a gitdir file: " + dotGit)
	}
	if filepath.IsAbs(adminDir) {
		return adminDir, nil
	}
	return filepath.Join(path, adminDir), nil
}
