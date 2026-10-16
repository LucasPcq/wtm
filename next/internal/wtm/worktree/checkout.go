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
