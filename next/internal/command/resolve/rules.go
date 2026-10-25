package resolve

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what resolve reads: the worktree the request names, whether its directory is gone, the error that kept that from being told, and whether the read was interrupted.
type Facts struct {
	Target      worktree.Worktree
	Missing     bool
	Err         error
	Interrupted bool
}

// The codes of resolve's failures.
const (
	CodeNoWorktree = Name + ".no_worktree"
	CodeUnreadable = Name + domain.Unreadable
)

// Target is the worktree req names of snapshot: by its branch, by a part of its branch found in no other, or by a path (located, resolved and existing, "" when the name is none) in it; else the deepest holding req.Caller.Cwd, where the caller stands; else the main one. It refuses a name that matches no worktree or several, and a repository with no worktree.
func Target(snapshot worktree.Snapshot, req Request, located string) (worktree.Worktree, error) {
	worktrees := snapshot.MainFirst()
	if len(worktrees) == 0 {
		return worktree.Worktree{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeNoWorktree, Message: "the repository has no worktree to resolve"}
	}

	return domain.TargetWorktree(worktrees, req.query(located))
}

// ----------------------------------------------------------------------------
// The detail

// A Detail is the worktree resolved: its branch (empty on none), its path, and whether its directory is gone.
type Detail struct {
	Branch  string `json:"branch"`
	Path    string `json:"path"`
	Missing bool   `json:"missing"` // its directory is gone: the path leads nowhere
}

// ----------------------------------------------------------------------------
// Report

// Report is the worktree the facts resolved as the read's one item.
func Report(facts Facts) Outcome {
	w := facts.Target
	if facts.Err != nil {
		return Outcome{Items: []Item{domain.Unread[Detail](domain.UnreadParams{Command: Name, Subject: w.Name(), Path: w.Path, Err: facts.Err})}, Interrupted: facts.Interrupted}
	}

	detail := Detail{Branch: w.Branch(), Path: w.Path, Missing: facts.Missing}
	return Outcome{Items: []Item{{Subject: w.Name(), Status: domain.StatusDone, Detail: &detail}}, Interrupted: facts.Interrupted}
}
