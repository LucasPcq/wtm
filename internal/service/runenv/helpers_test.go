package runenv_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runenv"
	"github.com/LucasPcq/wtm/internal/store"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

func jobEnv(t *testing.T, params runenv.Params) map[string]string {
	t.Helper()
	env, err := runenv.JobEnv(params)
	require.NoError(t, err)
	return env
}

func composeProject(t *testing.T, params runenv.Params) string {
	t.Helper()
	name, err := runenv.ComposeProject(params)
	require.NoError(t, err)
	return name
}

func writes(t *testing.T, params runenv.EnvParams) runenv.EnvWrites {
	t.Helper()
	resolved, err := runenv.Env(params)
	require.NoError(t, err)
	return resolved
}

func file(t *testing.T, files *storetest.Fake, path string) string {
	t.Helper()
	content, ok := files.File(path)
	require.True(t, ok, "%s is not there", path)
	return content
}

// editTo is another process rewriting a file whole.
func editTo(path, content string) store.EnvEdit {
	return store.EnvEdit{Path: path, Create: true, Edit: func([]domain.EnvLine) []domain.EnvLine { return rules.ParseEnv(content) }}
}
