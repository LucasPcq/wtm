package runenv_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/runenv"
)

const project = "shop"

// web is a per-worktree job on port 3000, infra a shared compose stack started
// from infra/.
func runConfig() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Kind: domain.JobKindService, Cmd: "npm start", Ports: map[string]int{"http": 3000}},
		{Name: "infra", Kind: domain.JobKindService, Cmd: "docker compose up -d", Cwd: "infra", Scope: domain.JobScopeShared},
	}}
}

// linked is a numbered worktree whose .env carries web's port and whose
// infra/.env names its compose project.
func linked(t *testing.T) runenv.EnvParams {
	t.Helper()
	run := runConfig()
	run.EnvPorts = []domain.EnvPortLink{{File: ".env", Key: "PORT", Job: "web", Port: "http"}}
	return runenv.EnvParams{
		Params:       runenv.Params{ProjectDir: projectDir(t), Run: run, Identity: runenv.Identity{Branch: "feat/a", Ordinal: 2}},
		WorktreePath: t.TempDir(),
		EnvFiles:     []domain.EnvFile{{Target: ".env"}, {Target: "infra/.env"}},
	}
}
