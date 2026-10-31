package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

const mainPath = "/repo/main"

var createdAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

// pullRequests are the open pull requests of the repository: one whose branch only origin has, a draft, a fork's and one whose branch a worktree holds.
var pullRequests = []worktree.PullRequest{
	{Number: 1, URL: "https://github.com/acme/acme/pull/1", Branch: "feat/remote", State: worktree.PullRequestOpen, Title: "Add the remote feature", Base: "main", Author: "alice"},
	{Number: 2, URL: "https://github.com/acme/acme/pull/2", Branch: "feat/draft", State: worktree.PullRequestOpen, Title: "A draft with a title long enough to be cut in the list of the picker", Base: "main", Author: "bob", Draft: true},
	{Number: 3, URL: "https://github.com/acme/acme/pull/3", Branch: "main", State: worktree.PullRequestOpen, Title: "From a fork", Base: "main", Author: "stranger", Fork: true},
	{Number: 4, URL: "https://github.com/acme/acme/pull/4", Branch: "feat/held", State: worktree.PullRequestOpen, Title: "Held by a worktree", Base: "main", Author: "alice"},
}

type seed struct {
	GitHub   error // what asking GitHub fails with, nil when it answers
	OnCreate []project.Hook
}

type world struct {
	git    *gittest.Fake
	store  *storetest.Fake
	config *configtest.Fake
	locks  *operationtest.Locks
	seed   seed
}

func newWorld(s seed) world {
	config := project.Config{
		Worktrees: project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch},
		Env:       project.Env{Strategy: project.EnvExample},
		OnCreate:  s.OnCreate,
	}
	return world{
		git: gittest.NewFake(gittest.FakeParams{
			Main:      mainPath,
			Branch:    "main",
			Remote:    []string{"main", "feat/remote", "feat/draft", "feat/held"},
			Worktrees: []worktree.Worktree{{Path: "/elsewhere/feat-held", State: worktree.OnBranch{Branch: "feat/held"}}},
		}),
		store:  storetest.NewFake(storetest.FakeParams{}),
		config: configtest.NewFake(configtest.FakeParams{Config: &config}),
		locks:  &operationtest.Locks{},
		seed:   s,
	}
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: mainPath, ProjectDir: mainPath}
}

func (w world) command(streams *output.Streams) *cobra.Command {
	return New(streams, w.env(), w.open, formtest.CommandLine)
}

func (w world) open(context.Context, string) (checkout.UseCase, error) {
	return w.usecase(), nil
}

func (w world) usecase() checkout.UseCase {
	return checkout.UseCase{
		Git:          w.git,
		PullRequest:  w.pullRequest,
		PullRequests: w.pullRequests,
		Repository:   repository.Service{Config: w.config, Git: w.git, MainPath: mainPath, Init: &domain.FollowUp{Command: "init"}},
		Operations: operation.Service{
			Locker:    w.locks,
			Publisher: w.locks,
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return createdAt },
		},
		Provision: provision.Service{
			Git:         w.git,
			Meta:        w.store,
			EnvFiles:    w.store,
			StateDir:    mainPath + "/.git/wtm",
			Answers:     w.config,
			RunHooks:    hooks(w.seed.OnCreate),
			PublicPort:  func(context.Context, int) int { return 0 },
			Shield:      shell.Shield,
			Now:         func() time.Time { return createdAt },
			LeftBehind:  func(string) domain.FollowUp { return domain.FollowUp{Command: "clean"} },
			Settle:      func(string) domain.FollowUp { return domain.FollowUp{Command: "env"} },
			ProxyStatus: func() domain.FollowUp { return domain.FollowUp{Command: "run proxy status"} },
		},
	}
}

func (w world) pullRequest(_ context.Context, number int) (worktree.PullRequest, error) {
	if w.seed.GitHub != nil {
		return worktree.PullRequest{}, w.seed.GitHub
	}
	for _, pr := range pullRequests {
		if pr.Number == number {
			return pr, nil
		}
	}
	return worktree.PullRequest{}, fmt.Errorf("pull request #%d: %w", number, worktree.ErrPullRequestNotFound)
}

func (w world) pullRequests(context.Context, worktree.PullRequestFilter) ([]worktree.PullRequest, error) {
	return pullRequests, w.seed.GitHub
}

// hooks runs the on_create hooks as succeeding, but one that runs false.
func hooks(onCreate []project.Hook) func(context.Context, env.HookChain) error {
	return func(context.Context, env.HookChain) error {
		for _, hook := range onCreate {
			if hook.Cmd == "false" {
				return fmt.Errorf("exit status 1")
			}
		}
		return nil
	}
}
