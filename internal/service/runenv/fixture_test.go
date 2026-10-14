package runenv_test

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/runenv"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

const (
	project    = "shop"
	projectDir = "/repo/shop"
	worktree   = "/wt/feat-a"
)

func runConfig() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Kind: domain.JobKindService, Cmd: "npm start", Ports: map[string]int{"http": 3000}},
		{Name: "infra", Kind: domain.JobKindService, Cmd: "docker compose up -d", Cwd: "infra", Scope: domain.JobScopeShared},
	}}
}

func params(identity runenv.Identity, files map[string]string) runenv.Params {
	return runenv.Params{ProjectDir: projectDir, Run: runConfig(), Identity: identity, Files: storetest.NewFake(storetest.FakeParams{Files: files})}
}

func linked() runenv.EnvParams {
	base := params(runenv.Identity{Branch: "feat/a", Ordinal: 2}, nil)
	base.Run.EnvPorts = []domain.EnvPortLink{{File: ".env", Key: "PORT", Job: "web", Port: "http"}}
	return runenv.EnvParams{
		Params:       base,
		WorktreePath: worktree,
		EnvFiles:     []domain.EnvFile{{Target: ".env"}, {Target: "infra/.env"}},
	}
}
