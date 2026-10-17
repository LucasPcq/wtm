package worktree

import "path/filepath"

type PlacementParams struct {
	Branch       string
	Path         string
	CheckedOutAt string
	Location     Location
	Occupant     string
	Local        bool
	Remote       bool
	Live         []string
	IfNotExists  bool
}

//go-sumtype:decl Placement

type Placement interface{ placement() }

type Unchanged struct{ Path string }

type CheckedOutElsewhere struct{ Path string }

type PathTaken struct{}

type Start string

const (
	StartCreate   Start = "create"
	StartCheckOut Start = "check_out"
	StartResume   Start = "resume"
)

type Placed struct {
	Start    Start
	Upstream string
}

func (Unchanged) placement()           {}
func (CheckedOutElsewhere) placement() {}
func (PathTaken) placement()           {}
func (Clash) placement()               {}
func (Placed) placement()              {}

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
		return Placed{Start: StartResume, Upstream: "origin/" + params.Branch}
	}
	return Placed{Start: StartCreate}
}

func (params PlacementParams) occupied() Placement {
	switch {
	case params.IfNotExists && params.Occupant == params.Branch:
		return Unchanged{Path: params.Path}
	case params.Occupant == "" || params.Occupant == params.Branch:
		return PathTaken{}
	}
	return Clash{Branch: params.Occupant, Name: filepath.Base(params.Path)}
}
