package shell

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/env"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachLineAHookPrintsReachesTheOutputStdoutAndStderrAlike(t *testing.T) {
	out := &collected{}

	err := RunHooks(t.Context(), env.HookChain{Hooks: []project.Hook{{Cmd: "echo one; echo two >&2"}}, WorkDir: t.TempDir(), Output: out.add})

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"one", "two"}, out.all())
}

func TestAnUnfinishedLineEndsWithItsHook(t *testing.T) {
	out := &collected{}

	err := RunHooks(t.Context(), env.HookChain{Hooks: []project.Hook{{Cmd: "printf first"}, {Cmd: "printf second"}}, WorkDir: t.TempDir(), Output: out.add})

	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, out.all())
}

func TestAFailedHookReturnsItsCommandAndExitCodeAndStopsTheChain(t *testing.T) {
	out := &collected{}

	err := RunHooks(t.Context(), env.HookChain{Hooks: []project.Hook{{Cmd: "exit 3"}, {Cmd: "echo never"}}, WorkDir: t.TempDir(), Output: out.add})

	var failure *env.HookFailure
	require.ErrorAs(t, err, &failure)
	assert.ErrorContains(t, err, "hook failed")
	assert.NotContains(t, err.Error(), "hook stopped")
	assert.Equal(t, "exit 3", failure.Cmd)
	require.NotNil(t, failure.ExitCode)
	assert.Equal(t, 3, *failure.ExitCode)
	assert.Equal(t, "hook failed: exit status 3", err.Error())
	assert.Empty(t, out.all())
}

func TestAFailedHooksOutputIsInItsLogFile(t *testing.T) {
	log := filepath.Join(t.TempDir(), "hooks", "on_create-feat%2Fx.log")

	err := RunHooks(t.Context(), env.HookChain{Hooks: []project.Hook{{Cmd: "echo installing; echo broken >&2; exit 3"}}, WorkDir: t.TempDir(), Log: log})

	var failure *env.HookFailure
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, 3, *failure.ExitCode)
	content, readErr := os.ReadFile(log)
	require.NoError(t, readErr)
	assert.Contains(t, string(content), "installing\n")
	assert.Contains(t, string(content), "broken\n")
}

func TestAHookKilledBySignalHasNoExitCode(t *testing.T) {
	err := RunHooks(t.Context(), env.HookChain{Hooks: []project.Hook{{Cmd: "kill -9 $$"}}, WorkDir: t.TempDir()})

	var failure *env.HookFailure
	require.ErrorAs(t, err, &failure)
	assert.Nil(t, failure.ExitCode)
}

func TestContinueOnErrorGoesPastAFailedHook(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")

	err := RunHooks(t.Context(), env.HookChain{Hooks: []project.Hook{{Cmd: "false", ContinueOnError: true}, {Cmd: "touch " + marker}}, WorkDir: dir})

	require.NoError(t, err)
	assert.FileExists(t, marker)
}

func TestNoHookRunsNothing(t *testing.T) {
	assert.NoError(t, RunHooks(t.Context(), env.HookChain{WorkDir: t.TempDir()}))
}

func TestABlankCommandIsSkipped(t *testing.T) {
	assert.NoError(t, RunHooks(t.Context(), env.HookChain{Hooks: []project.Hook{{Cmd: "   "}}, WorkDir: t.TempDir()}))
}

func TestACommandIsAShellLine(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "chained")

	err := RunHooks(t.Context(), env.HookChain{Hooks: []project.Hook{{Cmd: "echo first > " + marker + " && echo second >> " + marker}}, WorkDir: dir})

	require.NoError(t, err)
	content, readErr := os.ReadFile(marker)
	require.NoError(t, readErr)
	assert.Equal(t, "first\nsecond\n", string(content))
}

func TestAHookRunsInItsOwnCwdElseInTheWorktree(t *testing.T) {
	worktree := t.TempDir()
	elsewhere := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(worktree, "apps"), 0o755))
	out := &collected{}

	err := RunHooks(t.Context(), env.HookChain{
		Hooks:   []project.Hook{{Cmd: "pwd -P"}, {Cmd: "pwd -P", Cwd: elsewhere}, {Cmd: "pwd -P", Cwd: "apps"}},
		WorkDir: worktree,
		Output:  out.add,
	})

	require.NoError(t, err)
	assert.Equal(t, []string{resolved(t, worktree), resolved(t, elsewhere), resolved(t, filepath.Join(worktree, "apps"))}, out.all())
}

