// Package resolve reads where a worktree is, the one named by its branch, its path or a part of its branch found in no other, else the one the caller stands in: what the shell function go enters. A read, it changes nothing.
package resolve

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "resolve"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase resolves a worktree of one repository: the port it locates paths through and the service that reads the repository.
type UseCase struct {
	Git        Git
	Repository repository.Service
}

// An Outcome is what Run read: one Item, the worktree resolved.
type Outcome = domain.Read[Detail]

// An Item is the worktree resolved: its Detail, or the error that kept it from being read.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run resolves the worktree req names, or the one it stands in; a read, it takes no lock and runs no operation. An error returned is a refusal of the request or of the repository: a name that matches no worktree or several.
func (u UseCase) Run(ctx context.Context, req Request, _ domain.RunOptions) (Outcome, error) {
	// 1. Read the repository, settle the worktree the request names, then whether its directory is there.
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Outcome{}, err
	}

	// 2. Report it.
	return Report(facts), nil
}

// ----------------------------------------------------------------------------
// Questions

// Questions is resolve's form over the repository: the worktrees it may resolve, the one req resolves, and its problems.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	repo, located, err := u.observeRequest(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(repo, req, located), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what resolve reports: the repository, the worktree req names, and whether its directory is gone.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, located, err := u.observeRequest(ctx, req)
	if err != nil {
		return Facts{}, err
	}
	target, err := Target(repo.Snapshot, req, located)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Target: target}
	facts.Missing, facts.Err = repository.Gone(ctx, u.Git, target)
	facts.Interrupted = ctx.Err() != nil
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
