package reparent_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const (
	featA       = "feat/a"
	featB       = "feat/b"
	featC       = "feat/c"
	featX       = "feat/x"
	raw         = "raw"         // a worktree wtm did not create: no record
	topic       = "topic"       // a local branch no worktree holds
	remoteOnly  = "remote-only" // a branch only origin has
	projectToml = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n"
)

var createdAt = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

type world struct {
	root     string
	main     string
	stateDir string
	repo     git.Repo
	meta     store.Meta
	usecase  reparent.UseCase
}

// newWorld is a repository with a stack main → feat/a → feat/b → feat/c and feat/x on main, a worktree wtm did not create, one on no branch, a local branch no worktree holds and a branch only origin has.
func newWorld(t *testing.T) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	repo := gittest.Build(t, gittest.FakeParams{
		Main:   main,
		Branch: "main",
		Local:  []string{topic},
		Remote: []string{"main", remoteOnly},
		Worktrees: []worktree.Worktree{
			{Path: linkedPath(root, featA), State: worktree.OnBranch{Branch: featA}},
			{Path: linkedPath(root, featB), State: worktree.OnBranch{Branch: featB}},
			{Path: linkedPath(root, featC), State: worktree.OnBranch{Branch: featC}},
			{Path: linkedPath(root, featX), State: worktree.OnBranch{Branch: featX}},
			{Path: linkedPath(root, raw), State: worktree.OnBranch{Branch: raw}},
			{Path: linkedPath(root, "detached"), State: worktree.Detached{}},
		},
	})
	stateDir := filepath.Join(main, ".git", "wtm")
	writeFile(t, filepath.Join(stateDir, "config.toml"), projectToml)
	meta := store.Meta{StateDir: stateDir}
	w := world{root: root, main: main, stateDir: stateDir, repo: repo, meta: meta}
	for branch, parent := range map[string]string{featA: "main", featB: featA, featC: featB, featX: "main"} {
		w.record(t, branch, worktree.Meta{SourceBranch: parent, CreatedAt: createdAt.Format(time.RFC3339), Ordinal: 3})
	}

	ids := 0
	w.usecase = reparent.UseCase{
		Git:        repo,
		Meta:       meta,
		Repository: repository.Service{Config: config.Project{StateDir: stateDir, GlobalPath: filepath.Join(root, "no-global.toml")}, Git: repo, MainPath: main},
		Operations: operation.Service{
			Locker:    locks{oplock.Locks{StateDir: stateDir}},
			Publisher: &operationtest.Recorder{},
			NewOpID: func() string {
				ids++
				return "op-" + string(rune('0'+ids))
			},
			Now: func() time.Time { return createdAt },
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
