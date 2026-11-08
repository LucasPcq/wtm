// Package runlist reads what run.toml declares for one worktree: its profiles and its jobs, where each stands there, and what can be done with it; a read, it changes nothing and never starts wtm's daemon.
package runlist

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "run-list"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase lists what run.toml declares for a worktree of one repository: the ports it reads through, where wtm keeps the repository's state (the jobs' logs under it), and the service that reads the repository.
type UseCase struct {
	Git        Git
	Meta       Meta
	Jobs       Jobs
	ReadLog    ReadLog
	StateDir   string
	Repository repository.Service
}

// An Outcome is what Run read: the worktree read, the warnings of what could not be asked, and the follow-up picked of its actions.
type Outcome = domain.Read[Detail]

// An Item is the worktree read.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run reads what run.toml declares for the worktree req names, or the one the caller stands in: a read, it takes no lock, runs no operation and never starts the daemon. An error returned is a refusal of the request or of the repository.
func (u UseCase) Run(ctx context.Context, req Request, _ domain.RunOptions) (Outcome, error) {
	// 1. Read the repository, run.toml, the worktree, the jobs the daemon runs and the logs of those it does not list.
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Outcome{}, err
	}

	// 2. Report each profile and each job with what can be done with it.
	item, warnings := List(facts)
	return Outcome{Items: []Item{item}, Warnings: warnings, Interrupted: facts.Interrupted}, nil
}

// ----------------------------------------------------------------------------
// Questions

// Questions is run list's form over the worktrees of the repository, the one req names, else the one the caller stands in, its answer.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	repo, located, err := u.observeRequest(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(repo, req, located), nil
}

// ----------------------------------------------------------------------------
// Observe

// The codes of a repository run list cannot read: run.toml does not exist (followed by run init), or does not load.
const (
	CodeNotInitialized = Name + ".not_initialized"
	CodeInit           = Name + ".init"
	CodeRunInvalid     = Name + ".run_invalid"
)

// commandRunInit sets the run module up in a repository: not migrated yet.
const commandRunInit = "run init"

// Observe reads what run list reports: the repository and its run.toml, the worktree req names, the meta records, the jobs the daemon knows, never starting it, and the last line of the log of each job of run.toml it does not list there. A repository with no run.toml is refused with CodeNotInitialized, one whose run.toml does not load with CodeRunInvalid.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, located, err := u.observeRequest(ctx, req)
	if err != nil {
		return Facts{}, err
	}
	if repo.RunErr != nil {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeRunInvalid, Message: "invalid run.toml: " + repo.RunErr.Error()}
	}
	if !repo.Run.Declared {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeNotInitialized, Message: "no run.toml: no job is declared", FollowUp: &domain.FollowUp{Command: commandRunInit, Code: CodeInit, Message: "declare the jobs"}}
	}
	target, err := domain.TargetWorktree(repo.Snapshot.MainFirst(), req.query(located))
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Repository: repo, Target: target, Logged: map[string]bool{}}
	facts.Records, facts.RecordErrs = repository.Records(u.Meta, repo.Live())
	facts.Asked = repository.Ask(ctx, repository.AskParams{Jobs: u.Jobs, Restart: u.Repository.Restart}, func() {})
	if dir := run.LogDir(u.StateDir, target.Branch()); dir != "" && facts.JobsErr == nil {
		for _, job := range repo.Run.Jobs {
			if !run.Listed(facts.Jobs, target.Path, job.Name) {
				lines, _ := u.ReadLog(run.LogFile{Dir: dir, Job: job.Name}, 1)
				facts.Logged[job.Name] = len(lines) > 0
			}
		}
	}
	return facts, nil
}

// observeRequest reads the repository and the path the worktree req names stands for (located).
func (u UseCase) observeRequest(ctx context.Context, req Request) (worktree.Repository, string, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return worktree.Repository{}, "", err
	}

	located, err := repository.Located(ctx, u.Git, req.Worktree, req.Caller)
	return repo, located, err
}
