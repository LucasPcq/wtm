package env_test

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const (
	mainPath = "/repo/main"
	billing  = "feat/billing"
	login    = "fix/login"
	verbatim = "feat/verbatim"
	fresh    = "feat/fresh"
	webEnv   = "apps/web/.env"
	apiEnv   = "apps/api/.env"
)

var createdAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// A seed is a repository to reconcile: its git, its meta records, its .env files by path, the env strategy, its run.toml and the daemon's jobs.
type seed struct {
	Git      gittest.FakeParams
	Meta     map[string]worktree.Meta
	Files    map[string]string
	Strategy project.EnvStrategy
	Run      project.Run
	RunErr   error
	Jobs     []run.Job
}

// A world is env's use case over the fakes a seed describes.
type world struct {
	usecase env.UseCase
	store   *storetest.Fake
}

func newWorld(s seed) world {
	if s.Git.Main == "" {
		s.Git.Main, s.Git.Branch = mainPath, "main"
	}

	git := gittest.NewFake(s.Git)
	store := storetest.NewFake(storetest.FakeParams{Meta: s.Meta, Files: s.Files})
	config := project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"}, Env: project.Env{Strategy: s.Strategy, Files: []project.EnvFile{{Target: apiEnv}, {Target: webEnv}}}}
	ids := 0
	return world{
		usecase: env.UseCase{
			Git:        git,
			Meta:       store,
			EnvFiles:   store,
			Jobs:       func(context.Context) ([]run.Job, error) { return s.Jobs, nil },
			Repository: repository.Service{Config: configtest.NewFake(configtest.FakeParams{Config: &config, Run: s.Run, Fail: map[string]error{"LoadRun": s.RunErr}}), Git: git, MainPath: mainPath},
			Operations: operation.Service{
				Locker:    &operationtest.Locks{},
				Publisher: &operationtest.Recorder{},
				NewOpID: func() string {
					ids++
					return "op-" + string(rune('0'+ids))
				},
				Now: func() time.Time { return createdAt },
			},
			Provision: provision.Service{Git: git, Meta: store, PublicPort: func(context.Context, int) int { return 0 }},
		},
		store: store,
	}
}

// linked is a linked worktree under /repo/.trees on branch.
func linked(branch string) worktree.Worktree {
	return worktree.Worktree{Path: "/repo/.trees/" + branch, State: worktree.OnBranch{Branch: branch}}
}

// acme is run.toml of a monorepo: an api and a web job, each linked into its app's .env by its port, the web's .env holding the api's address.
func acme() project.Run {
	return project.Run{Declared: true, Addressing: project.AddressPorts, Jobs: []project.Job{
		{Name: "api", Kind: project.Service, Cmd: "pnpm dev", Cwd: "apps/api", Ports: map[string]int{"PORT": 4000}, URL: &project.URL{Port: "PORT"}},
		{Name: "web", Kind: project.Service, Cmd: "pnpm dev", Cwd: "apps/web", Ports: map[string]int{"PORT": 3000}, URL: &project.URL{Port: "PORT"}},
	}, EnvPorts: []project.EnvPortLink{
		{File: apiEnv, Key: "PORT", Job: "api", Port: "PORT"},
		{File: webEnv, Key: "PORT", Job: "web", Port: "PORT"},
		{File: webEnv, Key: "API_URL", Job: "api", Port: "PORT"},
	}}
}

// acmeSeed is a repository on acme's run.toml with the env strategy main: billing in step but its web PORT drifted and a JWT_SECRET main has and it lacks, login in step, a verbatim worktree, an unnumbered worktree never settled; each linked worktree has chosen its isolation.
func acmeSeed() seed {
	return seed{
		Git:      gittest.FakeParams{Worktrees: []worktree.Worktree{linked(billing), linked(login), linked(verbatim), linked(fresh)}},
		Strategy: project.EnvMain,
		Run:      acme(),
		Meta: map[string]worktree.Meta{
			billing:  {SourceBranch: "main", EnvStrategy: project.EnvMain, Ordinal: 2, Isolation: project.Isolated},
			login:    {SourceBranch: "main", EnvStrategy: project.EnvMain, Ordinal: 1, Isolation: project.Isolated},
			verbatim: {SourceBranch: "main", EnvStrategy: project.EnvMain, Ordinal: 3, Isolation: project.Verbatim},
			fresh:    {SourceBranch: "main", EnvStrategy: project.EnvMain, Isolation: project.Isolated},
		},
		Files: map[string]string{
			mainPath + "/" + apiEnv: "PORT=4000\nJWT_SECRET=s3cret\n",
			mainPath + "/" + webEnv: "PORT=3000\nAPI_URL=http://localhost:4000\n",

			"/repo/.trees/" + billing + "/" + apiEnv: "PORT=4020\nOLD=1\n",
			"/repo/.trees/" + billing + "/" + webEnv: "PORT=3010\nAPI_URL=http://localhost:4020\n",

			"/repo/.trees/" + login + "/" + apiEnv: "PORT=4010\nJWT_SECRET=s3cret\n",
			"/repo/.trees/" + login + "/" + webEnv: "PORT=3010\nAPI_URL=http://localhost:4010\n",

			"/repo/.trees/" + verbatim + "/" + apiEnv: "PORT=4000\nJWT_SECRET=s3cret\n",
			"/repo/.trees/" + verbatim + "/" + webEnv: "PORT=3000\nAPI_URL=http://localhost:4000\n",

			"/repo/.trees/" + fresh + "/" + apiEnv: "PORT=4000\nJWT_SECRET=s3cret\n",
			"/repo/.trees/" + fresh + "/" + webEnv: "PORT=3000\nAPI_URL=http://localhost:4000\n",
		},
	}
}
