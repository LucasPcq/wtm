package run

import (
	"cmp"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// A HeldRepo is the jobs of one repository as a surface reports them, by its
// main checkout's root (empty for jobs started for none), worktree by worktree.
type HeldRepo struct {
	Root      string         `json:"root"`
	Worktrees []HeldWorktree `json:"worktrees"`
}

// A HeldWorktree is the jobs of one worktree: its branch (empty on none), its path, and each job.
type HeldWorktree struct {
	Branch string    `json:"branch,omitempty"`
	Path   string    `json:"path"`
	Jobs   []HeldJob `json:"jobs"`
}

// A HeldJob is one job: its name, its kind, what of it the daemon holds, and whether it is a shared job's own run, which the worktrees holding a claim on it share.
type HeldJob struct {
	Name   string          `json:"name"`
	Kind   project.JobKind `json:"kind,omitempty"`
	Hold   Hold            `json:"hold"`
	Shared bool            `json:"shared,omitempty"`
}

// Held returns jobs by repository, the repositories and, in each, the
// worktrees in the order of their paths, a worktree's jobs in the order given;
// never nil.
func Held(jobs []Job) []HeldRepo {
	sorted := slices.Clone(jobs)
	slices.SortStableFunc(sorted, func(a, b Job) int {
		return cmp.Or(cmp.Compare(a.Repo, b.Repo), cmp.Compare(a.WorkDir, b.WorkDir))
	})
	repos := []HeldRepo{}
	for _, job := range sorted {
		if n := len(repos); n == 0 || repos[n-1].Root != job.Repo {
			repos = append(repos, HeldRepo{Root: job.Repo})
		}
		repo := &repos[len(repos)-1]
		if n := len(repo.Worktrees); n == 0 || repo.Worktrees[n-1].Path != job.WorkDir {
			repo.Worktrees = append(repo.Worktrees, HeldWorktree{Branch: job.Branch, Path: job.WorkDir})
		}
		wt := &repo.Worktrees[len(repo.Worktrees)-1]
		wt.Jobs = append(wt.Jobs, HeldJob{Name: job.Name, Kind: job.Kind, Hold: job.Hold, Shared: job.SharedDir != "" && job.Hold != HoldClaim})
	}
	return repos
}

// Count returns how many jobs repos hold.
func Count(repos []HeldRepo) int {
	n := 0
	for _, repo := range repos {
		for _, wt := range repo.Worktrees {
			n += len(wt.Jobs)
		}
	}
	return n
}
