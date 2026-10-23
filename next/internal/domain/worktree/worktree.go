// Package worktree is branches and worktrees as git lists them: names, refs, where a branch stands against origin, a worktree's checkout, its meta.json record, its identity, and the stack of worktrees by the branch each was created from.
package worktree

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"
)

// Worktree is one worktree as git lists it: its path, whether it is the main one, what it has checked out, whether it is locked, and whether its directory is gone (git calls it prunable).
type Worktree struct {
	Path     string
	Main     bool
	State    State
	Locked   bool
	Prunable bool
}

//go-sumtype:decl State

// State is what a worktree has checked out: OnBranch, Rebasing, Bisecting, Detached or Bare.
type State interface{ state() }

// OnBranch is a worktree on a branch.
type OnBranch struct{ Branch string }

// Rebasing is a worktree in the middle of a rebase of a branch.
type Rebasing struct{ Branch string }

// Bisecting is a worktree in the middle of a bisect started from a branch.
type Bisecting struct{ Branch string }

// Detached is a worktree on a commit, on no branch.
type Detached struct{}

// Bare is the bare repository's entry, which has no working tree.
type Bare struct{}

func (OnBranch) state()  {}
func (Rebasing) state()  {}
func (Bisecting) state() {}
func (Detached) state()  {}
func (Bare) state()      {}

// Merge asks to fast-forward the branch checked out in the worktree at Path onto Onto.
type Merge struct {
	Path string
	Onto Ref
}

// Branch returns the branch the worktree is on, rebasing or bisecting, "" when it is on none.
func (w Worktree) Branch() string {
	switch state := w.State.(type) {
	case OnBranch:
		return state.Branch
	case Rebasing:
		return state.Branch
	case Bisecting:
		return state.Branch
	case Detached, Bare:
	}

	return ""
}

// Holding returns the index of the deepest of list whose path holds the directory dir, dir itself or one of its parents, -1 when none does; an empty path or dir holds nothing.
func Holding[T any](list []T, dir string, path func(T) string) int {
	found, deepest := -1, 0
	for i, item := range list {
		root := path(item)
		holds := dir == root || strings.HasPrefix(dir, root+string(filepath.Separator))
		if dir != "" && root != "" && holds && len(root) > deepest {
			found, deepest = i, len(root)
		}
	}

	return found
}

// Location is a path resolved through its symbolic links, and whether something is there.
type Location struct {
	Path   string
	Exists bool
}

//go-sumtype:decl Checkout

// Checkout is the branch a new worktree checks out: a NewBranch, an ExistingBranch, or a TrackingBranch.
type Checkout interface{ checkout() }

// NewBranch is a branch created from From.
type NewBranch struct {
	Name string
	From Ref
}

// ExistingBranch is a local branch that already exists.
type ExistingBranch struct{ Name string }

// TrackingBranch is a local branch created to track the branch of origin of the same name.
type TrackingBranch struct{ Name string }

func (NewBranch) checkout()      {}
func (ExistingBranch) checkout() {}
func (TrackingBranch) checkout() {}

// CreatedBranch returns the branch checkout creates, and reports whether it creates one: an undo deletes it.
func CreatedBranch(checkout Checkout) (string, bool) {
	switch c := checkout.(type) {
	case NewBranch:
		return c.Name, true
	case TrackingBranch:
		return c.Name, true
	case ExistingBranch:
	}

	return "", false
}

// Addition asks to add a worktree at Path with Checkout.
type Addition struct {
	Path     string
	Checkout Checkout
}

const mainOrdinal = 0

// FreeOrdinal returns the lowest worktree number not in taken; the main worktree's number, 0, is never free.
func FreeOrdinal(taken []int) int {
	candidate := mainOrdinal + 1
	for slices.Contains(taken, candidate) {
		candidate++
	}

	return candidate
}

// Name returns how the worktree is named: its branch, else its path.
func (w Worktree) Name() string {
	return cmp.Or(w.Branch(), w.Path)
}
