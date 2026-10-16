package create_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/command/create"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/dispatchtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

const (
	branch   = "feat/x"
	other    = "feat/y"
	existing = "feature"
	remote   = "remote-only"
	mainEnv  = "PORT=3000\n"
	project  = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n\n[env]\nstrategy = \"main\"\n\n[[env.file]]\ntarget = \".env\"\n\n[hooks]\non_create = []\n"
	jobs     = "[[job]]\nname = \"web\"\nkind = \"service\"\ncmd = \"npm start\"\nports = { PORT = 3000 }\n\n[[env_port]]\nfile = \".env\"\nkey = \"PORT\"\njob = \"web\"\nport = \"PORT\"\n"
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
	repo     git.ReadWriter
	disk     storetest.Disk
	bench    *dispatchtest.Bench
	ports    create.Ports
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
	stateDir := filepath.Join(main, ".git", domain.StateDirName)
	writeFile(t, filepath.Join(stateDir, domain.ConfigFileName), configOf(s))
	if s.Run != "" {
		writeFile(t, filepath.Join(stateDir, domain.RunFileName), s.Run)
	}
	if !s.NoEnv {
		writeFile(t, filepath.Join(main, ".env"), mainEnv)
	}
	disk := storetest.OnDisk(stateDir)
	return world{
		root:     root,
		main:     main,
		stateDir: stateDir,
		trees:    filepath.Join(root, ".trees"),
		repo:     repo,
		disk:     disk,
		bench:    dispatchtest.New(t, dispatchtest.BenchParams{StateDir: stateDir}),
		ports: create.Ports{
			Git: repo, Meta: disk.Meta, Config: disk.Config, Env: disk.Env,
			Dir: main, StateDir: stateDir, Now: func() time.Time { return createdAt },
		},
	}
}

func linkedUnder(root string, branches []string) []git.Worktree {
	worktrees := make([]git.Worktree, 0, len(branches))
	for _, name := range branches {
		worktrees = append(worktrees, git.Worktree{Path: linkedPath(root, name), State: git.OnBranch{Branch: name}})
	}
	return worktrees
}

func linkedPath(root, branch string) string {
	return filepath.Join(root, "wt-"+rules.SanitizeBranchName(branch))
}

func configOf(s seed) string {
	if s.Config != "" {
		return s.Config
	}
	return project
}

func commitsOf(s seed) map[git.Ref][]string {
	commits := map[git.Ref][]string{
		git.LocalBranch{Name: existing}:  {"feature work"},
		git.RemoteBranch{Name: existing}: {"feature work"},
		git.RemoteBranch{Name: remote}:   {"remote work"},
	}
	if s.Behind {
		commits[git.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	if s.Diverged {
		commits[git.LocalBranch{Name: "main"}] = []string{"local main"}
		commits[git.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	return commits
}

func (w world) path(name string) string {
	return filepath.Join(w.trees, rules.SanitizeBranchName(name))
}
