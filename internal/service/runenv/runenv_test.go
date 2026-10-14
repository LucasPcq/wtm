package runenv_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/runenv"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

func TestALinkedWorktreesJobsRunOffsetByItsNumber(t *testing.T) {
	t.Setenv(domain.EnvComposeProjectName, "launched-from-another-worktree")

	env := jobEnv(t, params(runenv.Identity{Branch: "feat/a", Ordinal: 2}, nil))

	assert.Equal(t, "2", env[domain.EnvOrdinal])
	assert.Equal(t, "20", env[domain.EnvPortOffset])
	assert.Equal(t, "feat-a", env[domain.EnvWorktree])
	assert.Equal(t, project+"-feat-a", env[domain.EnvComposeProjectName])
}

func TestAVerbatimWorktreeRunsOnTheBasePortsWithNoComposeProject(t *testing.T) {
	verbatim := params(runenv.Identity{Branch: "feat/a", Ordinal: 2, Isolation: domain.IsolationVerbatim}, nil)

	env := jobEnv(t, verbatim)

	assert.Equal(t, "0", env[domain.EnvPortOffset])
	assert.NotContains(t, env, domain.EnvComposeProjectName)
	assert.Empty(t, composeProject(t, verbatim))
}

func TestTheMainCheckoutsComposeProjectNeverFollowsItsBranch(t *testing.T) {
	main := params(runenv.Identity{Branch: "chore/dev", IsMain: true}, nil)

	assert.Equal(t, project, composeProject(t, main))
	assert.Equal(t, project, jobEnv(t, main)[domain.EnvComposeProjectName])
}

func TestTheMainCheckoutsComposeProjectIsTheOneItsEnvNames(t *testing.T) {
	main := params(runenv.Identity{Branch: "main", IsMain: true}, map[string]string{projectDir + "/infra/.env": "COMPOSE_PROJECT_NAME=stack\n"})

	assert.Equal(t, "stack", composeProject(t, main))
	assert.Equal(t, "stack", jobEnv(t, main)[domain.EnvComposeProjectName])
}

func TestAMainCheckoutEnvThatDoesNotReadIsAnError(t *testing.T) {
	main := params(runenv.Identity{Branch: "main", IsMain: true}, nil)
	main.Files = storetest.NewFake(storetest.FakeParams{Fail: map[string]error{"Read": errors.New("permission denied")}})

	_, err := runenv.ComposeProject(main)
	require.Error(t, err)
	_, err = runenv.JobEnv(main)
	require.Error(t, err)
}

func TestALinkedWorktreesComposeProjectFollowsItsBranch(t *testing.T) {
	assert.Equal(t, project+"-feat-x", composeProject(t, params(runenv.Identity{Branch: "feat/x", Ordinal: 1}, nil)))
}
