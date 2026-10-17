package app_test

import (
	"cmp"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/operation"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

const (
	branchName  = "feat/x"
	projectToml = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n\n[env]\nstrategy = \"main\"\n\n[[env.file]]\ntarget = \".env\"\n"
	jobsToml    = "[[job]]\nname = \"web\"\nkind = \"service\"\ncmd = \"npm start\"\nports = { PORT = 3000 }\n\n[[env_port]]\nfile = \".env\"\nkey = \"PORT\"\njob = \"web\"\nport = \"PORT\"\n"
)

var clock = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

type counted struct {
	git.Repo
	observed *int
}

func (c counted) Worktrees(ctx context.Context) ([]worktree.Worktree, error) {
	*c.observed++
	return c.Repo.Worktrees(ctx)
}

type world struct {
	root      string
	main      string
	stateDir  string
	repo      git.Repo
	published *recorder
	deps      app.Deps
}

type seed struct {
	Config string
	Run    string
	Behind bool
}

func newWorld(t *testing.T, s seed) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	commits := map[branch.Ref][]string{}
	if s.Behind {
		commits[branch.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	repo := gittest.Build(t, gittest.FakeParams{Main: main, Branch: "main", Local: []string{"feature"}, Remote: []string{"main"}, Commits: commits})
	stateDir := filepath.Join(main, ".git", "wtm")
	writeFile(t, filepath.Join(stateDir, "config.toml"), cmp.Or(s.Config, projectToml))
	if s.Run != "" {
		writeFile(t, filepath.Join(stateDir, "run.toml"), s.Run)
	}
	writeFile(t, filepath.Join(main, ".env"), "PORT=3000\n")
	events := &recorder{}
	ids := 0
	meta := store.Meta{StateDir: stateDir}
	return world{
		root:      root,
		main:      main,
		stateDir:  stateDir,
		repo:      repo,
		published: events,
		deps: app.Deps{
			Create: create.Deps{
				Git: repo,
				LoadConfig: func() (project.Config, project.Global, error) {
					return config.Load(config.LoadParams{StateDir: stateDir, GlobalPath: filepath.Join(root, "no-global.toml")})
				},
				LoadRun: func() (run.Config, error) { return config.LoadRun(stateDir) },
				Remember: func(ctx context.Context, answers map[string]string) (wtm.Written, error) {
					return config.Remember(ctx, config.RememberParams{StateDir: stateDir, Answers: answers})
				},
				Provision: provision.Deps{
					Git:        repo,
					Meta:       meta,
					EnvFiles:   store.EnvFiles{},
					RunHooks:   shell.RunHooks,
					PublicPort: func(context.Context, int) int { return 0 },
					Shield:     shell.Shield,
					Now:        func() time.Time { return clock },
				},
				MainPath: main,
				StateDir: stateDir,
			},
			TakeLocks: func(op operation.Operation) (app.Hold, error) { return oplock.Locks{StateDir: stateDir}.Take(op) },
			Publisher: func(operation.Operation) wtm.Report { return events.add },
			NewOpID: func() string {
				ids++
				return "op-" + string(rune('0'+ids))
			},
			Now: func() time.Time { return clock },
		},
	}
}

func (w world) path(name string) string {
	return filepath.Join(w.root, ".trees", branch.PathSegment(name))
}

const (
	envHelperState = "WTM_NEXT_TEST_LOCK_STATE"
	readyLine      = "holding"
)

type unprotected struct{}

func (unprotected) Grow([]operation.Lock) (bool, error) { return false, nil }
func (unprotected) Protected() bool                     { return false }
func (unprotected) Release()                            {}
