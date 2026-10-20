// Package tree reads the worktrees of a repository as the forest of their stacks, each under the branch it was created from, with what each needs next: a read, it changes nothing.
package tree

import (
	"context"
	"errors"
	"io/fs"
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
	facts := Facts{Repository: repo}
	records, unread := map[string]worktree.Meta{}, map[string]error{}
	for _, w := range facts.nodes() {
		record, err := u.Meta.Load(w.Branch())
		// a worktree wtm did not create has no record: a root; one whose record does not read has no known place
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			unread[w.Branch()] = err
		}
		records[w.Branch()] = record
	}
	facts.Stack = worktree.StackOf(repo.Worktrees, records, repo.Config.Worktrees.BaseBranch)

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
	nodes := facts.nodes()
	read := repository.Each(nodes, func(w worktree.Worktree) State {
		if err := unread[w.Branch()]; err != nil {
			return State{Err: err}
		}
		return u.state(ctx, facts, w)
	})
	wg.Wait()

	facts.States = make(map[string]State, len(nodes))
	for i, w := range nodes {
		facts.States[w.Branch()] = read[i]
	}
	facts.Jobs, facts.JobsErr, facts.PullRequests, facts.PRsErr = jobs, jobsErr, pullRequests, prsErr
	facts.Interrupted = ctx.Err() != nil
	return facts, nil
}

// state reads one worktree: whether its directory is gone, then, if it is there, its changes and the commits it has past its parent and those of its parent it lacks, or past the base branch for a root, which has no parent to fall behind.
func (u UseCase) state(ctx context.Context, facts Facts, w worktree.Worktree) State {
	presence, err := repository.Present(ctx, u.Git, w)
	state := State{Missing: presence.Missing, Dirty: presence.Dirty, Err: err}
	against, counted := facts.countedAgainst(w)
	if state.Missing || state.Err != nil || !counted {
		return state
	}

	span := worktree.Span{Branch: w.Branch(), Base: against}
	if _, child := facts.Stack.Parent(w.Branch()); child {
		counts, err := u.Git.AheadBehindOf(ctx, span)
		if state.Err = interrupted(ctx, err); err == nil {
			state.Ahead, state.Behind = &counts.Ahead, &counts.Behind
		}
		return state
	}

	ahead, err := u.Git.Count(ctx, span)
	if state.Err = interrupted(ctx, err); err == nil {
		state.Ahead = &ahead
	}
	return state
}

// ----------------------------------------------------------------------------
// Helpers

// interrupted is err when it is the read's interruption, which cancels the node; a count git cannot make otherwise leaves the node not counted rather than unreadable.
func interrupted(ctx context.Context, err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}

	return ctx.Err()
}
