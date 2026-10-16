package jobenv

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
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

func TestHooksReadTheRunEnvOnlyOfAWorktreeThatChoseAndAComposeStack(t *testing.T) {
	compose := run.Config{Jobs: []run.Job{{Name: "db", Cmd: "docker compose up -d"}}}
	script := run.Config{Jobs: []run.Job{{Name: "web", Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}}}}
	cases := []struct {
		name   string
		params HooksReadRunParams
		want   bool
	}{
		{"isolated with a compose job", HooksReadRunParams{Run: compose, Isolation: run.Isolated}, true},
		{"verbatim with a compose job", HooksReadRunParams{Run: compose, Isolation: run.Verbatim}, true},
		{"never chose", HooksReadRunParams{Run: compose}, false},
		{"no compose job", HooksReadRunParams{Run: script, Isolation: run.Isolated}, false},
		{"no run config", HooksReadRunParams{Isolation: run.Isolated}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, HooksReadRun(tc.params))
		})
	}
}
