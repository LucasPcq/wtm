package cli

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

type session = form.Session[reparent.Request, reparent.Questions, reparent.Outcome]

type sessionParams struct {
	App     form.UseCase[reparent.Request, reparent.Questions, reparent.Outcome]
	Request reparent.Request
}

// The steps of reparent's form, in the order asked, then the recap.
const (
	stepWorktrees = iota
	stepTo
	stepRecap
)

const (
	mainPath    = "/repo/main"
	featA       = "feat/a"
	featB       = "feat/b"
	featC       = "feat/c"
	featX       = "feat/x"
	fingerprint = "0123456789abcdef01234567"
)

var createdAt = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

type seed struct {
	Cwd       string // where the caller stands, the main worktree when empty
	WriteFail error
}

type world struct {
	git    *gittest.Fake
	store  *storetest.Fake
	config *configtest.Fake
	locks  *operationtest.Locks
	seed   seed
}

func pathOf(branch string) string {
	return "/repo/wt-" + worktree.PathSegment(branch)
}

// newWorld is a repository with a stack main → feat/a → feat/b → feat/c and feat/x on main.
func newWorld(s seed) world {
	var fail map[string]error
	if s.WriteFail != nil {
		fail = map[string]error{"Update": s.WriteFail}
	}
	return world{
		git: gittest.NewFake(gittest.FakeParams{
			Main:   mainPath,
			Branch: "main",
			Local:  []string{"topic"},
			Worktrees: []worktree.Worktree{
				{Path: pathOf(featA), State: worktree.OnBranch{Branch: featA}},
				{Path: pathOf(featB), State: worktree.OnBranch{Branch: featB}},
				{Path: pathOf(featC), State: worktree.OnBranch{Branch: featC}},
				{Path: pathOf(featX), State: worktree.OnBranch{Branch: featX}},
			},
		}),
		store: storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{
			featA: {SourceBranch: "main"},
			featB: {SourceBranch: featA},
			featC: {SourceBranch: featB},
			featX: {SourceBranch: "main"},
		}, Fail: fail}),
		config: configtest.NewFake(configtest.FakeParams{Config: &project.Config{Worktrees: project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch}}}),
		locks:  &operationtest.Locks{},
		seed:   s,
	}
}

func (w world) env() uicli.Env {
	cwd := w.seed.Cwd
	if cwd == "" {
		cwd = mainPath
	}
	return uicli.Env{Cwd: cwd, ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (reparent.UseCase, error) {
	return w.usecase(), nil
}

func (w world) usecase() reparent.UseCase {
	return reparent.UseCase{
		Git:        w.git,
		Meta:       w.store,
		Repository: repository.Service{Config: w.config, Git: w.git, MainPath: mainPath, Init: &domain.FollowUp{Command: "init"}},
		Operations: operation.Service{
			Locker:    w.locks,
			Publisher: w.locks,
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return createdAt },
		},
	}
}
