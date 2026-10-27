package worktree

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// Repository is what a command reads before deciding: the path of the main checkout, the project's config and global settings, its run.toml, and the repository's snapshot.
type Repository struct {
	MainPath string
	Config   project.Config
	Global   project.Global
	Run      project.Run
	RunErr   error // why run.toml could not be loaded, nil when it was
	Snapshot
}

// PathOf is where the worktree of the branch name would be created: under the configured base path, from the main checkout.
func (r Repository) PathOf(name string) string {
	return filepath.Join(r.MainPath, r.Config.Worktrees.BasePath, PathSegment(name))
}

// NamesWorktrees reports whether run.toml gives each worktree a name derived from its branch, so two branches deriving the same name clash; a run.toml that failed to load is assumed to.
func (r Repository) NamesWorktrees() bool {
	return r.RunErr != nil || len(r.Run.Jobs) > 0 || len(r.Run.Profiles) > 0
}

// PlannedIsolation is the isolation a plan shows for the isolation given: none when run.toml declares nothing a worktree isolates; the one given, which the worktree records all the same, when run.toml could not be read.
func (r Repository) PlannedIsolation(given project.Isolation) project.Isolation {
	if r.RunErr == nil && !r.Run.IsolationApplies() {
		return ""
	}

	return given
}

// NamedLive returns the branches of the live worktrees a new worktree's name must not clash with: all of them when run.toml names worktrees, none otherwise.
func (r Repository) NamedLive() []string {
	if !r.NamesWorktrees() {
		return nil
	}

	return r.Live()
}

// ExistingBase returns the configured base branch when it is a branch, local or origin/<branch> origin has (Snapshot.Candidate): what a worktree whose parent is gone or loops moves onto; "" otherwise.
func (r Repository) ExistingBase() string {
	if base := r.Config.Worktrees.BaseBranch; r.Candidate(base) {
		return base
	}

	return ""
}

// MissingBase returns the configured base branch, and reports whether it is missing: configured, yet no local branch of that name.
func (r Repository) MissingBase() (string, bool) {
	base := r.Config.Worktrees.BaseBranch
	return base, base != "" && !r.Local(base)
}

// CountedAgainstBase reports whether the commits of the worktree w are counted against the base branch: not for the main worktree, one on no branch, nor when the base branch is not configured or missing.
func (r Repository) CountedAgainstBase(w Worktree) bool {
	base := r.Config.Worktrees.BaseBranch
	return !w.Main && w.Branch() != "" && base != "" && r.Local(base)
}

// CountedAgainstParent returns the branch the commits of the worktree w are counted against in stack, its parent or, for a root, the base branch, and reports whether they are counted: not for the main worktree nor a member of a cycle, which has no parent to sync onto, and only when both are local branches, which git can count.
func (r Repository) CountedAgainstParent(stack Stack, w Worktree) (string, bool) {
	branch := w.Branch()
	parent, ok := stack.Parent(branch)
	if !ok {
		parent = r.Config.Worktrees.BaseBranch
	}

	return parent, !w.Main && branch != "" && !stack.InCycle(branch) && parent != branch && r.Local(parent) && r.Local(branch)
}
