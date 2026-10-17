package worktree

import (
	"context"
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

// SnapshotSource is what ReadSnapshot reads a repository through.
type SnapshotSource interface {
	Worktrees(ctx context.Context) ([]Worktree, error)
	Branches(ctx context.Context) (Branches, error)
	AheadBehind(ctx context.Context, name string) (AheadBehind, error)
}

// ReadSnapshot returns the repository source reads: its worktrees, its local
// and remote branches, and how far ahead and behind its origin each local
// branch is when their tips differ.
func ReadSnapshot(ctx context.Context, source SnapshotSource) (Snapshot, error) {
	worktrees, err := source.Worktrees(ctx)
	if err != nil {
		return Snapshot{}, err
	}

	branches, err := source.Branches(ctx)
	if err != nil {
		return Snapshot{}, err
	}

	divergence := map[string]AheadBehind{}
	for _, name := range branches.Differing() {
		divergence[name], err = source.AheadBehind(ctx, name)
		if err != nil {
			return Snapshot{}, err
		}
	}
	return Snapshot{Worktrees: worktrees, Branches: branches, Divergence: divergence}, nil
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
