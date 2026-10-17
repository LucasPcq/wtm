package hooks_test

import (
	"context"
	"io"
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

	err := hooks.RunHooks(ctx, hooks.RunHooksParams{Hooks: []domain.HookCommand{{Cmd: "sleep 30"}}, WorkDir: t.TempDir(), Output: io.Discard})

	var failure hooks.Failure
	require.ErrorAs(t, err, &failure)
	assert.ErrorIs(t, err, domain.ErrHookStopped)
	assert.NotErrorIs(t, err, domain.ErrHookFailed)
}

func TestAFailedHookIsNotAStoppedOne(t *testing.T) {
	err := hooks.RunHooks(t.Context(), hooks.RunHooksParams{Hooks: []domain.HookCommand{{Cmd: "exit 1"}}, WorkDir: t.TempDir(), Output: io.Discard})

	assert.ErrorIs(t, err, domain.ErrHookFailed)
	assert.NotErrorIs(t, err, domain.ErrHookStopped)
}
