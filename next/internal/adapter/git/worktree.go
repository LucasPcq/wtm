package git

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

var (
	errNoWorktree = errors.New("git worktree list returned no worktree")
	errNoCheckout = errors.New("no checkout given")
)

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

func (r Repo) CommonDir(ctx context.Context) (string, error) {
	out, err := r.git(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	return strings.TrimSpace(string(out)), err
}

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

func (r Repo) AddWorktree(ctx context.Context, addition worktree.Addition) error {
	created, creates := worktree.CreatedBranch(addition.Checkout)
	if !creates {
		return r.add(ctx, addition)
	}
	existed, err := r.refExists(ctx, branch.LocalBranch{Name: created})
	if err != nil {
		return err
	}
	err = r.add(ctx, addition)
	if err == nil || existed {
		return err
	}
	return errors.Join(err, r.dropLeft(ctx, created))
}

func (r Repo) dropLeft(ctx context.Context, name string) error {
	left, err := r.refExists(ctx, branch.LocalBranch{Name: name})
	if err != nil || !left {
		return err
	}
	return r.DeleteBranch(ctx, name)
}

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
		local, err := r.refExists(ctx, branch.LocalBranch{Name: checkout.Name})
		if err != nil {
			return err
		}
		if !local {
			return fmt.Errorf("%w: %s", ErrNoLocalBranch, checkout.Name)
		}
		_, err = r.git(ctx, "worktree", "add", "--", addition.Path, checkout.Name)
		return err
	case worktree.TrackingBranch:
		_, err := r.git(ctx, "worktree", "add", "--track", "-b", checkout.Name, "--", addition.Path, r.remotePrefix()+checkout.Name)
		return err
	}
	return errNoCheckout
}

func (r Repo) RemoveWorktree(ctx context.Context, path string) error {
	_, err := r.git(ctx, "worktree", "remove", "--force", "--", path)
	return err
}

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
		if name := strings.TrimPrefix(strings.TrimSpace(string(headName)), headsPrefix); name != "" {
			return worktree.Rebasing{Branch: name}, nil
		}
	}
	return worktree.Detached{}, nil
}

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
