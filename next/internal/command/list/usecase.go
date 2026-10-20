// Package list reads the worktrees of a repository: what each holds, where its branch stands, the jobs up in it and its pull request; a read, it changes nothing.
package list

import (
	"context"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "list"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + repository.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + repository.ConfigInvalid

// A UseCase lists the worktrees of one repository: the ports it reads them through and the service that reads the repository.
type UseCase struct {
	Git          Git
	Meta         Meta
	Jobs         Jobs
	PullRequests PullRequests
	Repository   repository.Service
}

// An Outcome is what Run read: one Item per worktree, the warnings of what could not be read, and the follow-ups chosen of it.
type Outcome = domain.Read[Detail]

// An Item is one worktree: its Detail, or the error that kept it from being read.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run reads every worktree of the repository, its pull request when the request asks for them, after fetching origin when it asks to; a read, it takes no lock and runs no operation. An error returned is a refusal of the repository.
func (u UseCase) Run(ctx context.Context, req Request, _ domain.RunOptions) (Outcome, error) {
	// 1. A refresh fetches origin first, best effort: an origin out of reach leaves the counts of the refs already fetched.
	if req.Fetch {
		_ = u.Git.Fetch(ctx)
	}

	// 2. Read the repository and each worktree.
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Outcome{}, err
	}

	// 3. Report each worktree, the main one first, with what it offers.
	items, warnings := List(facts)
	return Outcome{Items: items, Warnings: warnings}, nil
}

// ----------------------------------------------------------------------------
// Questions

// Questions is list's form: a read, it asks nothing the repository decides.
func (u UseCase) Questions(context.Context, Request) (Questions, error) {
	return Questions{}, nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what list reports: the repository, what each worktree holds, the jobs up and, when req asks for them, the open pull requests. The worktrees, the daemon and GitHub are read at once.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Repository: repo}
	var (
		wg           sync.WaitGroup
		jobs         []run.Job
		pullRequests worktree.PullRequests
		jobsErr      error
		prsErr       error
	)
	wg.Go(func() { jobs, jobsErr = u.Jobs(ctx) })
	if req.WithPRs {
		wg.Go(func() { pullRequests, prsErr = u.PullRequests(ctx) })
	}
	states := u.states(ctx, facts)
	wg.Wait()

	facts.States, facts.Jobs, facts.JobsErr, facts.PullRequests, facts.PRsErr = states, jobs, jobsErr, pullRequests, prsErr
	return facts, nil
}

// states reads what each worktree of facts holds, a few at once.
func (u UseCase) states(ctx context.Context, facts Facts) map[string]State {
	worktrees := facts.Checkouts()
	read := repository.Each(worktrees, func(w worktree.Worktree) State { return u.state(ctx, facts, w) })
	states := make(map[string]State, len(worktrees))
	for i, w := range worktrees {
		states[w.Path] = read[i]
	}

	return states
}

// state reads one worktree: its meta record, which a worktree wtm did not create lacks, whether its directory is gone, then, if it is there, its changes and the commits its branch has past the base branch.
func (u UseCase) state(ctx context.Context, facts Facts, w worktree.Worktree) State {
	var state State
	if branch := w.Branch(); branch != "" {
		state.Meta, _ = u.Meta.Load(branch)
	}

	var presence repository.Presence
	presence, state.Err = repository.Present(ctx, u.Git, w)
	state.Missing, state.Dirty = presence.Missing, presence.Dirty
	if state.Missing || state.Err != nil || !facts.counted(w) {
		return state
	}

	// A branch with no commit yet, an orphan, has nothing to count: it is not counted rather than unreadable.
	gap, _ := u.Git.AheadBehindOf(ctx, worktree.Span{Branch: w.Branch(), Base: facts.Config.Worktrees.BaseBranch})
	state.Ahead = gap.Ahead
	return state
}
