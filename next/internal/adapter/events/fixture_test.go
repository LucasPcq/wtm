package events

import (
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// identity is the worktree the events of a test are about.
func identity() worktree.Identity {
	return worktree.Identity{Branch: "feat/a", Path: "/repo/.trees/feat-a", Parent: "main", Isolation: project.Isolated, CreatedAt: "2026-10-16T11:00:00Z"}
}

// fixedNow is the clock of a test's publisher.
func fixedNow() time.Time {
	return time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
}

// carried is one event of each type the stream carries, the operation's progress left out.
func carried() []domain.Event {
	numbered := identity()
	numbered.Ordinal = new(2)
	job := domain.JobChange{Worktree: domain.JobWorktree{Branch: "feat/a", Path: "/repo/.trees/feat-a"}, Job: domain.EventJob{Name: "db", Kind: project.Service, Hold: run.HoldDetached}}
	return []domain.Event{
		domain.Snapshot{Worktrees: []worktree.Identity{identity()}},
		domain.Ready{},
		domain.RepoAdded{},
		domain.RepoRemoved{},
		domain.WorktreeCreated{Worktree: identity()},
		domain.WorktreeUpdated{Worktree: numbered, Changed: []worktree.IdentityField{worktree.IdentityOrdinal}},
		domain.WorktreeChanged{Worktree: numbered, Changed: []worktree.PolledField{worktree.PolledHead, worktree.PolledJobs}},
		domain.WorktreeProvisioned{Worktree: numbered, OK: true},
		domain.WorktreeProvisioned{Worktree: numbered, Hook: "make setup", ExitCode: new(2)},
		domain.OperationStarted{Command: "create", Surface: "cli"},
		domain.UnitFinished{Subject: "feat/a", Status: domain.StatusDone},
		domain.OperationFinished{Command: "create", Surface: "cli", Code: "unit.phase_failed"},
		domain.JobStarted{JobChange: job},
		domain.JobStopped{JobChange: domain.JobChange{Worktree: domain.JobWorktree{Branch: "main", Path: "/repo"}, Job: domain.EventJob{Name: "db", Kind: project.Service, Hold: run.HoldDetached}, HeldBy: []domain.JobWorktree{job.Worktree}}},
		domain.JobCrashed{JobChange: domain.JobChange{Worktree: job.Worktree, Job: job.Job, ExitCode: new(-1), LastLines: []string{"listen EADDRINUSE"}}},
		domain.JobExited{JobChange: domain.JobChange{Worktree: job.Worktree, Job: domain.EventJob{Name: "migrate", Kind: project.Task, Hold: run.HoldOwn}, ExitCode: new(0)}},
	}
}
