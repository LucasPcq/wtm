// Package envshow reads one worktree's .env files app by app, file by file, key by key: where each file comes from, what each key follows (a job's port or address, a value of run.toml, a value wtm owns, nothing), its value in the main checkout and here, what wtm writes into it, and the keys to check; a read, it changes nothing and withholds every value wtm does not write unless asked.
package envshow

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "env-show"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase reads a worktree's .env files in one repository: the ports it reads through and the service that reads the repository.
type UseCase struct {
	Git        Git
	Meta       Meta
	Files      Files
	PublicPort PublicPort
	Repository repository.Service
}

// An Outcome is what Run read: the worktree read and the warnings of what keeps wtm from writing into its files.
type Outcome = domain.Read[Detail]

// An Item is the worktree read.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run reads the .env files of the worktree req names, or the one the caller stands in: a read, it takes no lock and runs no operation. An error returned is a refusal of the request or of the repository.
func (u UseCase) Run(ctx context.Context, req Request, _ domain.RunOptions) (Outcome, error) {
	// 1. Read the repository, run.toml, the worktree's record and its .env files with their sources.
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Outcome{}, err
	}

	// 2. Report each file, each key, what it follows and what wtm writes into it.
	item, warnings := Show(facts)
	return Outcome{Items: []Item{item}, Warnings: warnings}, nil
}

// ----------------------------------------------------------------------------
// Questions

// Questions is env show's form over the worktrees of the repository, the one req names, else the one the caller stands in, its answer.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	repo, located, err := u.observeRequest(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(repo, req, located), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what env show reports: the repository and its run.toml, the worktree req names, the meta records, the port a named address announces, and each declared .env file of the worktree with its template, its parent's copy and the main worktree's.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, located, err := u.observeRequest(ctx, req)
	if err != nil {
		return Facts{}, err
	}
	target, err := target(onBranch(repo.Snapshot), req, located)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Repository: repo, Target: target, ShowValues: req.ShowValues}
	facts.Records, _ = repository.Records(u.Meta, repo.Live())
	facts.PublicPort = u.PublicPort(ctx, repo.Global.ProxyPort())
	parent := ""
	if source := facts.record().SourceBranch; source != "" {
		parent = repo.CheckedOutAt(source)
	}
	facts.Reads, err = repository.EnvFiles(u.Files, repository.EnvFilesParams{Files: repo.Config.Env.Files, Dir: target.Path, ParentDir: parent, MainDir: repo.MainPath})
	return facts, err
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
