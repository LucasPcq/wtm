// Package tree reads the worktrees of a repository as the forest of their stacks, each under the branch it was created from, with what each needs next: a read, it changes nothing.
package tree

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "tree"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

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
	return Outcome{Items: items, Warnings: warnings, Interrupted: facts.Interrupted}, nil
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

	// The stack comes first: a worktree is counted against its parent. One whose record does not read has no known parent: a root.
	facts := Facts{Repository: repo}
	facts.Records, facts.RecordErrs = repository.Records(u.Meta, repo.Live())
	facts.Stack = worktree.StackOf(repo.Worktrees, facts.Records, repo.Config.Worktrees.BaseBranch)

	nodes := facts.nodes()
	var read []State
	facts.Asked = repository.Ask(ctx, repository.AskParams{Jobs: u.Jobs, PullRequests: u.PullRequests, Branches: repo.Live(), WithPRs: req.WithPRs}, func() {
		read = repository.Each(nodes, func(w worktree.Worktree) State { return u.state(ctx, facts, w) })
	})

	facts.States = make(map[string]State, len(nodes))
	for i, w := range nodes {
		facts.States[w.Branch()] = read[i]
	}
	facts.Behind = repository.Behind(ctx, u.Git, repo, facts.PullRequests, nodes)
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

	counts, err := u.Git.AheadBehindOf(ctx, worktree.Span{Branch: w.Branch(), Base: against})
	state.Err = repository.Interrupted(ctx, err)
	if err != nil {
		return state
	}

	// A root has no parent to fall behind: only what it has past the base is counted.
	state.Ahead = &counts.Ahead
	if _, child := facts.Stack.Parent(w.Branch()); child {
		state.Behind = &counts.Behind
	}
	return state
}
