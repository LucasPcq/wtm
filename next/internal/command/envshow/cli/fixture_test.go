package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/envshow"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

const (
	mainPath    = "/repo/main"
	billing     = "feat/billing"
	billingPath = "/repo/.trees/feat-billing"
)

// open opens env show over a monorepo of two apps: the api's PORT and the web's linked to their jobs, the web's API_URL to the api's address, a MAILER_URL following nothing on the stack's mailpit port, a secret copied; feat/billing, worktree 2, has its web PORT out of step.
func open(context.Context, string) (envshow.UseCase, error) {
	git := gittest.NewFake(gittest.FakeParams{Main: mainPath, Branch: "main", Worktrees: []worktree.Worktree{{Path: billingPath, State: worktree.OnBranch{Branch: billing}}}})
	store := storetest.NewFake(storetest.FakeParams{
		Meta: map[string]worktree.Meta{billing: {SourceBranch: "main", EnvStrategy: project.EnvMain, Ordinal: 2, Isolation: project.Isolated}},
		Files: map[string]string{
			mainPath + "/apps/api/.env":    "PORT=4000\nMAILER_URL=http://localhost:8025\nJWT_SECRET=s3cret\n",
			mainPath + "/apps/web/.env":    "PORT=3000\nAPI_URL=http://localhost:4000\n",
			billingPath + "/apps/api/.env": "PORT=4020\nMAILER_URL=http://localhost:8025\nJWT_SECRET=s3cret\n",
			billingPath + "/apps/web/.env": "PORT=3010\nAPI_URL=http://localhost:4020\n",
		},
	})
	config := project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"}, Env: project.Env{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: "apps/api/.env"}, {Target: "apps/web/.env"}}}}
	run := project.Run{Declared: true, Addressing: project.AddressPorts, Jobs: []project.Job{
		{Name: "api", Kind: project.Service, Cwd: "apps/api", Ports: map[string]int{"PORT": 4000}, URL: &project.URL{Port: "PORT"}},
		{Name: "web", Kind: project.Service, Cwd: "apps/web", Ports: map[string]int{"PORT": 3000}, URL: &project.URL{Port: "PORT"}},
		{Name: "stack", Kind: project.Service, Ports: map[string]int{"MAILPIT_UI": 8025}},
	}, EnvPorts: []project.EnvPortLink{
		{File: "apps/api/.env", Key: "PORT", Job: "api", Port: "PORT"},
		{File: "apps/web/.env", Key: "PORT", Job: "web", Port: "PORT"},
		{File: "apps/web/.env", Key: "API_URL", Job: "api", Port: "PORT"},
	}}
	return envshow.UseCase{
		Git:        git,
		Meta:       store,
		Files:      store,
		PublicPort: func(context.Context, int) int { return 0 },
		Repository: repository.Service{Config: configtest.NewFake(configtest.FakeParams{Config: &config, Run: run}), Git: git, MainPath: mainPath, Init: &domain.FollowUp{Command: "init"}},
	}, nil
}

func env() uicli.Env {
	return uicli.Env{Cwd: billingPath, ProjectDir: mainPath}
}
