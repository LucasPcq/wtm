package runenv

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
)

func TestAComposeProjectIsQualifiedByItsRepository(t *testing.T) {
	cases := []struct {
		name     string
		project  string
		worktree string
		want     string
	}{
		{"qualified by the repository", "myproject", "main", "myproject-main"},
		{"a repository in capitals", "MyProject", "feat-x", "myproject-feat-x"},
		{"no repository", "", "feat-x", "feat-x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, composeProjectName(composeNameParams{Project: tc.project, Worktree: tc.worktree}))
		})
	}
}

func TestTheDeclaredComposeProjectIsTheFirstEnvToNameOne(t *testing.T) {
	files := [][]dotenv.Line{dotenv.Parse([]byte("PORT=1\n")), dotenv.Parse([]byte("COMPOSE_PROJECT_NAME=\n"))}

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

	assert.Equal(t, []string{"infra", "."}, composeDirs(config))
	assert.Empty(t, composeDirs(run.Config{}))
}

func TestTheComposeEnvPathsAreTheEnvOfEachComposeDirectoryElseTheRoot(t *testing.T) {
	config := run.Config{Jobs: []run.Job{{Name: "db", Cmd: "docker compose up -d", Cwd: "infra"}, {Name: "root", Cmd: "docker compose up"}}}

	assert.Equal(t, []string{"infra/.env", ".env"}, ComposeEnvPaths(config))
	assert.Equal(t, []string{".env"}, ComposeEnvPaths(run.Config{}))
}
