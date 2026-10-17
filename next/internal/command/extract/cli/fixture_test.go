package cli

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
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
	Clean      bool
	Behind     bool // origin's main is ahead of main
	Isolates   bool // run.toml declares a port, so the isolation is asked
	Conflicts  map[string]worktree.ConflictKind
	NoConfig   bool
	OnCreate   []project.Hook
	RunHooks   func(ctx context.Context, chain env.HookChain) error
	GitFails   map[string]error
	StoreFails map[string]error
}

type world struct {
	git   *gittest.Fake
	store *storetest.Fake
	seed  seed
}

func newWorld(s seed) world {
	changes := map[string][]worktree.Change{sourcePath: slices.Clone(changed)}
	if s.Clean {
		changes = nil
	}
	config := &project.Config{
		Worktrees: project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch},
		Env:       project.Env{Strategy: project.EnvExample},
		OnCreate:  s.OnCreate,
	}
	if s.NoConfig {
		config = nil
	}
	commits := map[worktree.Ref][]string{}
	if s.Behind {
		commits[worktree.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	return world{
		git: gittest.NewFake(gittest.FakeParams{
			Main:   mainPath,
			Branch: "main",
			Remote: []string{"main"},
			Worktrees: []worktree.Worktree{
				{Path: sourcePath, State: worktree.OnBranch{Branch: source}},
				{Path: otherPath, State: worktree.OnBranch{Branch: other}},
			},
			Commits:   commits,
			Changes:   changes,
			Conflicts: s.Conflicts,
			Fail:      s.GitFails,
		}),
		store: storetest.NewFake(storetest.FakeParams{Config: config, Fail: s.StoreFails}),
		seed:  s,
	}
}

func (w world) open(context.Context) (Project, error) {
	return Project{UseCase: w.usecase(), Cwd: mainPath}, nil
}

func (w world) usecase() extract.UseCase {
	runHooks := w.seed.RunHooks
	if runHooks == nil {
		runHooks = func(context.Context, env.HookChain) error { return nil }
	}
	return extract.UseCase{
		Git:     w.git,
		Config:  stored{Fake: w.store, isolates: w.seed.Isolates},
		Answers: w.store,
		Meta:    w.store,
		Shield:  shell.Shield,
		Operations: operation.Service{
			Locker:    locker{},
			Publisher: locker{},
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return createdAt },
		},
		Provision: provision.Service{
			Git:        w.git,
			Meta:       w.store,
			EnvFiles:   w.store,
			RunHooks:   runHooks,
			PublicPort: func(context.Context, int) int { return 0 },
			Shield:     shell.Shield,
			Now:        func() time.Time { return createdAt },
			LeftBehind: cleanLine,
		},
		MainPath: mainPath,
		StateDir: stateDir,
	}
}

type stored struct {
	*storetest.Fake
	isolates bool
}

func (s stored) Load() (project.Config, project.Global, error) {
	config, found := s.Config()
	if !found {
		return project.Config{}, project.Global{}, project.ErrConfigNotFound
	}
	return config, project.Global{}, nil
}

func (s stored) LoadRun() (project.Run, error) {
	if s.isolates {
		return project.Run{EnvPorts: []project.EnvPortLink{{}}}, nil
	}
	return project.Run{}, nil
}

type locker struct{}

func (locker) Take(domain.Operation) (operation.Hold, error) { return held{}, nil }
func (locker) Open(domain.Operation) domain.Report           { return func(domain.Event) {} }

type held struct{}

func (held) Grow([]domain.Lock) (bool, error) { return false, nil }
func (held) Protected() bool                  { return true }
func (held) Release()                         {}

func hookFails(context.Context, env.HookChain) error {
	return errors.New("hook failed: exit status 3")
}

// previewing is a use case whose every preview has a fingerprint of its own and whose applies end on errs in turn; it records the fingerprint each apply was held to.
type previewing struct {
	useCase
	previews int
	errs     []error
	held     []string
}

func (p *previewing) Run(ctx context.Context, req extract.Request, opts domain.RunOptions) (extract.Outcome, error) {
	if opts.DryRun {
		p.previews++
		outcome, err := p.useCase.Run(ctx, req, opts)
		outcome.Fingerprint = fmt.Sprintf("preview-%d", p.previews)
		return outcome, err
	}

	p.held = append(p.held, opts.ExpectFingerprint)
	err := p.errs[0]
	p.errs = p.errs[1:]
	return extract.Outcome{}, err
}

// enters is a terminal on which the user keeps pressing enter.
type enters struct{}

func (enters) Read(p []byte) (int, error) {
	time.Sleep(time.Millisecond)
	p[0] = '\r'
	return 1, nil
}

// settled asks to move two files from the source to the other worktree: every step but the mode is given.
func settled() extract.Request {
	return extract.Request{Source: source, Files: []string{"apps/api/a.txt", "apps/api/new.txt"}, To: other}
}

func cleanLine(branch string) []string { return []string{"wtm", "clean", branch, "--force"} }
