package env

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/stretchr/testify/assert"
)

func TestTargetsKeepTheLinksOfADeclaredEnvAndNameTheOthers(t *testing.T) {
	config := project.Run{
		EnvPorts:  []project.EnvPortLink{{File: ".env", Key: "PORT"}, {File: "web/.env", Key: "WEB_PORT"}},
		EnvValues: []project.EnvValueLink{{File: ".env", Key: "REALM"}, {File: "api/.env", Key: "URL"}},
	}

	kept, ignored := Targets(config, []project.EnvFile{{Target: ".env"}})

	assert.Equal(t, []project.EnvPortLink{{File: ".env", Key: "PORT"}}, kept.EnvPorts)
	assert.Equal(t, []project.EnvValueLink{{File: ".env", Key: "REALM"}}, kept.EnvValues)
	assert.Equal(t, []Ignored{
		{Kind: LinkPort, Key: "WEB_PORT", File: "web/.env"},
		{Kind: LinkValue, Key: "URL", File: "api/.env"},
	}, ignored)
}

func TestTargetsOfARunWithNoLinkIgnoreNothing(t *testing.T) {
	config := project.Run{Jobs: []project.Job{{Name: "web"}}}

	kept, ignored := Targets(config, nil)

	assert.Equal(t, config, kept)
	assert.Empty(t, ignored)
}

func TestTheOwnedKeysGoToTheEnvOfTheComposeJobsOwnDirectory(t *testing.T) {
	config := project.Run{Jobs: []project.Job{
		{Name: "stack", Cmd: "docker compose -f docker-compose.yml up -d", Cwd: "."},
		{Name: "web", Cmd: "pnpm run dev", Cwd: "apps/shop/web"},
	}}

	writes := ownedWrites(
		config,
		[]project.EnvFile{{Target: ".env", Template: ".env.example"}, {Target: "apps/shop/web/.env", Template: "apps/shop/web/.env.example"}},
		"repo-feat-x",
	)

	assert.Equal(t, []OwnedEntry{{File: ".env", Key: "COMPOSE_PROJECT_NAME", Value: "repo-feat-x"}}, writes)
}

func TestAComposeDirectoryWithNoEnvTargetGetsNoOwnedKey(t *testing.T) {
	writes := ownedWrites(
		project.Run{Jobs: []project.Job{{Name: "stack", Cmd: "docker compose up -d", Cwd: "infra"}}},
		[]project.EnvFile{{Target: ".env"}},
		"repo-feat-x",
	)

	assert.Empty(t, writes)
}

func TestAnOwnedValueNobodyResolvedIsNotWritten(t *testing.T) {
	writes := ownedWrites(
		project.Run{Jobs: []project.Job{{Name: "stack", Cmd: "docker compose up -d"}}},
		[]project.EnvFile{{Target: ".env"}},
		"",
	)

	assert.Empty(t, writes)
}

func TestTheOwnedTargetsAreTheEnvFilesAComposeStackReads(t *testing.T) {
	config := project.Run{Jobs: []project.Job{
		{Name: "infra", Cmd: "docker compose up -d", Cwd: "infra"},
		{Name: "root", Cmd: "docker-compose up"},
		{Name: "web", Cmd: "pnpm dev", Cwd: "apps/web"},
	}}
	files := []project.EnvFile{{Target: ".env"}, {Target: "infra/.env"}, {Target: "apps/web/.env"}}

	assert.Equal(t, []string{"infra/.env", ".env"}, OwnedTargets(config, files))
	assert.Empty(t, OwnedTargets(config, nil))
	assert.Empty(t, OwnedTargets(project.Run{}, files))
}
