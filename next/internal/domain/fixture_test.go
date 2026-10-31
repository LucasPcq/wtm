package domain

import (
	"maps"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

var identity = worktree.Identity{Path: "/trees/feat-x", Branch: "feat/x"}

// everyEvent holds one event of each variant of Event.
var everyEvent = []Event{
	UnitStarted{Subject: "feat/x"},
	PhaseStarted{Subject: "feat/x", Phase: PhaseOnCreate, Log: "/logs/on_create.log"},
	PhaseFinished{Subject: "feat/x", Phase: PhaseOnCreate, Failed: true},
	Output{Subject: "feat/x", Line: "set up <feat/x>"},
	UnitFinished{Subject: "feat/x", Status: StatusDone},
	WorktreeCreated{Worktree: identity},
	WorktreeUpdated{Worktree: identity, Changed: []worktree.IdentityField{worktree.IdentityOrdinal}},
	WorktreeChanged{Worktree: identity, Changed: []worktree.PolledField{worktree.PolledHead}},
	WorktreeProvisioned{Worktree: identity, Hook: "make setup", ExitCode: new(3)},
	WorktreeRemoved{Worktree: identity},
	OperationStarted{Command: "create", Surface: "cli"},
	OperationFinished{Command: "create", Surface: "cli", Code: "unit.phase_failed"},
	Snapshot{Worktrees: []worktree.Identity{identity}},
	Ready{},
	RepoAdded{},
	RepoRemoved{},
	JobStarted{JobChange: job},
	JobStopped{JobChange: job},
	JobCrashed{JobChange: JobChange{Worktree: job.Worktree, Job: job.Job, ExitCode: new(1), LastLines: []string{"listen EADDRINUSE"}}},
	JobExited{JobChange: JobChange{Worktree: job.Worktree, Job: EventJob{Name: "migrate", Kind: project.Task}, ExitCode: new(0)}},
}

var job = JobChange{Worktree: JobWorktree{Branch: "feat/x", Path: "/trees/feat-x"}, Job: EventJob{Name: "web", Kind: project.Service, URL: "http://localhost:3100"}}

// wrapper is an error that wraps inner, so a test sees whether AsError looks through wrapping.
type wrapper struct{ inner error }

func (w wrapper) Error() string { return "wrapped" }
func (w wrapper) Unwrap() error { return w.inner }

// answers is a request that takes every path but "nope".
type answers map[string][]string

func (a answers) Answer(path string, values []string) (answers, error) {
	if path == "nope" {
		return a, Unknown(path)
	}

	next := maps.Clone(a)
	next[path] = values
	return next, nil
}

func (a answers) Values(path string) []string { return a[path] }

// A calledRequest records who asked it.
type calledRequest struct{ caller Caller }

func (r calledRequest) From(caller Caller) calledRequest {
	r.caller = caller
	return r
}
