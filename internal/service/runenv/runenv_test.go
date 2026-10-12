package runenv_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/runenv"
)

func TestALinkedWorktreesJobsRunOffsetByItsNumber(t *testing.T) {
	t.Setenv(domain.EnvComposeProjectName, "launched-from-another-worktree")

	env := runenv.JobEnv(runenv.Params{ProjectDir: projectDir(t), Run: runConfig(), Identity: runenv.Identity{Branch: "feat/a", Ordinal: 2}})

	assert.Equal(t, "2", env[domain.EnvOrdinal])
	assert.Equal(t, "20", env[domain.EnvPortOffset])
	assert.Equal(t, "feat-a", env[domain.EnvWorktree])
	assert.Equal(t, "shop-feat-a", env[domain.EnvComposeProjectName])
}

func TestAVerbatimWorktreeRunsOnTheBasePortsWithNoComposeProject(t *testing.T) {
	params := runenv.Params{ProjectDir: projectDir(t), Run: runConfig(), Identity: runenv.Identity{Branch: "feat/a", Ordinal: 2, Isolation: domain.IsolationVerbatim}}

	env := runenv.JobEnv(params)

	assert.Equal(t, "0", env[domain.EnvPortOffset])
	assert.NotContains(t, env, domain.EnvComposeProjectName)
	assert.Empty(t, runenv.ComposeProject(params))
}

func TestTheMainCheckoutsComposeProjectNeverFollowsItsBranch(t *testing.T) {
	params := runenv.Params{ProjectDir: projectDir(t), Run: runConfig(), Identity: runenv.Identity{Branch: "chore/dev", IsMain: true}}

	assert.Equal(t, "shop", runenv.ComposeProject(params))
	assert.Equal(t, "shop", runenv.JobEnv(params)[domain.EnvComposeProjectName])
}

func TestTheMainCheckoutsComposeProjectIsTheOneItsEnvNames(t *testing.T) {
	dir := projectDir(t)
	writeFile(t, filepath.Join(dir, "infra", domain.EnvFileName), "COMPOSE_PROJECT_NAME=stack\n")
	params := runenv.Params{ProjectDir: dir, Run: runConfig(), Identity: runenv.Identity{Branch: "main", IsMain: true}}

	assert.Equal(t, "stack", runenv.ComposeProject(params))
	assert.Equal(t, "stack", runenv.JobEnv(params)[domain.EnvComposeProjectName])
}

func TestALinkedWorktreesComposeProjectFollowsItsBranch(t *testing.T) {
	params := runenv.Params{ProjectDir: projectDir(t), Run: runConfig(), Identity: runenv.Identity{Branch: "feat/x", Ordinal: 1}}

	assert.Equal(t, "shop-feat-x", runenv.ComposeProject(params))
}
