package sync_test

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
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const (
	featA       = "feat/a"
	featB       = "feat/b"
	featC       = "feat/c"
	solo        = "feat/solo"
	dirty       = "feat/dirty"
	dirtyKid    = "feat/dirty-kid"
	busy        = "feat/busy"
	gone        = "feat/gone"
	orphan      = "feat/orphan" // its parent was deleted
	raw         = "raw"         // a worktree wtm did not create: no record
	projectToml = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n"
)

var (
	createdAt  = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	errRefused = errors.New("fatal: the rebase refused")
)

type world struct {
	root     string
	main     string
	stateDir string
	repo     git.Repo
	meta     store.Meta
	usecase  sync.UseCase
}

// newWorld is a repository whose main moved on since its stack main → feat/a → feat/b → feat/c and feat/solo were created and pushed, with feat/dirty → feat/dirty-kid on main, feat/dirty holding an uncommitted change, a worktree in the middle of a rebase, one whose directory is gone, one whose parent was deleted, one wtm did not create and one on no branch.
func newWorld(t *testing.T) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	branches := []string{featA, featB, featC, solo, dirty, dirtyKid, gone, orphan, raw}
	worktrees := []worktree.Worktree{{Path: linkedPath(root, busy), State: worktree.Rebasing{Branch: busy}}, {Path: linkedPath(root, "detached"), State: worktree.Detached{}}}
	for _, branch := range branches {
		worktrees = append(worktrees, worktree.Worktree{Path: linkedPath(root, branch), State: worktree.OnBranch{Branch: branch}})
	}
	repo := gittest.Build(t, gittest.FakeParams{Main: main, Branch: "main", Remote: []string{"main"}, Worktrees: worktrees, Gone: []string{linkedPath(root, gone)}})
	stateDir := filepath.Join(main, ".git", "wtm")
	writeFile(t, filepath.Join(stateDir, "config.toml"), projectToml)

	w := world{root: root, main: main, stateDir: stateDir, repo: repo, meta: store.Meta{StateDir: stateDir}}
	for i, parent := range [][2]string{{featA, "main"}, {featB, featA}, {featC, featB}, {solo, "main"}, {dirty, "main"}, {dirtyKid, dirty}, {busy, "main"}, {gone, "main"}, {orphan, "feat/deleted"}} {
		w.record(t, parent[0], worktree.Meta{SourceBranch: parent[1], CreatedAt: createdAt.Add(time.Duration(i) * time.Second).Format(time.RFC3339)})
	}
	w.commit(t, featA, "a.txt", "a")
	w.startFrom(t, featB, featA)
	w.commit(t, featB, "b.txt", "b")
	w.startFrom(t, featC, featB)
	w.commit(t, featC, "c.txt", "c")
	w.commit(t, solo, "solo.txt", "solo")
	for _, branch := range []string{featA, featB, featC, solo} {
		gittest.Git(t, main, "push", "-q", "origin", branch)
	}
	w.commit(t, "main", "main.txt", "moved on")
	gittest.Git(t, main, "push", "-q", "origin", "main")
	writeFile(t, filepath.Join(w.at(dirty), "wip.txt"), "work in progress\n")

	ids := 0
	w.usecase = sync.UseCase{
		Git:        repo,
		Meta:       w.meta,
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

// refusingRebase rebases as git does but refuses the rebase in the worktree at path.
type refusingRebase struct {
	git.Repo
	path string
}

func (r refusingRebase) Rebase(ctx context.Context, rebase worktree.Rebase) (worktree.Rebased, error) {
	if rebase.Path == r.path {
		return worktree.Rebased{}, errRefused
	}
	return r.Repo.Rebase(ctx, rebase)
}

// A seeded is one worktree of facts built by hand: its branch and its parent ("" for none), the commits of its parent it lacks, its uncommitted changes, whether its directory is gone, where it stands against origin's branch (none when off origin) and the commits of origin it never held, and its state, on its branch by default.
type seeded struct {
	branch    string
	parent    string
	behind    int
	changes   int
	missing   bool
	origin    worktree.AheadBehind
	offOrigin bool
	never     int
	state     worktree.State
}

// factsOf are facts as Observe would read them of worktrees, the first one the main worktree, on main.
func factsOf(worktrees ...seeded) sync.Facts {
	facts := sync.Facts{Checks: map[string]sync.Check{}, Located: map[string]string{}, Records: map[string]worktree.Meta{}, RecordErrs: map[string]error{}}
	facts.Config.Worktrees.BaseBranch = "main"
	facts.Divergence = map[string]worktree.AheadBehind{}
	for i, s := range worktrees {
		state := s.state
		if state == nil {
			state = worktree.OnBranch{Branch: s.branch}
		}
		facts.Worktrees = append(facts.Worktrees, worktree.Worktree{Path: "/repo/" + worktree.PathSegment(s.branch), Main: i == 0, State: state})
		facts.Branches.Local = append(facts.Branches.Local, worktree.Branch{Name: s.branch, Tip: s.branch + "-local"})
		if !s.offOrigin {
			facts.Branches.Remote = append(facts.Branches.Remote, worktree.Branch{Name: s.branch, Tip: s.branch + "-origin"})
			facts.Divergence[s.branch] = s.origin
		}
		if s.parent != "" {
			facts.Records[s.branch] = worktree.Meta{SourceBranch: s.parent, CreatedAt: createdAt.Add(time.Duration(i) * time.Second).Format(time.RFC3339)}
		}
		facts.Checks[s.branch] = sync.Check{Missing: s.missing, Changes: s.changes, Behind: s.behind, Never: s.never, Fork: worktree.Fork{Commit: s.parent + "-fork", Own: 1}}
	}
	facts.Stack = worktree.StackOf(facts.Worktrees, facts.Records, "main")
	return facts
}
