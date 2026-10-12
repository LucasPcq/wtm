package git

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

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
// A worktree whose directory is gone is merely detached.
func detachedState(path string) (State, error) {
	adminDir, err := adminDirOf(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Detached{}, nil
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
		if branch := strings.TrimPrefix(strings.TrimSpace(string(headName)), headsPrefix); branch != "" {
			return Rebasing{Branch: branch}, nil
		}
	}
	return Detached{}, nil
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
