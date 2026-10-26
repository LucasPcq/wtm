package fastforward_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const (
	behind      = "feat/behind"
	stale       = "feat/stale"
	ahead       = "feat/ahead"
	diverged    = "feat/diverged"
	dirty       = "feat/dirty"
	dirtyLatest = "feat/dirty-latest"
	latest      = "feat/latest"
	unpushed    = "feat/unpushed"
	rebasing    = "feat/rebasing"
	bisecting   = "feat/bisecting"
	picking     = "feat/picking"
	gone        = "feat/gone"
	loose       = "feat/loose"
	projectToml = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n"
)

var (
	createdAt      = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	errRefused     = errors.New("error: The following untracked working tree files would be overwritten by merge")
	errUnreachable = errors.New("fatal: unable to access origin")
)

type world struct {
	root     string
	main     string
	stateDir string
	repo     git.Repo
	usecase  fastforward.UseCase
}

// newWorld is a repository with a worktree in every standing fast-forward decides on: behind origin, behind it with origin's branch not fetched yet, ahead, diverged, behind with uncommitted changes, up to date with them, up to date, never pushed, behind in the middle of a rebase, a bisect and a cherry-pick, behind with its directory gone, and on no branch; the main worktree is behind too, and a branch behind is checked out nowhere.
func newWorld(t *testing.T) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	upstream := []string{"upstream"}
	repo := gittest.Build(t, gittest.FakeParams{
		Main:   main,
		Branch: "main",
		Local:  []string{loose},
		Remote: []string{"main", behind, stale, ahead, diverged, dirty, dirtyLatest, latest, rebasing, bisecting, picking, gone, loose},
		Worktrees: []worktree.Worktree{
			{Path: linkedPath(root, behind), State: worktree.OnBranch{Branch: behind}},
			{Path: linkedPath(root, stale), State: worktree.OnBranch{Branch: stale}},
			{Path: linkedPath(root, ahead), State: worktree.OnBranch{Branch: ahead}},
			{Path: linkedPath(root, diverged), State: worktree.OnBranch{Branch: diverged}},
			{Path: linkedPath(root, dirty), State: worktree.OnBranch{Branch: dirty}},
			{Path: linkedPath(root, dirtyLatest), State: worktree.OnBranch{Branch: dirtyLatest}},
			{Path: linkedPath(root, latest), State: worktree.OnBranch{Branch: latest}},
			{Path: linkedPath(root, unpushed), State: worktree.OnBranch{Branch: unpushed}},
			{Path: linkedPath(root, rebasing), State: worktree.Rebasing{Branch: rebasing}},
			{Path: linkedPath(root, bisecting), State: worktree.Bisecting{Branch: bisecting}},
			{Path: linkedPath(root, picking), State: worktree.OnBranch{Branch: picking}, Midway: worktree.MidwayCherryPick},
			{Path: linkedPath(root, gone), State: worktree.OnBranch{Branch: gone}},
			{Path: linkedPath(root, "detached"), State: worktree.Detached{}},
		},
		Gone:  []string{linkedPath(root, gone)},
		Dirty: []string{linkedPath(root, dirty), linkedPath(root, dirtyLatest)},
		Commits: map[worktree.Ref][]string{
			worktree.RemoteBranch{Name: "main"}:    upstream,
			worktree.RemoteBranch{Name: behind}:    upstream,
			worktree.RemoteBranch{Name: dirty}:     upstream,
			worktree.RemoteBranch{Name: rebasing}:  upstream,
			worktree.RemoteBranch{Name: bisecting}: upstream,
			worktree.RemoteBranch{Name: picking}:   upstream,
			worktree.RemoteBranch{Name: gone}:      upstream,
			worktree.RemoteBranch{Name: loose}:     upstream,
			worktree.LocalBranch{Name: ahead}:      {"local work"},
			worktree.LocalBranch{Name: diverged}:   {"local work"},
			worktree.RemoteBranch{Name: diverged}:  upstream,
		},
		Origin: map[string][]string{stale: {"upstream of stale"}},
	})
	stateDir := filepath.Join(main, ".git", "wtm")
	writeFile(t, filepath.Join(stateDir, "config.toml"), projectToml)

	ids := 0
	w := world{root: root, main: main, stateDir: stateDir, repo: repo}
	w.usecase = fastforward.UseCase{
		Git:        repo,
		Shield:     shell.Shield,
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
		Provision: provision.Service{Git: repo},
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

// refusingMerge fast-forwards as git does but refuses every merge in a worktree, as git refuses one an untracked file stands in the way of.
type refusingMerge struct {
	git.Repo
}

func (refusingMerge) MergeFFOnly(context.Context, worktree.Merge) error {
	return errRefused
}

// unfetchable fetches nothing: origin cannot be reached.
type unfetchable struct {
	git.Repo
}

func (unfetchable) Fetch(context.Context) error {
	return errUnreachable
}

func (unfetchable) FetchRef(context.Context, string) error {
	return errUnreachable
}
