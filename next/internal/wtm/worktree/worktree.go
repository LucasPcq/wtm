// Package worktree is a worktree as git lists it and as wtm records it: its state, its checkout, its meta.json and its identity.
package worktree

import "github.com/LucasPcq/wtm/next/internal/wtm/branch"

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
	Onto branch.Ref
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
