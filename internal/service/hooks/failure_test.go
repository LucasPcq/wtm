package hooks_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/hooks"
)

func TestAStoppedHookIsNotAFailedOne(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(100*time.Millisecond, cancel)

	err := hooks.Execute(ctx, hooks.ExecuteParams{Hooks: []domain.HookCommand{{Cmd: "sleep 30"}}, WorkDir: t.TempDir()})

	var failure hooks.Failure
	require.ErrorAs(t, err, &failure)
	assert.ErrorIs(t, err, domain.ErrHookStopped)
	assert.NotErrorIs(t, err, domain.ErrHookFailed)
}

func TestAFailedHookIsNotAStoppedOne(t *testing.T) {
	err := hooks.Execute(t.Context(), hooks.ExecuteParams{Hooks: []domain.HookCommand{{Cmd: "exit 1"}}, WorkDir: t.TempDir()})

	assert.ErrorIs(t, err, domain.ErrHookFailed)
	assert.NotErrorIs(t, err, domain.ErrHookStopped)
}
