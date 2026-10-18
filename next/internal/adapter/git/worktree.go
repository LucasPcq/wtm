package git

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

var (
	errNoWorktree = errors.New("git worktree list returned no worktree")
	errNoCheckout = errors.New("no checkout given")
)

// Worktrees returns the repository's worktrees, the main one first. A detached
// worktree in the middle of a rebase, or of a bisect, is reported as rebasing
// the branch it started from.
func (r Repo) Worktrees(ctx context.Context) ([]worktree.Worktree, error) {
	out, err := r.git(ctx, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}

	worktrees := parseWorktrees(string(out))
	for i, w := range worktrees {
		if _, detached := w.State.(worktree.Detached); !detached {
			continue
		}

		state, err := detachedState(w.Path)
		if err != nil {
			return nil, err
		}
		worktrees[i].State = state
	}

	return worktrees, nil
}

// MainWorktree returns the repository's main worktree.
func (r Repo) MainWorktree(ctx context.Context) (worktree.Worktree, error) {
	worktrees, err := r.Worktrees(ctx)
	if err != nil {
		return worktree.Worktree{}, err
	}

	if len(worktrees) == 0 {
		return worktree.Worktree{}, errNoWorktree
	}
	return worktrees[0], nil
}

// CommonDir returns the absolute path of the repository's common git
// directory, shared by all its worktrees.
func (r Repo) CommonDir(ctx context.Context) (string, error) {
	out, err := r.git(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	return strings.TrimSpace(string(out)), err
}

// Locate resolves path, relative to r.Dir unless absolute, to its real path
// and reports whether something already exists there.
func (r Repo) Locate(_ context.Context, path string) (worktree.Location, error) {
	absolute := path
	if !filepath.IsAbs(path) {
		joined, err := filepath.Abs(filepath.Join(r.Dir, path))
		if err != nil {
			return worktree.Location{}, err
		}
		absolute = joined
	}

	resolved, err := RealPath(absolute)
	if err != nil {
		return worktree.Location{}, err
	}

	_, err = os.Lstat(resolved)
	if errors.Is(err, fs.ErrNotExist) {
		return worktree.Location{Path: resolved}, nil
	}
	if err != nil {
		return worktree.Location{}, err
	}
	return worktree.Location{Path: resolved, Exists: true}, nil
}

// RealPath returns path with its symlinks resolved, as git spells a worktree's
// path; a part that does not exist yet is kept as given under its resolved
// parent.
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

// AddWorktree adds a worktree at addition.Path with its checkout. A branch the
// checkout creates is deleted again when the add fails, unless it existed
// before. Checking out an existing branch with no local branch fails with
// ErrNoLocalBranch.
func (r Repo) AddWorktree(ctx context.Context, addition worktree.Addition) error {
	created, creates := worktree.CreatedBranch(addition.Checkout)
	if !creates {
		return r.add(ctx, addition)
	}

	existed, err := r.refExists(ctx, worktree.LocalBranch{Name: created})
	if err != nil {
		return err
	}

	err = r.add(ctx, addition)
	if err == nil || existed {
		return err
	}
	return errors.Join(err, r.dropLeft(ctx, created))
}

// dropLeft deletes the branch name if a failed add left it behind.
func (r Repo) dropLeft(ctx context.Context, name string) error {
	left, err := r.refExists(ctx, worktree.LocalBranch{Name: name})
	if err != nil || !left {
		return err
	}
	return r.DeleteBranch(ctx, name)
}

// add runs git worktree add for each kind of checkout.
func (r Repo) add(ctx context.Context, addition worktree.Addition) error {
	switch checkout := addition.Checkout.(type) {
	case worktree.NewBranch:
		from, err := r.refName(checkout.From)
		if err != nil {
			return err
		}
		_, err = r.git(ctx, "worktree", "add", "--no-track", "-b", checkout.Name, "--", addition.Path, from)
		return err
	case worktree.ExistingBranch:
		local, err := r.refExists(ctx, worktree.LocalBranch(checkout))
		if err != nil {
			return err
		}
		if !local {
			return fmt.Errorf("%w: %s", ErrNoLocalBranch, checkout.Name)
		}

		_, err = r.git(ctx, "worktree", "add", "--", addition.Path, checkout.Name)
		return err
	case worktree.TrackingBranch:
		_, err := r.git(ctx, "worktree", "add", "--track", "-b", checkout.Name, "--", addition.Path, remotePrefix+checkout.Name)
		return err
	}
	return errNoCheckout
}

// RemoveWorktree removes the worktree at path, its changes included.
func (r Repo) RemoveWorktree(ctx context.Context, path string) error {
	_, err := r.git(ctx, "worktree", "remove", "--force", "--", path)
	return err
}

// parseWorktrees reads git worktree list --porcelain -z; the first worktree is
// the main one and each starts detached until a branch or bare line says
// otherwise.
func parseWorktrees(out string) []worktree.Worktree {
	var worktrees []worktree.Worktree
	var current *worktree.Worktree
	for field := range strings.SplitSeq(out, "\x00") {
		key, value, _ := strings.Cut(field, " ")
		switch {
		case key == "worktree":
			worktrees = append(worktrees, worktree.Worktree{Path: value, Main: len(worktrees) == 0, State: worktree.Detached{}})
			current = &worktrees[len(worktrees)-1]
		case current == nil:
		case key == "branch":
			current.State = worktree.OnBranch{Branch: strings.TrimPrefix(value, headsPrefix)}
		case key == "bare":
			current.State = worktree.Bare{}
		case field == "":
			current = nil
		}
	}

	return worktrees
}

// detachedState is what a detached worktree is doing: rebasing or bisecting the
// branch its rebase or bisect state names, or plainly detached. A worktree whose .git is
// gone, or whose rebase or bisect began on no branch, reads as detached.
func detachedState(path string) (worktree.State, error) {
	adminDir, err := adminDirOf(path)
	if errors.Is(err, fs.ErrNotExist) {
		return worktree.Detached{}, nil
	}
	if err != nil {
		return nil, err
	}

	for _, rebase := range []string{"rebase-merge", "rebase-apply"} {
		headName, err := os.ReadFile(filepath.Join(adminDir, rebase, "head-name"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}

		// A rebase begun detached names "detached HEAD", not a branch.
		if name, ok := strings.CutPrefix(strings.TrimSpace(string(headName)), headsPrefix); ok && name != "" {
			return worktree.Rebasing{Branch: name}, nil
		}
	}

	return bisectState(adminDir)
}

// bisectState is the state of a detached worktree whose git directory is
// adminDir: bisecting the branch a bisect in progress started from, or
// detached.
func bisectState(adminDir string) (worktree.State, error) {
	start, err := os.ReadFile(filepath.Join(adminDir, "BISECT_START"))
	if errors.Is(err, fs.ErrNotExist) {
		return worktree.Detached{}, nil
	}
	if err != nil {
		return nil, err
	}

	// git records the branch's short name, or the commit when the bisect began detached.
	name := strings.TrimSpace(string(start))
	if name == "" || isObjectID(name) {
		return worktree.Detached{}, nil
	}
	return worktree.Bisecting{Branch: name}, nil
}

// isObjectID reports whether s spells a full SHA-1 or SHA-256 object id.
func isObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	return strings.Trim(s, "0123456789abcdef") == ""
}

// adminDirOf returns the git directory of the worktree at path: its .git
// directory, or the one its .git file points to.
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
