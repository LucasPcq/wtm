package worktree

import "path/filepath"

// PlacementParams is what Place decides from: the branch asked for, the path it would land at and what is there, and the repository's branches and live worktrees.
type PlacementParams struct {
	Branch       string
	Path         string
	CheckedOutAt string   // the worktree the branch is checked out in, "" when none
	Location     Location // the path, resolved, and whether something is there
	Occupant     string   // the branch of the worktree at the path, "" when none
	Local        bool     // the branch exists locally
	Remote       bool     // the branch exists on origin
	Live         []string // the branches of the live worktrees
	IfNotExists  bool     // a worktree already there for the branch is not a refusal
}

//go-sumtype:decl Placement

// Placement is where a branch asked for a new worktree lands: Unchanged, CheckedOutElsewhere, PathTaken, a Clash, or Placed.
type Placement interface{ placement() }

// Unchanged is a branch that already has a worktree, at Path, asked for only if it does not: nothing to do.
type Unchanged struct{ Path string }

// CheckedOutElsewhere is a branch already checked out in the worktree at Path.
type CheckedOutElsewhere struct{ Path string }

// PathTaken is a path already occupied by something that is not another branch's worktree.
type PathTaken struct{}

// Start is how a placed branch's worktree begins.
type Start string

// The ways a worktree begins.
const (
	StartCreate   Start = "create"    // a new branch
	StartCheckOut Start = "check_out" // the existing local branch
	StartResume   Start = "resume"    // a local branch tracking the branch of origin
)

// Placed is a branch that gets a new worktree, started as Start; Upstream is the branch of origin it tracks, for StartResume.
type Placed struct {
	Start    Start
	Upstream string
}

func (Unchanged) placement()           {}
func (CheckedOutElsewhere) placement() {}
func (PathTaken) placement()           {}
func (Clash) placement()               {}
func (Placed) placement()              {}

// Place returns where the branch asked for lands.
func Place(params PlacementParams) Placement {
	if params.CheckedOutAt != "" {
		if params.IfNotExists {
			return Unchanged{Path: params.CheckedOutAt}
		}
		return CheckedOutElsewhere{Path: params.CheckedOutAt}
	}

	if params.Location.Exists {
		return params.occupied()
	}

	if clash, found := ClashWith(params.Branch, params.Live); found {
		return clash
	}

	switch {
	case params.Local:
		return Placed{Start: StartCheckOut}
	case params.Remote:
		return Placed{Start: StartResume, Upstream: OriginPrefix + params.Branch}
	}

	return Placed{Start: StartCreate}
}

// occupied returns the placement of a branch whose path already holds something: its own worktree, an unknown file, or another branch's worktree.
func (params PlacementParams) occupied() Placement {
	switch {
	case params.IfNotExists && params.Occupant == params.Branch:
		return Unchanged{Path: params.Path}
	case params.Occupant == "" || params.Occupant == params.Branch:
		return PathTaken{}
	}

	return Clash{Branch: params.Occupant, Name: filepath.Base(params.Path)}
}

// Standing is where an existing branch a new worktree checks out stood against origin, or the branch of origin it resumes, as the JSON envelope reports it.
type Standing struct {
	ExistingBranch bool       `json:"existing_branch"`
	OriginState    Divergence `json:"origin_state,omitempty"`
	OriginAhead    int        `json:"origin_ahead,omitempty"`
	OriginBehind   int        `json:"origin_behind,omitempty"`
	Upstream       string     `json:"upstream,omitempty"`
}

// Checkout is how the worktree of the placed branch gets it: the local branch as it is, a local branch tracking origin's, or a new branch from from.
func (p Placed) Checkout(branch string, from Ref) Checkout {
	switch p.Start {
	case StartCheckOut:
		return ExistingBranch{Name: branch}
	case StartResume:
		return TrackingBranch{Name: branch}
	case StartCreate:
	}

	return NewBranch{Name: branch, From: from}
}
