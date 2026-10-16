package jobenv

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
)

func TestTheDeclaredComposeProjectIsTheFirstEnvToNameOne(t *testing.T) {
	files := [][]dotenv.Line{dotenv.Parse([]byte("PORT=1\n")), dotenv.Parse([]byte("COMPOSE_PROJECT_NAME=\nCOMPOSE_PROJECT_NAME=later\n"))}

	assert.Empty(t, declaredComposeProject(files))
	assert.Equal(t, "stack", declaredComposeProject(append(files, dotenv.Parse([]byte("COMPOSE_PROJECT_NAME=stack\n")))))
}

func TestTheComposeDirectoriesAreWhereAStackStartsOnceEach(t *testing.T) {
	config := run.Config{Jobs: []run.Job{
		{Name: "db", Cmd: "docker compose up -d db", Cwd: "infra"},
		{Name: "cache", Cmd: "./up.sh", Stop: "docker-compose down", Cwd: "infra"},
		{Name: "root", Cmd: "docker compose up"},
		{Name: "web", Cmd: "pnpm dev", Cwd: "apps/web"},
	}}

	assert.Equal(t, []string{"infra", "."}, ComposeDirs(config))
	assert.Empty(t, ComposeDirs(run.Config{}))
}

func TestTheComposeEnvPathsAreTheEnvOfEachComposeDirectoryElseTheRoot(t *testing.T) {
	config := run.Config{Jobs: []run.Job{{Name: "db", Cmd: "docker compose up -d", Cwd: "infra"}, {Name: "root", Cmd: "docker compose up"}}}

	assert.Equal(t, []string{"infra/.env", ".env"}, ComposeEnvPaths(config))
	assert.Equal(t, []string{".env"}, ComposeEnvPaths(run.Config{}))
}
