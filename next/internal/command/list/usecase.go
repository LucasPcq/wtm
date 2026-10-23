// Package list reads the worktrees of a repository: what each holds, where its branch stands, the jobs up in it and its pull request; a read, it changes nothing.
package list

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "list"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

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
	// 1. Read the repository and each worktree, origin fetched first when the request asks to.
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Outcome{}, err
	}

	// 2. Report each worktree, the main one first, with what it offers.
	items, warnings := List(facts, req.Caller)
	return Outcome{Items: items, Warnings: warnings, Interrupted: facts.Interrupted}, nil
}

// ----------------------------------------------------------------------------
// Questions

// Questions is list's form: a read, it asks nothing the repository decides.
func (u UseCase) Questions(context.Context, Request) (Questions, error) {
	return Questions{}, nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what list reports: the repository, after fetching origin when req asks to, the meta records, what each worktree holds, the jobs up and, when req asks for them, the newest pull request of each branch. The worktrees, the daemon and GitHub are read at once.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	// A refresh fetches origin first, best effort: an origin out of reach leaves the counts of the refs already fetched, and a warning.
	var fetchErr error
	if req.Fetch {
		fetchErr = u.Git.Fetch(ctx)
	}

	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Repository: repo, FetchErr: fetchErr}
	facts.Records, facts.RecordErrs = repository.Records(u.Meta, repo.Live())
	worktrees := facts.Checkouts()
	var read []State
	facts.Asked = repository.Ask(ctx, repository.AskParams{Jobs: u.Jobs, PullRequests: u.PullRequests, Branches: repo.Live(), WithPRs: req.WithPRs}, func() {
		read = repository.Each(worktrees, func(w worktree.Worktree) State { return u.state(ctx, facts, w) })
	})

	facts.States = make(map[string]State, len(worktrees))
	for i, w := range worktrees {
		facts.States[w.Path] = read[i]
	}
	return facts, nil
}

// state reads one worktree: whether its directory is gone, then, if it is there, its changes and the commits its branch has past the base branch.
func (u UseCase) state(ctx context.Context, facts Facts, w worktree.Worktree) State {
	presence, err := repository.Present(ctx, u.Git, w)
	state := State{Missing: presence.Missing, Dirty: presence.Dirty, Err: err}
	if state.Missing || state.Err != nil || !facts.CountedAgainstBase(w) {
		return state
	}

	// A branch with no commit yet, an orphan, has nothing to count: it is not counted rather than unreadable.
	gap, err := u.Git.AheadBehindOf(ctx, worktree.Span{Branch: w.Branch(), Base: facts.Config.Worktrees.BaseBranch})
	if state.Err = repository.Interrupted(ctx, err); err != nil {
		return state
	}

	state.Ahead = gap.Ahead
	return state
}
