package worktree

import "errors"

// ErrNoCommonHistory is the error of a branch and its base that share no commit: there is no fork point to rebase from.
var ErrNoCommonHistory = errors.New("no common history")

// A Fork is where a branch forked from its base: the commit, and the branch's own commits after it.
type Fork struct {
	Commit string
	Own    int
}

// A Rebase replays, in the worktree at Path, the commits of its branch after Upstream onto Onto: Keep leaves a rebase that stops on a conflict in progress, where it is aborted otherwise, and Autostash stashes the worktree's uncommitted changes around it.
type Rebase struct {
	Path      string
	Onto      Ref
	Upstream  string
	Keep      bool
	Autostash bool
}

// Rebased is what a rebase that ran did: the files it stopped on a conflict in, none when it went through, and whether it left that rebase in progress, and whether git kept the stashed changes in its stash, their apply having conflicted.
type Rebased struct {
	Conflicts []string
	Kept      bool
	Stashed   bool
}

// Conflicted reports whether the rebase stopped on a conflict.
func (r Rebased) Conflicted() bool {
	return len(r.Conflicts) > 0
}

// A BranchUpdate moves the local branch Branch from the commit From to the commit To, only while it still stands at From.
type BranchUpdate struct {
	Branch string
	From   string
	To     string
}

// A Push sends the local branch Branch to origin's branch of the same name, overwriting it only while it stands at Expect: a commit, or "" for a branch origin must not have yet.
type Push struct {
	Branch string
	Expect string
}
