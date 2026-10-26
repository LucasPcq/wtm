package teardown_test

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

const (
	command = "sweep" // the command asking, whose codes teardown words
	plain   = "feat/plain"
	parent  = "feat/parent"
	child   = "feat/child"
	local   = "feat/local"   // its commit only feat/stacked holds
	stacked = "feat/stacked" // feat/local's commit, then one no other branch holds
	alone   = "feat/alone"   // a branch no worktree holds
)

var errRefused = errors.New("refused")

// world is a repository with a worktree per state a removal decides on, its records, and the service over them.
type world struct {
	git     *gittest.Fake
	store   *storetest.Fake
	service teardown.Service
	hooks   *[]env.HookChain
}

type seed struct {
	Gone      []string // worktrees whose directory was deleted behind git's back
	Fail      map[string]error
	StoreFail map[string]error
	RunHooks  func(context.Context, env.HookChain) error
}

func at(branch string) string {
	return "/repo/wt-" + worktree.PathSegment(branch)
}

func newWorld(s seed) world {
	git := gittest.NewFake(gittest.FakeParams{
		Main:   "/repo/main",
		Branch: "main",
		Local:  []string{alone},
		Worktrees: []worktree.Worktree{
			{Path: at(plain), State: worktree.OnBranch{Branch: plain}},
			{Path: at(parent), State: worktree.OnBranch{Branch: parent}},
			{Path: at(child), State: worktree.OnBranch{Branch: child}},
			{Path: at(local), State: worktree.OnBranch{Branch: local}},
			{Path: at(stacked), State: worktree.OnBranch{Branch: stacked}},
		},
		Commits: map[worktree.Ref][]string{
			worktree.LocalBranch{Name: local}:   {"local work"},
			worktree.LocalBranch{Name: stacked}: {"local work", "stacked work"},
		},
		Gone: s.Gone,
		Fail: s.Fail,
	})
	store := storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{
		plain:  {SourceBranch: "main"},
		parent: {SourceBranch: "main"},
		child:  {SourceBranch: parent},
		alone:  {SourceBranch: "main"},
	}, Fail: s.StoreFail})
	hooks := &[]env.HookChain{}
	runHooks := s.RunHooks
	if runHooks == nil {
		runHooks = func(_ context.Context, chain env.HookChain) error {
			*hooks = append(*hooks, chain)
			return nil
		}
	}
	return world{
		git:     git,
		store:   store,
		hooks:   hooks,
		service: teardown.Service{Git: git, Meta: store, EnvFiles: store, StateDir: "/repo/main/.git/wtm", Gone: repository.GoneIn(git), RunHooks: runHooks, Shield: func(ctx context.Context) (context.Context, func()) { return ctx, func() {} }},
	}
}

// leaving removes the worktree from git's list but fails as git does when files it cannot delete stay on disk.
type leaving struct {
	*gittest.Fake
}

func (l leaving) RemoveWorktree(ctx context.Context, path string) error {
	if err := l.Fake.RemoveWorktree(ctx, path); err != nil {
		return err
	}
	return errRefused
}

func hookFails(context.Context, env.HookChain) error {
	return &env.HookFailure{Cmd: "make down", Err: errRefused}
}

// onClean is a config whose on_clean hook is make down.
func onClean() project.Config {
	return project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}, OnClean: []project.Hook{{Cmd: "make down"}}}
}
