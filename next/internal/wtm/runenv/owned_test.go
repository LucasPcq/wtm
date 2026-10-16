package runenv

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
)

func TestTheOwnedKeysGoToTheEnvOfTheComposeJobsOwnDirectory(t *testing.T) {
	config := run.Config{Jobs: []run.Job{
		{Name: "stack", Cmd: "docker compose -f docker-compose.yml up -d", Cwd: "."},
		{Name: "web", Cmd: "pnpm run dev", Cwd: "apps/shop/web"},
	}}

	writes := ownedWrites(ownedWritesParams{
		Run:    config,
		Files:  []project.EnvFile{{Target: ".env", Template: ".env.example"}, {Target: "apps/shop/web/.env", Template: "apps/shop/web/.env.example"}},
		Values: map[string]string{"COMPOSE_PROJECT_NAME": "repo-feat-x"},
	})

	assert.Equal(t, []OwnedEntry{{File: ".env", Key: "COMPOSE_PROJECT_NAME", Value: "repo-feat-x"}}, writes)
}

func TestAComposeDirectoryWithNoEnvTargetGetsNoOwnedKey(t *testing.T) {
	writes := ownedWrites(ownedWritesParams{
		Run:    run.Config{Jobs: []run.Job{{Name: "stack", Cmd: "docker compose up -d", Cwd: "infra"}}},
		Files:  []project.EnvFile{{Target: ".env"}},
		Values: map[string]string{"COMPOSE_PROJECT_NAME": "repo-feat-x"},
	})

	assert.Empty(t, writes)
}

func TestAnOwnedValueNobodyResolvedIsNotWritten(t *testing.T) {
	writes := ownedWrites(ownedWritesParams{
		Run:    run.Config{Jobs: []run.Job{{Name: "stack", Cmd: "docker compose up -d"}}},
		Files:  []project.EnvFile{{Target: ".env"}},
		Values: map[string]string{"COMPOSE_PROJECT_NAME": ""},
	})

	assert.Empty(t, writes)
}

func TestTheOwnedTargetsAreTheEnvFilesAComposeStackReads(t *testing.T) {
	config := run.Config{Jobs: []run.Job{
		{Name: "infra", Cmd: "docker compose up -d", Cwd: "infra"},
		{Name: "root", Cmd: "docker-compose up"},
		{Name: "web", Cmd: "pnpm dev", Cwd: "apps/web"},
	}}
	files := []project.EnvFile{{Target: ".env"}, {Target: "infra/.env"}, {Target: "apps/web/.env"}}

	assert.Equal(t, []string{"infra/.env", ".env"}, OwnedTargets(OwnedTargetsParams{Run: config, Files: files}))
	assert.Empty(t, OwnedTargets(OwnedTargetsParams{Run: config}))
	assert.Empty(t, OwnedTargets(OwnedTargetsParams{Files: files}))
}
