// Package tree reads the worktrees of a repository as the forest of their stacks, each under the branch it was created from, with what each needs next: a read, it changes nothing.
package tree

import (
	"context"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "tree"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + repository.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + repository.ConfigInvalid

// stateReaders bounds the worktrees read at once, each costing a few git commands.
const stateReaders = 8

// A UseCase reads the tree of one repository: the ports it reads it through and the service that reads the repository.
type UseCase struct {
	Git          Git
	Meta         Meta
	Jobs         Jobs
	PullRequests PullRequests
	Repository   repository.Service
}

// An Outcome is what Run read: one Item per node of the tree, depth first, and the warnings of what could not be read.
type Outcome = domain.Read[Detail]

// An Item is one node: its Detail, and the error that kept its worktree from being read.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run reads the tree of the repository, each branch's pull request when the request asks for them; a read, it takes no lock, runs no operation and never fetches. An error returned is a refusal of the repository.
func (u UseCase) Run(ctx context.Context, req Request, _ domain.RunOptions) (Outcome, error) {
	// 1. Read the repository, its stack and each worktree.
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Outcome{}, err
	}

	// 2. Report each node, parents before their children, with what it needs next.
	items, warnings := Tree(facts)
	return Outcome{Items: items, Warnings: warnings}, nil
}

// ----------------------------------------------------------------------------
// Questions

// Questions is tree's form: a read, it asks nothing the repository decides.
func (u UseCase) Questions(context.Context, Request) (Questions, error) {
	return Questions{}, nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what tree reports: the repository, the meta records that make its stack, what each worktree holds against its parent, the jobs up and, when req asks for them, the pull requests. The worktrees, the daemon and GitHub are read at once.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	// The stack comes first: a worktree is counted against its parent.
	facts := Facts{Repository: repo, Records: map[string]worktree.Meta{}}
	for _, w := range facts.nodes() {
		// a worktree wtm did not create has no record: a root
		facts.Records[w.Branch()], _ = u.Meta.Load(w.Branch())
	}
	facts.Stack = worktree.StackOf(repo.Worktrees, facts.Records, repo.Config.Worktrees.BaseBranch)

	var (
		wg           sync.WaitGroup
		jobs         []run.Job
		pullRequests []worktree.PullRequest
		jobsErr      error
		prsErr       error
	)
	wg.Go(func() { jobs, jobsErr = u.Jobs(ctx) })
	if req.WithPRs {
		branches := slices.DeleteFunc(repo.Live(), func(branch string) bool { return branch == "" })
		wg.Go(func() { pullRequests, prsErr = u.PullRequests(ctx, branches) })
	}
	states := u.states(ctx, facts)
	wg.Wait()

	facts.States, facts.Jobs, facts.JobsErr, facts.PullRequests, facts.PRsErr = states, jobs, jobsErr, pullRequests, prsErr
	return facts, nil
}

// states reads what each worktree of the tree holds, a few at once.
func (u UseCase) states(ctx context.Context, facts Facts) map[string]State {
	var mu sync.Mutex
	nodes := facts.nodes()
	states := make(map[string]State, len(nodes))
	readers := make(chan struct{}, stateReaders)
	var wg sync.WaitGroup
	for _, w := range nodes {
		readers <- struct{}{}
		wg.Go(func() {
			defer func() { <-readers }()
			state := u.state(ctx, facts, w)
			mu.Lock()
			states[w.Branch()] = state
			mu.Unlock()
		})
	}
	wg.Wait()

	return states
}

// state reads one worktree: whether its directory is gone, then, if it is there, its changes and the commits it and its parent each have that the other lacks.
func (u UseCase) state(ctx context.Context, facts Facts, w worktree.Worktree) State {
	var state State

	// git never lists a locked worktree prunable, even with its directory gone: the disk is asked.
	state.Missing = w.Prunable
	if !state.Missing {
		var location worktree.Location
		location, state.Err = u.Git.Locate(ctx, w.Path)
		state.Missing = state.Err == nil && !location.Exists
	}
	if state.Missing || state.Err != nil {
		return state
	}

	state.Dirty, state.Err = u.Git.Dirty(ctx, w.Path)
	parent, counted := facts.countedAgainst(w)
	if state.Err != nil || !counted {
		return state
	}

	// A count git cannot make leaves the node not counted rather than unreadable.
	state.Ahead = u.count(ctx, worktree.Span{Branch: w.Branch(), Base: parent})
	state.Behind = u.count(ctx, worktree.Span{Branch: parent, Base: w.Branch()})
	return state
}

// ----------------------------------------------------------------------------
// Helpers

func (u UseCase) count(ctx context.Context, span worktree.Span) *int {
	n, err := u.Git.Count(ctx, span)
	if err != nil {
		return nil
	}

	return &n
}
