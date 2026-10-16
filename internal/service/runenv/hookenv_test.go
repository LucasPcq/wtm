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

func TestAHookLearnsTheJobEnvironmentOfItsWorktree(t *testing.T) {
	env, err := runenv.HookEnv(runenv.HookEnvParams{Params: params(runenv.Identity{Branch: "feat/a", Ordinal: 2}, nil), WorktreePath: worktree})

	require.NoError(t, err)
	assert.Equal(t, "20", env[domain.EnvPortOffset])
	assert.Equal(t, project+"-feat-a", env[domain.EnvComposeProjectName])
}

func TestAHookRunsUnderTheComposeProjectItsWorktreesEnvNames(t *testing.T) {
	linked := params(runenv.Identity{Branch: "feat/a", Ordinal: 2}, map[string]string{worktree + "/infra/.env": "COMPOSE_PROJECT_NAME=renamed\n"})

	env, err := runenv.HookEnv(runenv.HookEnvParams{Params: linked, WorktreePath: worktree})

	require.NoError(t, err)
	assert.Equal(t, "renamed", env[domain.EnvComposeProjectName])
}

func TestAHookEnvWhoseWorktreeEnvDoesNotReadIsAnError(t *testing.T) {
	linked := params(runenv.Identity{Branch: "feat/a", Ordinal: 2}, nil)
	linked.Files = storetest.NewFake(storetest.FakeParams{Fail: map[string]error{"Read": errors.New("permission denied")}})

	_, err := runenv.HookEnv(runenv.HookEnvParams{Params: linked, WorktreePath: worktree})

	require.Error(t, err)
}
