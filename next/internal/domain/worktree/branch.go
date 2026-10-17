package worktree

import "strconv"

// Branch is a branch as git lists it: its name, without `origin/` for a branch of origin, and the commit at its tip.
type Branch struct {
	Name string
	Tip  string
}

//go-sumtype:decl Ref

// Ref is a branch a new branch can start from: a LocalBranch or a RemoteBranch.
type Ref interface{ ref() }

// LocalBranch is a local branch, by name.
type LocalBranch struct{ Name string }

// RemoteBranch is a branch of origin, by its name without `origin/`.
type RemoteBranch struct{ Name string }

func (LocalBranch) ref()  {}
func (RemoteBranch) ref() {}

// AheadBehind counts the commits a local branch has that its origin branch lacks (Ahead), and the reverse (Behind).
type AheadBehind struct {
	Ahead  int
	Behind int
}

// Divergence is where a local branch stands against its origin branch.
type Divergence string

// The ways a branch stands against origin; NoOrigin when origin has no branch of that name.
const (
	NoOrigin Divergence = ""
	UpToDate Divergence = "up-to-date"
	Behind   Divergence = "behind"
	Ahead    Divergence = "ahead"
	Diverged Divergence = "diverged"
)

// Branches is the repository's local branches and the branches of origin.
type Branches struct {
	Local  []Branch
	Remote []Branch
}

// Divergence returns where a branch with these counts stands: diverged when it is both ahead and behind, up to date when it is neither.
func (ab AheadBehind) Divergence() Divergence {
	switch {
	case ab.Ahead > 0 && ab.Behind > 0:
		return Diverged
	case ab.Behind > 0:
		return Behind
	case ab.Ahead > 0:
		return Ahead
	}

	return UpToDate
}

// String spells the counts as a user reads them: `1 ahead, 2 behind`.
func (ab AheadBehind) String() string {
	return strconv.Itoa(ab.Ahead) + " ahead, " + strconv.Itoa(ab.Behind) + " behind"
}
