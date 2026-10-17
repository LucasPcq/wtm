// Package worktree is branches and worktrees as git lists them: names, refs, where a branch stands against origin, a worktree's checkout, its meta.json record and its identity.
package worktree

import (
	"slices"
)

type Worktree struct {
	Path  string
	Main  bool
	State State
}

//go-sumtype:decl State

type State interface{ state() }

type OnBranch struct{ Branch string }

type Rebasing struct{ Branch string }

type Detached struct{}

type Bare struct{}

func (OnBranch) state() {}
func (Rebasing) state() {}
func (Detached) state() {}
func (Bare) state()     {}

type Merge struct {
	Path string
	Onto Ref
}

func (w Worktree) Branch() string {
	switch state := w.State.(type) {
	case OnBranch:
		return state.Branch
	case Rebasing:
		return state.Branch
	case Detached, Bare:
	}
	return ""
}

type Location struct {
	Path   string
	Exists bool
}

//go-sumtype:decl Checkout

type Checkout interface{ checkout() }

type NewBranch struct {
	Name string
	From Ref
}

type ExistingBranch struct{ Name string }

type TrackingBranch struct{ Name string }

func (NewBranch) checkout()      {}
func (ExistingBranch) checkout() {}
func (TrackingBranch) checkout() {}

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

type Addition struct {
	Path     string
	Checkout Checkout
}

const mainOrdinal = 0

func FreeOrdinal(taken []int) int {
	candidate := mainOrdinal + 1
	for slices.Contains(taken, candidate) {
		candidate++
	}
	return candidate
}
