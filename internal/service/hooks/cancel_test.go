package hooks

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// An interrupted run stops the hook it is waiting on, and with it whatever the
// hook started, instead of waiting for an install to finish.
func TestACancelledRunStopsTheHookInFlight(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(200*time.Millisecond, cancel)

	begin := time.Now()
	err := RunHooks(ctx, RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "sleep 30"}, {Cmd: "echo never"}},
		WorkDir: t.TempDir(),
		Output:  io.Discard,
	})
	if err == nil {
		t.Fatal("a cancelled hook reported success")
	}
	if elapsed := time.Since(begin); elapsed > domain.SubprocessInterruptGrace {
		t.Fatalf("the hook ran %v after the cancel", elapsed)
	}
}
