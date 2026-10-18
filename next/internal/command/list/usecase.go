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

// stateReaders bounds the worktrees read at once, each costing a few git commands.
const stateReaders = 8

// A UseCase lists the worktrees of one repository: the ports it reads them through, the service that reads the repository, and the command lines of the actions a worktree offers.
type UseCase struct {
	Git          Git
	Meta         Meta
	Jobs         Jobs
	PullRequests PullRequests
	Repository   repository.Service
	Commands     Commands
}

// An Outcome is what Run read: one Item per worktree, the warnings of what could not be read, and the follow-ups chosen of it.
type Outcome = domain.Read[Detail]

// An Item is one worktree: its Detail, or the error that kept it from being read.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run reads every worktree of the repository, its pull request when the request asks for them, after fetching their branches from origin when it asks to; a read, it takes no lock and runs no operation. An error returned is a refusal of the repository.
func (u UseCase) Run(ctx context.Context, req Request, _ domain.RunOptions) (Outcome, error) {
	// 1. A refresh fetches the branches of the worktrees first, best effort: an origin out of reach leaves the counts of the refs already fetched.
	if req.Fetch {
		u.fetch(ctx)
	}

	// 2. Read the repository and each worktree.
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Outcome{}, err
	}

	// 3. Report each worktree, the main one first.
	items, warnings := List(facts)
	return Outcome{Items: items, Warnings: warnings}, nil
}

// FollowUp is the command line that carries out action on the worktree of detail, as the next step of a read.
func (u UseCase) FollowUp(detail Detail, action Action) domain.FollowUp {
	return domain.FollowUp{Argv: u.Commands(action, detail), Code: Name + "." + string(action), Message: Labels[action]}
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what list reports: the repository, what each worktree holds, the jobs up and, when req asks for them, the open pull requests. The worktrees, the daemon and GitHub are read at once.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Repository: repo, WithPRs: req.WithPRs}
	var (
		wg           sync.WaitGroup
		jobs         []run.Job
		pullRequests []worktree.PullRequest
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
	var mu sync.Mutex
	states := make(map[string]State, len(facts.Worktrees))
	readers := make(chan struct{}, stateReaders)
	var wg sync.WaitGroup
	for _, w := range facts.Worktrees {
		readers <- struct{}{}
		wg.Go(func() {
			defer func() { <-readers }()
			state := u.state(ctx, facts, w)
			mu.Lock()
			states[w.Path] = state
			mu.Unlock()
		})
	}
	wg.Wait()

	return states
}

// state reads one worktree: its changes, unless its directory is gone, the commits its branch has past the base branch, and its meta record, which a worktree wtm did not create lacks.
func (u UseCase) state(ctx context.Context, facts Facts, w worktree.Worktree) State {
	var state State
	if branch := w.Branch(); branch != "" {
		state.Meta, _ = u.Meta.Load(branch)
	}
	if w.Prunable {
		return state
	}

	state.Dirty, state.Err = u.Git.Dirty(ctx, w.Path)
	if state.Err != nil || !facts.counted(w) {
		return state
	}

	state.Ahead, state.Err = u.Git.Count(ctx, worktree.Span{Branch: w.Branch(), Base: facts.Config.Worktrees.BaseBranch})
	return state
}

// ----------------------------------------------------------------------------
// Helpers

// fetch fetches from origin the branch of each worktree origin has, ignoring a fetch that fails.
func (u UseCase) fetch(ctx context.Context) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return
	}

	for _, branch := range repo.Live() {
		if branch != "" && repo.Remote(branch) {
			_ = u.Git.FetchRef(ctx, branch)
		}
	}
}
