package create_test

import (
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
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const (
	branchName  = "feat/x"
	other       = "feat/y"
	existing    = "feature"
	remote      = "remote-only"
	mainEnv     = "PORT=3000\n"
	projectToml = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n\n[env]\nstrategy = \"main\"\n\n[[env.file]]\ntarget = \".env\"\n\n[hooks]\non_create = []\n"
	jobsToml    = "[[job]]\nname = \"web\"\nkind = \"service\"\ncmd = \"npm start\"\nports = { PORT = 3000 }\n\n[[env_port]]\nfile = \".env\"\nkey = \"PORT\"\njob = \"web\"\nport = \"PORT\"\n"
)

const remembering = projectToml + "\n[wizard.remembered]\nenv_strategy = \"example\"\nisolation = \"verbatim\"\nsource_update = \"ff\"\n"

var createdAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

type seed struct {
	Linked   []string
	Config   string
	Run      string
	NoEnv    bool
	Behind   bool
	Diverged bool
}

type world struct {
	root      string
	main      string
	stateDir  string
	trees     string
	repo      git.Repo
	meta      store.Meta
	published *recorder
	usecase   create.UseCase
}

func newWorld(t *testing.T, s seed) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	repo := gittest.Build(t, gittest.FakeParams{
		Main:      main,
		Branch:    "main",
		Local:     []string{existing},
		Remote:    []string{"main", existing, remote},
		Worktrees: linkedUnder(root, s.Linked),
		Commits:   commitsOf(s),
	})
	stateDir := filepath.Join(main, ".git", "wtm")
	writeFile(t, filepath.Join(stateDir, "config.toml"), configOf(s))
	if s.Run != "" {
		writeFile(t, filepath.Join(stateDir, "run.toml"), s.Run)
	}
	if !s.NoEnv {
		writeFile(t, filepath.Join(main, ".env"), mainEnv)
	}
	meta := store.Meta{StateDir: stateDir}
	projectConfig := config.Project{StateDir: stateDir, GlobalPath: filepath.Join(root, "no-global.toml")}
	published := &recorder{}
	ids := 0
	return world{
		root:      root,
		main:      main,
		stateDir:  stateDir,
		trees:     filepath.Join(root, ".trees"),
		repo:      repo,
		meta:      meta,
		published: published,
		usecase: create.UseCase{
			Git:    repo,
			Config: projectConfig,
			Operations: operation.Service{
				Locker:    locks{oplock.Locks{StateDir: stateDir}},
				Publisher: published,
				NewOpID: func() string {
					ids++
					return "op-" + string(rune('0'+ids))
				},
				Now: func() time.Time { return createdAt },
			},
			Provision: provision.Service{
				Git:        repo,
				Meta:       meta,
				EnvFiles:   store.EnvFiles{},
				StateDir:   stateDir,
				Answers:    projectConfig,
				RunHooks:   shell.RunHooks,
				PublicPort: func(context.Context, int) int { return 0 },
				Shield:     shell.Shield,
				Now:        func() time.Time { return createdAt },
				LeftBehind: cleanLine,
			},
			MainPath: main,
		},
	}
}

// linkedUnder are worktrees of branches, each at its linkedPath under root, outside the base path.
func linkedUnder(root string, branches []string) []worktree.Worktree {
	worktrees := make([]worktree.Worktree, 0, len(branches))
	for _, name := range branches {
		worktrees = append(worktrees, worktree.Worktree{Path: linkedPath(root, name), State: worktree.OnBranch{Branch: name}})
	}
	return worktrees
}

func linkedPath(root, name string) string {
	return filepath.Join(root, "wt-"+worktree.PathSegment(name))
}

func configOf(s seed) string {
	if s.Config != "" {
		return s.Config
	}
	return projectToml
}

// commitsOf are the commits each ref carries beyond main for the seed: origin's main ahead when Behind, and local main too when Diverged.
func commitsOf(s seed) map[worktree.Ref][]string {
	commits := map[worktree.Ref][]string{
		worktree.LocalBranch{Name: existing}:  {"feature work"},
		worktree.RemoteBranch{Name: existing}: {"feature work"},
		worktree.RemoteBranch{Name: remote}:   {"remote work"},
	}
	if s.Behind {
		commits[worktree.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	if s.Diverged {
		commits[worktree.LocalBranch{Name: "main"}] = []string{"local main"}
		commits[worktree.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	return commits
}

func (w world) path(name string) string {
	return filepath.Join(w.trees, worktree.PathSegment(name))
}

const (
	envHelperState = "WTM_NEXT_TEST_LOCK_STATE"
	readyLine      = "holding"
)

type locks struct {
	oplock.Locks
}

func (l locks) Take(op domain.Operation) (operation.Hold, error) {
	return l.Locks.Take(op)
}

// counted counts the snapshots a use case reads, to see how many times it plans.
type counted struct {
	git.Repo
	observed *int
}

func (c counted) Snapshot(ctx context.Context) (worktree.Snapshot, error) {
	*c.observed++
	return c.Repo.Snapshot(ctx)
}

// unprotected is a locker whose hold protects nothing, as when the lock files cannot be taken.
type unprotected struct{}

func (unprotected) Take(domain.Operation) (operation.Hold, error) { return unprotected{}, nil }
func (unprotected) Grow([]domain.Lock) (bool, error)              { return false, nil }
func (unprotected) Protected() bool                               { return false }
func (unprotected) Release()                                      {}

type answering struct {
	written domain.Written
	err     error
}

func (a answering) Remember(context.Context, map[string]string) (domain.Written, error) {
	return a.written, a.err
}

func cleanLine(branch string) []string { return []string{"wtm", "clean", branch, "--force"} }
