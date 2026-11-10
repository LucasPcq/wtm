package domain

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAHookSpelledReadsBackAsItself(t *testing.T) {
	cases := map[string]struct {
		hook    project.Hook
		spelled string
	}{
		"command": {project.Hook{Cmd: "pnpm install && make"}, "pnpm install && make"},
		"cwd":     {project.Hook{Cmd: "pnpm db:generate", Cwd: "packages/db"}, `{"cmd":"pnpm db:generate","cwd":"packages/db"}`},
		"go_on":   {project.Hook{Cmd: "docker compose down", ContinueOnError: true}, `{"cmd":"docker compose down","continue_on_error":true}`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.spelled, SpellHook(c.hook))
			assert.Equal(t, c.hook, HookOf(c.spelled))
		})
	}
}

func TestAValueThatIsNoHookObjectIsItsCommand(t *testing.T) {
	for name, value := range map[string]string{
		"shell_group": "{ make; }",
		"no_cmd":      `{"cwd":"x"}`,
		"unknown_key": `{"cmd":"x","when":"always"}`,
		"trailing":    `{"cmd":"x"} y`,
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, project.Hook{Cmd: value}, HookOf(value))
		})
	}
}

func TestAJSONObjectThatIsNoHookIsRefused(t *testing.T) {
	_, err := HooksOf("on_create", []string{"make", `{"cmd": "make", "cdw": "apps/api"}`, "{ make; }"})

	failure, ok := errors.AsType[*Error](err)
	require.True(t, ok, err)
	require.Len(t, failure.Fields, 1)
	assert.Equal(t, FieldError{Path: "on_create[1]", Code: CodeNotAHook, Template: "{path} is JSON but no hook: it takes cmd, cwd and continue_on_error"}, failure.Fields[0])
}
