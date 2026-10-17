package hooks

import (
	"context"
	"errors"
	"io"
	"strings"
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
	if !errors.Is(err, domain.ErrHookStopped) || strings.Contains(err.Error(), "exit status") {
		t.Errorf("err = %v, want the hook named as stopped, not by the status its shell exited with", err)
	}
	if elapsed := time.Since(begin); elapsed > domain.SubprocessInterruptGrace {
		t.Fatalf("the hook ran %v after the cancel", elapsed)
	}
}

// continue_on_error forgives a hook that failed, not an interrupt: the hooks
// after an interrupted one are never started, nor reported as failing one by one.
func TestAnInterruptStopsTheSequencePastContinueOnError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(200*time.Millisecond, cancel)
	var beats []domain.HookBeat

	err := RunHooks(ctx, RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "sleep 30", ContinueOnError: true}, {Cmd: "true", ContinueOnError: true}, {Cmd: "true"}},
		WorkDir: t.TempDir(),
		Output:  io.Discard,
		OnHook:  func(beat domain.HookBeat) { beats = append(beats, beat) },
	})

	if err == nil {
		t.Fatal("an interrupted sequence reported success")
	}
	if len(beats) != 2 {
		t.Errorf("beats = %+v, want the interrupted hook alone, started and stopped", beats)
	}
}

// An interrupt between two hooks starts no more of them, and says so.
func TestAnInterruptBetweenHooksStartsNoMore(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var beats []domain.HookBeat

	err := RunHooks(ctx, RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "true"}, {Cmd: "true"}, {Cmd: "true"}},
		WorkDir: t.TempDir(),
		Output:  io.Discard,
		OnHook: func(beat domain.HookBeat) {
			beats = append(beats, beat)
			if !beat.Started {
				cancel()
			}
		},
	})

	if !errors.Is(err, domain.ErrCancelled) || !strings.Contains(err.Error(), "2 hook(s) not run") {
		t.Fatalf("err = %v, want the two hooks left named as not run", err)
	}
	if len(beats) != 2 {
		t.Errorf("beats = %+v, want only the first hook run", beats)
	}
}
