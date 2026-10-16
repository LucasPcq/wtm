package worktree

import "github.com/LucasPcq/wtm/next/internal/wtm/branch"

//go-sumtype:decl Checkout

type Checkout interface{ checkout() }

type NewBranch struct {
	Name string
	From branch.Ref
}

type ExistingBranch struct{ Name string }

type TrackingBranch struct{ Name string }

func (NewBranch) checkout()      {}
func (ExistingBranch) checkout() {}
func (TrackingBranch) checkout() {}
