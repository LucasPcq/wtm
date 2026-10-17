package create_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
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
	root     string
	main     string
	stateDir string
	trees    string
	repo     git.Repo
	meta     store.Meta
	deps     create.Deps
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
	return world{
		root:     root,
		main:     main,
		stateDir: stateDir,
		trees:    filepath.Join(root, ".trees"),
		repo:     repo,
		meta:     meta,
		deps: create.Deps{
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
				Now:        func() time.Time { return createdAt },
			},
			MainPath: main,
			StateDir: stateDir,
		},
	}
}

func linkedUnder(root string, branches []string) []worktree.Worktree {
	worktrees := make([]worktree.Worktree, 0, len(branches))
	for _, name := range branches {
		worktrees = append(worktrees, worktree.Worktree{Path: linkedPath(root, name), State: worktree.OnBranch{Branch: name}})
	}
	return worktrees
}

func linkedPath(root, name string) string {
	return filepath.Join(root, "wt-"+branch.PathSegment(name))
}

func configOf(s seed) string {
	if s.Config != "" {
		return s.Config
	}
	return projectToml
}

func commitsOf(s seed) map[branch.Ref][]string {
	commits := map[branch.Ref][]string{
		branch.LocalBranch{Name: existing}:  {"feature work"},
		branch.RemoteBranch{Name: existing}: {"feature work"},
		branch.RemoteBranch{Name: remote}:   {"remote work"},
	}
	if s.Behind {
		commits[branch.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	if s.Diverged {
		commits[branch.LocalBranch{Name: "main"}] = []string{"local main"}
		commits[branch.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	return commits
}

func (w world) path(name string) string {
	return filepath.Join(w.trees, branch.PathSegment(name))
}