func TestAPlaceholderPathReachesTheCommandAsSpelledOnDisk(t *testing.T) {
	for _, name := range []string{"it's", "cost$HOME", `a"b`, "sp ace", "back`tick`"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			worktree := filepath.Join(root, name)
			require.NoError(t, os.MkdirAll(worktree, 0o755))
			hooks := []project.Hook{
				{Cmd: "printf %s {{worktree}} > {{root}}/unquoted"},
				{Cmd: `printf %s "{{worktree}}" > "{{root}}/double"`},
				{Cmd: "printf %s '{{worktree}}' > '{{root}}/single'"},
			}

			vars := env.HookVars{Worktree: worktree, Root: root}
			for i, hook := range hooks {
				hooks[i] = vars.Hook(hook)
			}

			err := RunHooks(t.Context(), env.HookChain{Hooks: hooks, WorkDir: worktree})

			require.NoError(t, err)
			for _, file := range []string{"unquoted", "double", "single"} {
				content, readErr := os.ReadFile(filepath.Join(root, file))
				require.NoError(t, readErr)
				assert.Equal(t, worktree, string(content), file)
			}
		})
	}
}

func TestTheEnvGivenWinsAndNoInheritedWorktreeVariableFillsOneLeftUnset(t *testing.T) {
	t.Setenv("COMPOSE_PROJECT_NAME", "launched-from-another-worktree")
	t.Setenv("WTM_PORT_OFFSET", "90")
	out := &collected{}

	err := RunHooks(t.Context(), env.HookChain{
		Hooks:   []project.Hook{{Cmd: `echo "[$WTM_PORT_OFFSET][$COMPOSE_PROJECT_NAME]"`}},
		WorkDir: t.TempDir(),
		Env:     map[string]string{"WTM_PORT_OFFSET": "20"},
		Unset:   []string{"WTM_PORT_OFFSET", "COMPOSE_PROJECT_NAME"},
		Output:  out.add,
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"[20][]"}, out.all())
}

func TestAVariableToUnsetIsUnsetEvenWhenNoEnvIsGiven(t *testing.T) {
	t.Setenv("COMPOSE_PROJECT_NAME", "launched-from-another-worktree")
	t.Setenv("WTM_PORT_OFFSET", "90")
	out := &collected{}

	err := RunHooks(t.Context(), env.HookChain{
		Hooks:   []project.Hook{{Cmd: `echo "[$WTM_PORT_OFFSET][$COMPOSE_PROJECT_NAME]"`}},
		WorkDir: t.TempDir(),
		Unset:   []string{"WTM_PORT_OFFSET", "COMPOSE_PROJECT_NAME"},
		Output:  out.add,
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"[][]"}, out.all())
}

func TestWithoutEnvAHookInheritsTheProcessEnvironment(t *testing.T) {
	t.Setenv("COMPOSE_PROJECT_NAME", "inherited")
	out := &collected{}

	err := RunHooks(t.Context(), env.HookChain{Hooks: []project.Hook{{Cmd: "echo $COMPOSE_PROJECT_NAME"}}, WorkDir: t.TempDir(), Output: out.add})

	require.NoError(t, err)
	assert.Equal(t, []string{"inherited"}, out.all())
}

func TestACancelledRunStopsTheHookInFlightAndNamesItStopped(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(200*time.Millisecond, cancel)
	begin := time.Now()

	err := RunHooks(ctx, env.HookChain{Hooks: []project.Hook{{Cmd: "sleep 30"}, {Cmd: "echo never"}}, WorkDir: t.TempDir()})

	var failure *env.HookFailure
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, "sleep 30", failure.Cmd)
	assert.ErrorContains(t, err, "hook stopped")
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotContains(t, err.Error(), "hook failed")
	assert.NotContains(t, err.Error(), "exit status")
	assert.Less(t, time.Since(begin), interruptGrace)
}

