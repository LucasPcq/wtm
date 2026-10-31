package cli

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

type sessionParams struct {
	App     form.UseCase[sync.Request, sync.Questions, sync.Outcome]
	Request sync.Request
}

// The steps of sync's form, in the order asked, then the recap.
const (
	stepWorktrees = iota
	stepAll
	stepDescendants
	stepForce
	stepKeepConflict
	stepFFParents
	stepPush
	stepNoFetch
	stepRecap
)

const (
	mainPath    = "/repo/main"
	featA       = "feat/a"
	featB       = "feat/b"
	featC       = "feat/c"
	featX       = "feat/x"
	dirty       = "feat/dirty"
	fingerprint = "0123456789abcdef01234567"
)

var createdAt = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

// A seed is what sets a world apart: the files the rebase of each branch stops on a conflict in.
type seed struct {
	Conflicts map[string][]string
}

type world struct {
	git    *gittest.Fake
	store  *storetest.Fake
	config *configtest.Fake
	locks  *operationtest.Locks
}

func pathOf(branch string) string {
	return "/repo/wt-" + worktree.PathSegment(branch)
}

// newWorld is a repository whose main moved on since its stack main → feat/a → feat/b → feat/c, pushed, and feat/x were created from it, with feat/dirty on main holding an uncommitted change.
func newWorld(s seed) world {
	return world{
		git: gittest.NewFake(gittest.FakeParams{
			Main:   mainPath,
			Branch: "main",
			Remote: []string{"main", featA, featB, featC},
			Worktrees: []worktree.Worktree{
				{Path: pathOf(featA), State: worktree.OnBranch{Branch: featA}},
				{Path: pathOf(featB), State: worktree.OnBranch{Branch: featB}},
				{Path: pathOf(featC), State: worktree.OnBranch{Branch: featC}},
				{Path: pathOf(featX), State: worktree.OnBranch{Branch: featX}},
				{Path: pathOf(dirty), State: worktree.OnBranch{Branch: dirty}},
			},
			Dirty:   []string{pathOf(dirty)},
			Changes: map[string][]worktree.Change{pathOf(dirty): {{Path: "wip.txt", Status: worktree.ChangeUntracked}}},
			Commits: map[worktree.Ref][]string{
				worktree.LocalBranch{Name: "main"}:  {"moved on"},
				worktree.RemoteBranch{Name: "main"}: {"moved on"},
				worktree.LocalBranch{Name: featA}:   {"a"},
				worktree.RemoteBranch{Name: featA}:  {"a"},
				worktree.LocalBranch{Name: featB}:   {"a", "b"},
				worktree.RemoteBranch{Name: featB}:  {"a", "b"},
				worktree.LocalBranch{Name: featC}:   {"a", "b", "c"},
				worktree.RemoteBranch{Name: featC}:  {"a", "b", "c"},
				worktree.LocalBranch{Name: featX}:   {"x"},
			},
			Rebases: s.Conflicts,
		}),
		store: storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{
			featA: {SourceBranch: "main", CreatedAt: createdAt.Format(time.RFC3339)},
			featB: {SourceBranch: featA, CreatedAt: createdAt.Add(time.Second).Format(time.RFC3339)},
			featC: {SourceBranch: featB, CreatedAt: createdAt.Add(2 * time.Second).Format(time.RFC3339)},
			featX: {SourceBranch: "main", CreatedAt: createdAt.Add(3 * time.Second).Format(time.RFC3339)},
			dirty: {SourceBranch: "main", CreatedAt: createdAt.Add(4 * time.Second).Format(time.RFC3339)},
		}}),
		config: configtest.NewFake(configtest.FakeParams{Config: &project.Config{Worktrees: project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch}}}),
		locks:  &operationtest.Locks{},
	}
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: mainPath, ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (sync.UseCase, error) {
	return w.usecase(), nil
}

func (w world) usecase() sync.UseCase {
	return sync.UseCase{
		Git:        w.git,
		Meta:       w.store,
		Shield:     shell.Shield,
		Repository: repository.Service{Config: w.config, Git: w.git, MainPath: mainPath, Init: &domain.FollowUp{Command: "init"}},
		Operations: operation.Service{
			Locker:    w.locks,
			Publisher: w.locks,
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return createdAt },
		},
		Provision: provision.Service{Git: w.git},
	}
}
