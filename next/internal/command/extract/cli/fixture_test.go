package cli

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

// useCase is what the form calls of extract's use case, so a test can stand in for it.
type useCase = form.UseCase[extract.Request, extract.Questions, extract.Outcome]

type sessionParams struct {
	App     useCase
	Request extract.Request
	Options domain.RunOptions
}

// The steps of extract's form, in the order asked, then the recap.
const (
	stepSource = iota
	stepFiles
	stepTarget
	stepBranch
	stepFrom
	stepEnvFrom
	stepIsolation
	stepFastForward
	stepMode
	stepOnConflict
	stepRecap
)

const (
	mainPath    = "/repo/main"
	stateDir    = "/repo/main/.git/wtm"
	sourcePath  = "/repo/wt-feat-login"
	otherPath   = "/repo/wt-feat-other"
	source      = "feat/login"
	other       = "feat/other"
	split       = "feat/split"
	fingerprint = "0123456789abcdef01234567"
)

var createdAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

var changed = []worktree.Change{
	{Path: "apps/api/a.txt", Status: worktree.ChangeModified},
	{Path: "apps/api/new.txt", Status: worktree.ChangeUntracked},
	{Path: "DOC.md", Status: worktree.ChangeRenamed, OrigPath: "README.md"},
	{Path: "old.txt", Status: worktree.ChangeDeleted},
}

type seed struct {
	Env        uicli.Env // zero runs in the main worktree
	Clean      bool
	Behind     bool // origin's main is ahead of main
	Held       bool // origin's feat/login is ahead of it: the source holds the changes and is not fast-forwarded
	Isolates   bool // run.toml declares a port, so the isolation is asked
	Conflicts  map[string]worktree.ConflictKind
	Remembered map[string]string
	NoConfig   bool
	OnCreate   []project.Hook
	RunHooks   func(ctx context.Context, chain env.HookChain) error
	GitFails   map[string]error
	StoreFails map[string]error
}

type world struct {
	git    *gittest.Fake
	store  *storetest.Fake
	config *configtest.Fake
	locks  *operationtest.Locks
	seed   seed
	seen   *seen
}

// seen is what the use case was given: the directory it was opened from.
type seen struct {
	dirs []string
}

func (w world) env() uicli.Env {
	env := uicli.Env{Cwd: mainPath, ProjectDir: mainPath}
	if w.seed.Env.ProjectDir != "" {
		env = w.seed.Env
	}
	return env
}

func newWorld(s seed) world {
	changes := map[string][]worktree.Change{sourcePath: slices.Clone(changed)}
	if s.Clean {
		changes = nil
	}
	config := &project.Config{
		Worktrees:  project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch},
		Env:        project.Env{Strategy: project.EnvExample},
		OnCreate:   s.OnCreate,
		Remembered: s.Remembered,
	}
	if s.NoConfig {
		config = nil
	}
	var run project.Run
	if s.Isolates {
		run = project.Run{Declared: true, EnvPorts: []project.EnvPortLink{{}}}
	}
	commits := map[worktree.Ref][]string{}
	if s.Behind {
		commits[worktree.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	remote := []string{"main"}
	if s.Held {
		remote = append(remote, source)
		commits[worktree.RemoteBranch{Name: source}] = []string{"upstream feat/login"}
	}
	return world{
		git: gittest.NewFake(gittest.FakeParams{
			Main:   mainPath,
			Branch: "main",
			Remote: remote,
			Worktrees: []worktree.Worktree{
				{Path: sourcePath, State: worktree.OnBranch{Branch: source}},
				{Path: otherPath, State: worktree.OnBranch{Branch: other}},
			},
			Commits:   commits,
			Changes:   changes,
			Conflicts: s.Conflicts,
			Fail:      s.GitFails,
		}),
		store:  storetest.NewFake(storetest.FakeParams{Fail: s.StoreFails}),
		config: configtest.NewFake(configtest.FakeParams{Config: config, Run: run}),
		locks:  &operationtest.Locks{},
		seed:   s,
		seen:   &seen{},
	}
}

func (w world) open(_ context.Context, dir string) (extract.UseCase, error) {
	w.seen.dirs = append(w.seen.dirs, dir)
	return w.usecase(), nil
}

func (w world) usecase() extract.UseCase {
	runHooks := w.seed.RunHooks
	if runHooks == nil {
		runHooks = func(context.Context, env.HookChain) error { return nil }
	}
	return extract.UseCase{
		Git:        w.git,
		Repository: repository.Service{Config: w.config, Git: w.git, MainPath: mainPath, Init: &domain.FollowUp{Command: "init"}},
		Meta:       w.store,
		Shield:     shell.Shield,
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
			StateDir:    stateDir,
			Answers:     w.config,
			RunHooks:    runHooks,
			PublicPort:  func(context.Context, int) int { return 0 },
			Shield:      shell.Shield,
			Now:         func() time.Time { return createdAt },
			LeftBehind:  cleanLine,
			Settle:      settleLine,
			ProxyStatus: proxyStatusLine,
		},
	}
}

func hookFails(context.Context, env.HookChain) error {
	return errors.New("hook failed: exit status 3")
}

// previewing is a use case whose every preview has a fingerprint of its own and whose applies end on errs in turn.
type previewing = formtest.Previewing[extract.Request, extract.Questions, extract.Plan, extract.Detail]

// settled asks to move two files from the source to the other worktree: every step but the mode is given.
func settled() extract.Request {
	return extract.Request{Source: source, Files: []string{"apps/api/a.txt", "apps/api/new.txt"}, To: other}
}

func cleanLine(branch string) domain.FollowUp {
	return domain.FollowUp{Command: "clean", Answers: []domain.Answered{{Path: "worktree", Values: []string{branch}}, {Path: "force", Values: []string{"true"}}}}
}

func settleLine(branch string) domain.FollowUp {
	return domain.FollowUp{Command: "env", Answers: []domain.Answered{{Path: "worktree", Values: []string{branch}}}}
}

func proxyStatusLine() domain.FollowUp {
	return domain.FollowUp{Command: "run proxy status"}
}
