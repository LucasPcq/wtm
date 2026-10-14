package hooks_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/hooks"
)

func TestEachLineAHookPrintsIsEmittedStdoutAndStderrAlike(t *testing.T) {
	var lines []string

	err := hooks.Execute(t.Context(), hooks.ExecuteParams{
		Hooks:   []domain.HookCommand{{Cmd: "echo one; echo two >&2"}},
		WorkDir: t.TempDir(),
		Output:  func(line string) { lines = append(lines, line) },
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"one", "two"}, lines, "two pipes keep no order between them")
}

func TestAnUnfinishedLineEndsWithItsHook(t *testing.T) {
	var lines []string

	err := hooks.Execute(t.Context(), hooks.ExecuteParams{
		Hooks:   []domain.HookCommand{{Cmd: "printf first"}, {Cmd: "printf second"}},
		WorkDir: t.TempDir(),
		Output:  func(line string) { lines = append(lines, line) },
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, lines)
}

func TestAFailedHookIsNamedWithItsExitCode(t *testing.T) {
	err := hooks.Execute(t.Context(), hooks.ExecuteParams{
		Hooks:   []domain.HookCommand{{Cmd: "exit 3"}, {Cmd: "echo never"}},
		WorkDir: t.TempDir(),
		Output:  func(string) { t.Error("a hook after the failed one ran") },
	})

	var failure hooks.Failure
	require.ErrorAs(t, err, &failure)
	assert.ErrorIs(t, err, domain.ErrHookFailed)
	assert.Equal(t, "exit 3", failure.Cmd)
	require.NotNil(t, failure.ExitCode)
	assert.Equal(t, 3, *failure.ExitCode)
}

func TestAnInterruptedHookIsReportedStoppedWithItsOutputSoFar(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var lines []string
	begin := time.Now()

	err := hooks.Execute(ctx, hooks.ExecuteParams{
		Hooks:   []domain.HookCommand{{Cmd: "printf 'a\\nb\\npartial'; sleep 30"}, {Cmd: "echo never"}},
		WorkDir: t.TempDir(),
		Output: func(line string) {
			lines = append(lines, line)
			if line == "b" {
				time.AfterFunc(100*time.Millisecond, cancel)
			}
		},
	})

	require.ErrorIs(t, err, domain.ErrHookStopped)
	assert.NotContains(t, err.Error(), "exit status")
	var failure hooks.Failure
	require.True(t, errors.As(err, &failure))
	assert.Equal(t, "printf 'a\\nb\\npartial'; sleep 30", failure.Cmd)
	assert.Equal(t, []string{"a", "b", "partial"}, lines)
	assert.Less(t, time.Since(begin), domain.SubprocessInterruptGrace)
}

func TestAnInterruptBeforeAHookStartsNone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := hooks.Execute(ctx, hooks.ExecuteParams{
		Hooks:   []domain.HookCommand{{Cmd: "echo never"}},
		WorkDir: t.TempDir(),
		Output:  func(string) { t.Error("a hook ran after the interrupt") },
	})

	require.ErrorIs(t, err, domain.ErrCancelled)
}

func TestEachHookSaysWhenItStartsAndHowItEndedAFailureGonePastIncluded(t *testing.T) {
	var beats []domain.HookBeat

	err := hooks.Execute(t.Context(), hooks.ExecuteParams{
		Hooks:   []domain.HookCommand{{Cmd: "echo broken >&2; exit 2", ContinueOnError: true}, {Cmd: "true"}},
		WorkDir: t.TempDir(),
		OnHook:  func(beat domain.HookBeat) { beats = append(beats, beat) },
	})

	require.NoError(t, err)
	require.Len(t, beats, 4)
	assert.True(t, beats[0].Started)
	assert.False(t, beats[1].Started)
	assert.Equal(t, "echo broken >&2; exit 2", beats[1].Cmd)
	assert.NotEmpty(t, beats[1].Err)
	assert.Equal(t, "broken", beats[1].Stderr)
	assert.Positive(t, beats[1].Duration)
	assert.True(t, beats[2].Started)
	assert.Empty(t, beats[3].Err)
}
