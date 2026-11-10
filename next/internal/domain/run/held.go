package run

import (
	"cmp"
	"path/filepath"
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
// worktrees in the order of their paths, a worktree's jobs by name; never nil.
func Held(jobs []Job) []HeldRepo {
	sorted := Placed(jobs)
	slices.SortStableFunc(sorted, func(a, b Job) int {
		return cmp.Or(cmp.Compare(a.Repo, b.Repo), cmp.Compare(a.WorkDir, b.WorkDir), cmp.Compare(a.Name, b.Name))
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

// Placed returns jobs, each one started for no repository (a claim the legacy wtm recorded with no origin) placed in the repository and on the branch of another job of its worktree, else in the repository of the shared job it holds.
func Placed(jobs []Job) []Job {
	placed := slices.Clone(jobs)
	for i, job := range placed {
		if job.Repo != "" {
			continue
		}
		if at := slices.IndexFunc(jobs, func(other Job) bool { return other.Repo != "" && other.WorkDir == job.WorkDir }); at >= 0 {
			placed[i].Repo, placed[i].Branch = jobs[at].Repo, cmp.Or(job.Branch, jobs[at].Branch)
			continue
		}
		if at := slices.IndexFunc(jobs, func(other Job) bool { return other.Repo != "" && job.SharedDir != "" && other.WorkDir == job.SharedDir }); at >= 0 {
			placed[i].Repo = jobs[at].Repo
		}
	}
	return placed
}

// Names returns each job of repos as a recap names it: its name, then its worktree and its repository, when it was started for one.
func Names(repos []HeldRepo) []string {
	var names []string
	for _, repo := range repos {
		in := ""
		if repo.Root != "" {
			in = " · " + filepath.Base(repo.Root)
		}
		for _, wt := range repo.Worktrees {
			for _, job := range wt.Jobs {
				names = append(names, job.Name+" ("+cmp.Or(wt.Branch, wt.Path)+in+")")
			}
		}
	}
	return names
}