func TestAnInterruptedHookKeepsItsOutputSoFar(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	out := &collected{}

	err := RunHooks(ctx, env.HookChain{
		Hooks:   []project.Hook{{Cmd: "printf 'a\\nb\\npartial'; sleep 30"}},
		WorkDir: t.TempDir(),
		Output: func(line string) {
			out.add(line)
			if line == "b" {
				time.AfterFunc(100*time.Millisecond, cancel)
			}
		},
	})

	require.ErrorContains(t, err, "hook stopped")
	assert.Equal(t, []string{"a", "b", "partial"}, out.all())
}

func TestAnInterruptStopsTheChainPastContinueOnError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(200*time.Millisecond, cancel)
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")

	err := RunHooks(ctx, env.HookChain{Hooks: []project.Hook{{Cmd: "sleep 30", ContinueOnError: true}, {Cmd: "touch " + marker, ContinueOnError: true}}, WorkDir: dir})

	require.ErrorIs(t, err, context.Canceled)
	assert.NoFileExists(t, marker)
}

func TestAnInterruptBeforeTheFirstHookStartsNone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out := &collected{}

	err := RunHooks(ctx, env.HookChain{Hooks: []project.Hook{{Cmd: "echo never"}, {Cmd: "echo never"}}, WorkDir: t.TempDir(), Output: out.add})

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, "context canceled: 2 hook(s) not run", err.Error())
	assert.Empty(t, out.all())
}

func TestAnInterruptBetweenHooksStartsNoMore(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	dir := t.TempDir()
	second, third := filepath.Join(dir, "second"), filepath.Join(dir, "third")

	err := RunHooks(ctx, env.HookChain{
		Hooks:   []project.Hook{{Cmd: "echo first"}, {Cmd: "touch " + second}, {Cmd: "touch " + third}},
		WorkDir: dir,
		Output:  func(string) { cancel() },
	})

	require.ErrorIs(t, err, context.Canceled)
	assert.NoFileExists(t, second)
	assert.NoFileExists(t, third)
}

func TestACancelledHookIsInterruptedBeforeItIsTerminated(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	out := &collected{}

	err := RunHooks(ctx, env.HookChain{
		Hooks:   []project.Hook{{Cmd: `trap 'echo int; exit 3' INT; echo ready; while :; do sleep 0.05; done`}},
		WorkDir: t.TempDir(),
		Output: func(line string) {
			out.add(line)
			if line == "ready" {
				cancel()
			}
		},
	})

	require.ErrorContains(t, err, "hook stopped")
	assert.Equal(t, []string{"ready", "int"}, out.all())
}

func TestACancelledHookTakesAGrandchildItStartedWithIt(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var grandchild int

	err := RunHooks(ctx, env.HookChain{
		Hooks:   []project.Hook{{Cmd: "sleep 30 & echo $!; wait"}},
		WorkDir: t.TempDir(),
		Output: func(line string) {
			grandchild, _ = strconv.Atoi(strings.TrimSpace(line))
			cancel()
		},
	})
	t.Cleanup(func() { _ = syscall.Kill(grandchild, syscall.SIGKILL) })

	require.ErrorContains(t, err, "hook stopped")
	require.Positive(t, grandchild)
	assert.Eventually(t, func() bool { return syscall.Kill(grandchild, 0) != nil }, 2*interruptGrace, 20*time.Millisecond)
}

func TestTheTwoStreamsReachTheOutputOneLineAtATime(t *testing.T) {
	var mu sync.Mutex
	lines := 0
	unguarded := func(string) { lines++ }

	err := RunHooks(t.Context(), env.HookChain{
		Hooks:   []project.Hook{{Cmd: "for i in 1 2 3 4 5 6 7 8 9 10; do echo out; echo err >&2; done"}},
		WorkDir: t.TempDir(),
		Output:  unguarded,
	})

	require.NoError(t, err)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 20, lines)
}

func TestAHookThatBackgroundsAJobSucceedsWithoutWaitingForIt(t *testing.T) {
	out := &collected{}
	begin := time.Now()

	err := RunHooks(t.Context(), env.HookChain{Hooks: []project.Hook{{Cmd: "sleep 7 & echo started"}}, WorkDir: t.TempDir(), Output: out.add})

	require.NoError(t, err)
	assert.Equal(t, []string{"started"}, out.all())
	assert.Less(t, time.Since(begin), 5*time.Second)
}
