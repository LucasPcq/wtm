package runenv

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/stretchr/testify/assert"
)

func TestAHookLearnsTheJobEnvironmentOfItsWorktree(t *testing.T) {
	env := HookEnv(HookEnvParams{Params: shopParams("feat/a", 2, "")})

	assert.Equal(t, "20", env["WTM_PORT_OFFSET"])
	assert.Equal(t, shopProject+"-feat-a", env["COMPOSE_PROJECT_NAME"])
}

func TestAHookRunsUnderTheComposeProjectItsWorktreesEnvNames(t *testing.T) {
	env := HookEnv(HookEnvParams{
		Params:     shopParams("feat/a", 2, ""),
		ComposeEnv: [][]dotenv.Line{dotenv.Parse([]byte("COMPOSE_PROJECT_NAME=renamed\n"))},
	})

	assert.Equal(t, "renamed", env["COMPOSE_PROJECT_NAME"])
}

func TestAHookKeepsTheDerivedComposeProjectWhenNoEnvNamesOne(t *testing.T) {
	env := HookEnv(HookEnvParams{
		Params:     shopParams("feat/a", 2, ""),
		ComposeEnv: [][]dotenv.Line{dotenv.Parse([]byte("PORT=1\nCOMPOSE_PROJECT_NAME=\n"))},
	})

	assert.Equal(t, shopProject+"-feat-a", env["COMPOSE_PROJECT_NAME"])
}
