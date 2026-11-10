package detect

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
)

func TestARepositorySuggestsItsEnvFilesHooksAndFolders(t *testing.T) {
	dir := repository(t, map[string]string{
		"package.json":               `{"packageManager": "pnpm@9.0.0"}`,
		"package-lock.json":          "{}",
		"docker-compose.yml":         "services: {}",
		"apps/api/.env.example":      "PORT=1",
		"apps/web/.env":              "PORT=2",
		"apps/web/.env.local":        "X=1",
		"packages/db/src/schema.sql": "",
		"node_modules/x/.env":        "",
		".trees/feat-a/.env":         "",
		"nested/.git":                "gitdir: elsewhere",
		"nested/.env":                "",
		".github/workflows/ci.yml":   "",
	})

	detected := Repository{Dir: dir}.Detect()

	assert.Equal(t, project.Detected{
		EnvFiles: []project.EnvFile{
			{Target: "apps/api/.env", Template: "apps/api/.env.example"},
			{Target: "apps/web/.env"},
			{Target: "apps/web/.env.local", Local: true},
		},
		OnCreate: []project.FoundHook{{Hook: project.Hook{Cmd: "pnpm install"}, File: "package.json"}},
		OnClean:  []project.FoundHook{{Hook: project.Hook{Cmd: "docker compose down"}, File: "docker-compose.yml"}},
		Dirs:     []string{"apps", "apps/api", "apps/web", "packages", "packages/db"},
	}, detected)
}

func TestAnEmptyRepositorySuggestsNothing(t *testing.T) {
	detected := Repository{Dir: t.TempDir()}.Detect()

	assert.Empty(t, detected.EnvFiles)
	assert.Empty(t, detected.OnCreate)
	assert.Empty(t, detected.OnClean)
	assert.Empty(t, detected.Dirs)
}
