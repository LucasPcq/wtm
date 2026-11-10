package envshow_test

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/envshow"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const (
	mainPath = "/repo/main"
	billing  = "feat/billing"
	webEnv   = "apps/web/.env"
	apiEnv   = "apps/api/.env"
	docsEnv  = "apps/docs/.env"
)

// A seed is a repository to read: its meta records, its .env files by path, its run.toml and why it does not load.
type seed struct {
	Meta   map[string]worktree.Meta
	Files  map[string]string
	Run    project.Run
	RunErr error
}

func linkedPath(branch string) string {
	return "/repo/.trees/" + branch
}

func newUseCase(s seed) envshow.UseCase {
	git := gittest.NewFake(gittest.FakeParams{Main: mainPath, Branch: "main", Worktrees: []worktree.Worktree{{Path: linkedPath(billing), State: worktree.OnBranch{Branch: billing}}}})
	store := storetest.NewFake(storetest.FakeParams{Meta: s.Meta, Files: s.Files})
	config := project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"}, Env: project.Env{Strategy: project.EnvExample, Files: []project.EnvFile{{Target: apiEnv}, {Target: webEnv}, {Target: docsEnv}}}}
	return envshow.UseCase{
		Git:        git,
		Meta:       store,
		Files:      store,
		PublicPort: func(context.Context, int) int { return 0 },
		Repository: repository.Service{Config: configtest.NewFake(configtest.FakeParams{Config: &config, Run: s.Run, Fail: map[string]error{"LoadRun": s.RunErr}}), Git: git, MainPath: mainPath},
	}
}

// acmeSeed is a monorepo of three apps whose .env files come from their templates: the api's PORT and the web's linked to their jobs, the web's API_URL to the api's address, a MAILER_URL that follows nothing but holds the stack's mailpit port, a secret copied; feat/billing, worktree 2, has its web PORT out of step, and lacks the docs' .env.
func acmeSeed() seed {
	return seed{
		Meta: map[string]worktree.Meta{billing: {SourceBranch: "main", EnvStrategy: project.EnvExample, Ordinal: 2, Isolation: project.Isolated}},
		Run: project.Run{Declared: true, Addressing: project.AddressPorts, Jobs: []project.Job{
			{Name: "api", Kind: project.Service, Cwd: "apps/api", Ports: map[string]int{"PORT": 4000}, URL: &project.URL{Port: "PORT"}},
			{Name: "web", Kind: project.Service, Cwd: "apps/web", Ports: map[string]int{"PORT": 3000}, URL: &project.URL{Port: "PORT"}},
			{Name: "stack", Kind: project.Service, Ports: map[string]int{"MAILPIT_UI": 8025}},
		}, EnvPorts: []project.EnvPortLink{
			{File: apiEnv, Key: "PORT", Job: "api", Port: "PORT"},
			{File: webEnv, Key: "PORT", Job: "web", Port: "PORT"},
			{File: webEnv, Key: "API_URL", Job: "api", Port: "PORT"},
		}},
		Files: map[string]string{
			mainPath + "/" + apiEnv:                        "PORT=4000\nMAILER_URL=http://localhost:8025\nJWT_SECRET=s3cret\n",
			mainPath + "/" + webEnv:                        "PORT=3000\nAPI_URL=http://localhost:4000\n",
			linkedPath(billing) + "/apps/api/.env.example": "PORT=\nMAILER_URL=\nJWT_SECRET=\n",
			linkedPath(billing) + "/apps/web/.env.example": "PORT=\nAPI_URL=\n",
			linkedPath(billing) + "/" + apiEnv:             "PORT=4020\nMAILER_URL=http://localhost:8025\nJWT_SECRET=s3cret\n",
			linkedPath(billing) + "/" + webEnv:             "PORT=3010\nAPI_URL=http://localhost:4020\n",
		},
	}
}
