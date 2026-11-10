package cli

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	envcmd "github.com/LucasPcq/wtm/next/internal/command/env"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

type sessionParams struct {
	App     form.UseCase[envcmd.Request, envcmd.Questions, envcmd.Outcome]
	Request envcmd.Request
}

// The steps of env's form, in the order asked, then the recap.
const (
	stepWorktrees = iota
	_             // all
	_             // from
	_             // overwrite
	stepPrune
	_ // isolation
	_ // show values
	stepRecap
)

const (
	mainPath = "/repo/main"
	billing  = "feat/billing"
	login    = "fix/login"
	apiEnv   = "apps/api/.env"
	webEnv   = "apps/web/.env"
)

var createdAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

type world struct {
	git    *gittest.Fake
	store  *storetest.Fake
	config *configtest.Fake
	locks  *operationtest.Locks
	jobs   []run.Job
}

func pathOf(branch string) string {
	return "/repo/wt-" + worktree.PathSegment(branch)
}

// newWorld is a monorepo whose run.toml links an api's and a web's port into their app's .env, its env strategy main: feat/billing's web PORT drifted, it lacks main's JWT_SECRET and holds an OLD key no source has; fix/login is in step.
func newWorld() world {
	config := project.Config{Worktrees: project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch}, Env: project.Env{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: apiEnv}, {Target: webEnv}}}}
	runToml := project.Run{Declared: true, Addressing: project.AddressPorts, Jobs: []project.Job{
		{Name: "api", Kind: project.Service, Cmd: "pnpm dev", Cwd: "apps/api", Ports: map[string]int{"PORT": 4000}, URL: &project.URL{Port: "PORT"}},
		{Name: "web", Kind: project.Service, Cmd: "pnpm dev", Cwd: "apps/web", Ports: map[string]int{"PORT": 3000}, URL: &project.URL{Port: "PORT"}},
	}, EnvPorts: []project.EnvPortLink{
		{File: apiEnv, Key: "PORT", Job: "api", Port: "PORT"},
		{File: webEnv, Key: "PORT", Job: "web", Port: "PORT"},
	}}
	return world{
		git: gittest.NewFake(gittest.FakeParams{Main: mainPath, Branch: "main", Worktrees: []worktree.Worktree{
			{Path: pathOf(billing), State: worktree.OnBranch{Branch: billing}},
			{Path: pathOf(login), State: worktree.OnBranch{Branch: login}},
		}}),
		store: storetest.NewFake(storetest.FakeParams{
			Meta: map[string]worktree.Meta{
				billing: {SourceBranch: "main", EnvStrategy: project.EnvMain, Ordinal: 2, Isolation: project.Isolated},
				login:   {SourceBranch: "main", EnvStrategy: project.EnvMain, Ordinal: 1, Isolation: project.Isolated},
			},
			Files: map[string]string{
				mainPath + "/" + apiEnv:        "PORT=4000\nJWT_SECRET=s3cret\n",
				mainPath + "/" + webEnv:        "PORT=3000\n",
				pathOf(billing) + "/" + apiEnv: "PORT=4020\nOLD=1\n",
				pathOf(billing) + "/" + webEnv: "PORT=3010\n",
				pathOf(login) + "/" + apiEnv:   "PORT=4010\nJWT_SECRET=s3cret\n",
				pathOf(login) + "/" + webEnv:   "PORT=3010\n",
			},
		}),
		config: configtest.NewFake(configtest.FakeParams{Config: &config, Run: runToml}),
		locks:  &operationtest.Locks{},
	}
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: pathOf(billing), ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (envcmd.UseCase, error) {
	return w.usecase(), nil
}

func (w world) usecase() envcmd.UseCase {
	return envcmd.UseCase{
		Git:        w.git,
		Meta:       w.store,
		EnvFiles:   w.store,
		Jobs:       func(context.Context) ([]run.Job, error) { return w.jobs, nil },
		Repository: repository.Service{Config: w.config, Git: w.git, MainPath: mainPath, Init: &domain.FollowUp{Command: "init"}},
		Operations: operation.Service{
			Locker:    w.locks,
			Publisher: w.locks,
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return createdAt },
		},
		Provision: provision.Service{Git: w.git, Meta: w.store},
	}
}
