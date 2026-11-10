package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComposeReadsEveryServiceEveryPortAndEveryPinnedName(t *testing.T) {
	compose, err := repository(t, "docker-compose.yml").Compose()

	require.NoError(t, err)
	assert.NotEmpty(t, compose.Digest)
	compose.Digest = ""
	assert.Equal(t, read(), compose)
}

func TestARepositoryWithNoComposeFileHasNone(t *testing.T) {
	compose, err := Repository{Dir: t.TempDir()}.Compose()

	require.NoError(t, err)
	assert.Equal(t, project.Compose{}, compose)
}

func TestComposeTakesTheFileDockerComposePicksFirst(t *testing.T) {
	repo := repository(t, "docker-compose.yml")
	require.NoError(t, os.WriteFile(filepath.Join(repo.Dir, "compose.yaml"), []byte("services: {}\n"), 0o644))

	compose, err := repo.Compose()

	require.NoError(t, err)
	assert.Equal(t, "compose.yaml", compose.File)
}

func TestAComposeFileThatDoesNotParseSaysWhy(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "compose.yml"), []byte("services: [\n"), 0o644))

	compose, err := Repository{Dir: dir}.Compose()

	require.NoError(t, err)
	assert.NotEmpty(t, compose.Err)
}

func TestSaveComposeRewritesTheValuesAndNothingElse(t *testing.T) {
	repo := repository(t, "docker-compose.yml")
	compose, err := repo.Compose()
	require.NoError(t, err)
	edits := []project.ComposeEdit{
		{At: compose.Services[0].Ports[0].At, New: `"${POSTGRES_PORT:-5432}:5432"`},
		{At: compose.Services[2].Ports[1].At, New: `${MAILPIT_PORT_1025:-1025}`},
		{At: compose.Names[0].At, New: `"${COMPOSE_PROJECT_NAME:-acme}-postgres"`},
	}

	written, err := repo.SaveCompose(t.Context(), project.ComposeRewrite{File: compose.File, Edits: edits, Replaces: compose.Digest})

	require.NoError(t, err)
	assert.True(t, written.Changed)
	before := readFile(t, filepath.Join("testdata", "docker-compose.yml"))
	after := strings.NewReplacer(`"5432:5432"`, `"${POSTGRES_PORT:-5432}:5432"`, "published: 1025", "published: ${MAILPIT_PORT_1025:-1025}", "container_name: acme-postgres", `container_name: "${COMPOSE_PROJECT_NAME:-acme}-postgres"`).Replace(before)
	assert.Equal(t, after, readFile(t, filepath.Join(repo.Dir, "docker-compose.yml")))
}

func TestSaveComposeRefusesAFileChangedSinceItWasRead(t *testing.T) {
	repo := repository(t, "docker-compose.yml")
	compose, err := repo.Compose()
	require.NoError(t, err)
	changed := "services: {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(repo.Dir, "docker-compose.yml"), []byte(changed), 0o644))

	_, err = repo.SaveCompose(t.Context(), project.ComposeRewrite{File: compose.File, Edits: []project.ComposeEdit{{At: compose.Services[0].Ports[0].At, New: "x"}}, Replaces: compose.Digest})

	require.ErrorIs(t, err, project.ErrComposeChanged)
	assert.Equal(t, changed, readFile(t, filepath.Join(repo.Dir, "docker-compose.yml")))
}

func TestSaveComposeRefusesARewriteThatWouldNotReadBack(t *testing.T) {
	cases := map[string]string{
		"not_yaml":        `"${POSTGRES_PORT:-5432}:5432`,
		"container_moved": `"${POSTGRES_PORT:-5432}:5433"`,
		"host_literal":    `"5433:5432"`,
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			repo := repository(t, "docker-compose.yml")
			compose, err := repo.Compose()
			require.NoError(t, err)
			before := readFile(t, filepath.Join(repo.Dir, "docker-compose.yml"))

			_, err = repo.SaveCompose(t.Context(), project.ComposeRewrite{File: compose.File, Edits: []project.ComposeEdit{{At: compose.Services[0].Ports[0].At, New: edit}}, Replaces: compose.Digest})

			require.ErrorContains(t, err, "would not read back")
			assert.Equal(t, before, readFile(t, filepath.Join(repo.Dir, "docker-compose.yml")))
		})
	}
}

func TestApplyRefusesAnEditWhoseTextIsNoLongerThere(t *testing.T) {
	_, err := Apply("ports:\n  - \"5433:5432\"\n", []project.ComposeEdit{{At: project.ComposeSpan{Line: 2, Column: 5, Text: `"5432:5432"`}, New: "x"}})

	require.ErrorIs(t, err, project.ErrComposeChanged)
}

func TestApplyMakesTwoEditsOfOneLine(t *testing.T) {
	edited, err := Apply("a: 1 2\n", []project.ComposeEdit{{At: project.ComposeSpan{Line: 1, Column: 4, Text: "1"}, New: "one"}, {At: project.ComposeSpan{Line: 1, Column: 6, Text: "2"}, New: "two"}})

	require.NoError(t, err)
	assert.Equal(t, "a: one two\n", edited)
}
