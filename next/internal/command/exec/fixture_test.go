package exec_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const (
	featA       = "feat/a"
	featB       = "feat/b"
	numbered    = "feat/numbered"
	unnumbered  = "feat/unnumbered"
	gone        = "feat/gone"
	projectToml = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n"
	// runToml declares a compose job, so the hooks of an isolated worktree read its run environment, and a port.
	runToml = "[[job]]\nname = \"db\"\ncmd = \"docker compose up db\"\nports = { DB_PORT = 5432 }\n"
)

var startedAt = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

type world struct {
	root     string
	main     string
	stateDir string
	meta     store.Meta
	usecase  exec.UseCase
}

// newWorld is a repository with worktrees in every state exec decides on: two plain ones, one on no branch, one whose directory is gone, and two isolated ones, numbered and not; run.toml declares a compose job and a port.
func newWorld(t *testing.T) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	repo := gittest.Build(t, gittest.FakeParams{
		Main:   main,
		Branch: "main",
		Worktrees: []worktree.Worktree{
			{Path: linkedPath(root, featA), State: worktree.OnBranch{Branch: featA}},
			{Path: linkedPath(root, featB), State: worktree.OnBranch{Branch: featB}},
			{Path: linkedPath(root, "detached"), State: worktree.Detached{}},
			{Path: linkedPath(root, numbered), State: worktree.OnBranch{Branch: numbered}},
			{Path: linkedPath(root, unnumbered), State: worktree.OnBranch{Branch: unnumbered}},
			{Path: linkedPath(root, gone), State: worktree.OnBranch{Branch: gone}},
		},
		Gone: []string{linkedPath(root, gone)},
	})
	stateDir := filepath.Join(main, ".git", "wtm")
	writeFile(t, filepath.Join(stateDir, "config.toml"), projectToml)
	writeFile(t, filepath.Join(stateDir, "run.toml"), runToml)
	meta := store.Meta{StateDir: stateDir}
	w := world{root: root, main: main, stateDir: stateDir, meta: meta}
	w.record(t, numbered, worktree.Meta{SourceBranch: "main", Ordinal: 2, Isolation: "isolated"})
	w.record(t, unnumbered, worktree.Meta{SourceBranch: "main", Isolation: "isolated"})

	ids := 0
	w.usecase = exec.UseCase{
		Git:        repo,
		Meta:       meta,
		EnvFiles:   store.EnvFiles{},
		RunHooks:   shell.RunHooks,
		Syntax:     shell.Syntax,
		StateDir:   stateDir,
		CPUs:       8,
		Now:        time.Now,
		Repository: repository.Service{Config: config.Project{StateDir: stateDir, GlobalPath: filepath.Join(root, "no-global.toml")}, Git: repo, MainPath: main},
		Operations: operation.Service{
			Locker:    locks{oplock.Locks{StateDir: stateDir}},
			Publisher: &operationtest.Recorder{},
			NewOpID: func() string {
				ids++
				return "op-" + string(rune('0'+ids))
			},
			Now: func() time.Time { return startedAt },
		},
	}
	return w
}

func linkedPath(root, name string) string {
	return filepath.Join(root, "wt-"+worktree.PathSegment(name))
}

type locks struct {
	oplock.Locks
}

func (l locks) Take(op domain.Operation) (operation.Hold, error) {
	return l.Locks.Take(op)
}
