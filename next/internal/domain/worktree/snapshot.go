package worktree

import (
	"slices"
	"strings"
)

// OriginPrefix spells a branch of origin the way a user names it: origin/<branch>.
const OriginPrefix = "origin/"

// Snapshot is the repository at one moment: its worktrees as git lists them, its local and remote branches, and the ahead/behind counts of each local branch whose origin tip differs.
type Snapshot struct {
	Worktrees  []Worktree
	Branches   Branches
	Divergence map[string]AheadBehind
}

// Local reports whether name is a local branch.
func (s Snapshot) Local(name string) bool {
	return slices.ContainsFunc(s.Branches.Local, func(b Branch) bool { return b.Name == name })
}

// Remote reports whether name is a branch of origin.
func (s Snapshot) Remote(name string) bool {
	return slices.ContainsFunc(s.Branches.Remote, func(b Branch) bool { return b.Name == name })
}

// Candidate reports whether name can be started from: a local branch, or origin/<branch> of a branch on origin.
func (s Snapshot) Candidate(name string) bool {
	remote, onOrigin := strings.CutPrefix(name, OriginPrefix)
	return s.Local(name) || onOrigin && s.Remote(remote)
}

// Ref returns the ref a new branch starts from when asked to start from from: origin's branch for origin/<branch> without a local branch of that name, the local branch otherwise.
func (s Snapshot) Ref(from string) Ref {
	if remote, onOrigin := strings.CutPrefix(from, OriginPrefix); onOrigin && !s.Local(from) {
		return RemoteBranch{Name: remote}
	}

	return LocalBranch{Name: from}
}

// Tip returns the commit the local branch name stands at, "" when there is no such branch.
func (s Snapshot) Tip(name string) string {
	at := slices.IndexFunc(s.Branches.Local, func(b Branch) bool { return b.Name == name })
	if at < 0 {
		return ""
	}

	return s.Branches.Local[at].Tip
}

// CheckedOutAt returns the path of the worktree name is checked out in, or "" when none is.
func (s Snapshot) CheckedOutAt(name string) string {
	at := slices.IndexFunc(s.Worktrees, func(w Worktree) bool { return w.Branch() == name })
	if at < 0 {
		return ""
	}

	return s.Worktrees[at].Path
}

// Occupant returns the branch of the worktree at location, or "" when no worktree is there.
func (s Snapshot) Occupant(location Location) string {
	at := slices.IndexFunc(s.Worktrees, func(w Worktree) bool { return w.Path == location.Path })
	if at < 0 {
		return ""
	}

	return s.Worktrees[at].Branch()
}

// AheadBehind returns the counts of the commits name has that origin's branch lacks, and the reverse; zero when their tips agree.
func (s Snapshot) AheadBehind(name string) AheadBehind {
	return s.Divergence[name]
}

// Standing returns where name stands against origin: NoOrigin when origin has no such branch.
func (s Snapshot) Standing(name string) Divergence {
	if !s.Remote(name) {
		return NoOrigin
	}

	return s.AheadBehind(name).Divergence()
}

// An Origin is where a branch stands against origin's branch of the same name: the commits each has that the other lacks, and what they make of it.
type Origin struct {
	Ahead  int        `json:"ahead"`
	Behind int        `json:"behind"`
	State  Divergence `json:"state"`
}

// OriginOf returns where name stands against origin, nil when name is empty or origin has no such branch.
func (s Snapshot) OriginOf(name string) *Origin {
	standing := s.Standing(name)
	if name == "" || standing == NoOrigin {
		return nil
	}

	counts := s.AheadBehind(name)
	return &Origin{Ahead: counts.Ahead, Behind: counts.Behind, State: standing}
}

// MainFirst returns the Checkouts, the main worktree first, as a reader of one of them lists them.
func (s Snapshot) MainFirst() []Worktree {
	worktrees := s.Checkouts()
	slices.SortStableFunc(worktrees, func(a, b Worktree) int {
		switch {
		case a.Main == b.Main:
			return 0
		case a.Main:
			return -1
		}
		return 1
	})

	return worktrees
}

// Checkouts returns the worktrees that have a working tree: all but a bare repository's entry.
func (s Snapshot) Checkouts() []Worktree {
	return slices.DeleteFunc(slices.Clone(s.Worktrees), func(w Worktree) bool {
		_, bare := w.State.(Bare)
		return bare
	})
}

// Live returns the branch of each worktree, "" for one on no branch.
func (s Snapshot) Live() []string {
	names := make([]string, 0, len(s.Worktrees))
	for _, w := range s.Worktrees {
		names = append(names, w.Branch())
	}

	return names
}

// Differing returns the local branches whose origin tip is another commit: the ones a snapshot counts ahead and behind.
func (b Branches) Differing() []string {
	var names []string
	for _, local := range b.Local {
		at := slices.IndexFunc(b.Remote, func(remote Branch) bool { return remote.Name == local.Name })
		if at >= 0 && b.Remote[at].Tip != local.Tip {
			names = append(names, local.Name)
		}
	}

	return names
}

// StandingOf is where the branch of a placed worktree stood against origin: the counts of an existing branch it checks out, the branch of origin it resumes, nothing for a new branch.
func (s Snapshot) StandingOf(branch string, placed Placed) Standing {
	switch placed.Start {
	case StartCheckOut:
		counts := s.AheadBehind(branch)
		return Standing{ExistingBranch: true, OriginState: s.Standing(branch), OriginAhead: counts.Ahead, OriginBehind: counts.Behind}
	case StartResume:
		return Standing{Upstream: placed.Upstream}
	case StartCreate:
	}

	return Standing{}
}

// StartsFrom is the local branch the new worktree of branch begins from, started as start: branch itself when it checks out the existing branch, from when a new branch starts from that local branch, "" otherwise.
func (s Snapshot) StartsFrom(branch string, start Start, from string) string {
	switch {
	case start == StartCheckOut:
		return branch
	case start == StartCreate && s.Local(from):
		return from
	}

	return ""
}
