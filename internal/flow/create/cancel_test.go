package create

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

type interruptingRecorder struct {
	*recorder
	cancel context.CancelFunc
}

func (r interruptingRecorder) Stage(ctx context.Context, params flow.StageParams) error {
	r.cancel()
	return r.recorder.Stage(ctx, params)
}

// Ctrl-C under the creation spinner lets the worktree be created whole, then
// stops before its hooks — and says the worktree exists but was not set up.
func TestAnInterruptedCreateKeepsTheWorktreeAndSkipsItsHooks(t *testing.T) {
	ctx := testContext(t)
	ctx.Config.Project.Hooks.OnCreate = []domain.HookCommand{{Cmd: "echo hooked"}}
	presenter := interruptingRecorder{recorder: newRecorder()}
	ctx.Publisher = presenter.Recorder
	runCtx, cancel := context.WithCancel(t.Context())
	presenter.cancel = cancel

	outcome, err := Run(runCtx, Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/x"}, From: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyEnv: "", KeyRecap: confirmCreate}},
		Presenter: presenter,
	})

	if !errors.Is(err, domain.ErrCancelled) || !strings.Contains(err.Error(), "not set up") {
		t.Fatalf("err = %v, want the cancellation naming the worktree left unset", err)
	}
	if len(outcome.Failed) != 1 || outcome.Failed[0].ExitCode != domain.ExitCodeCancelled || outcome.Failed[0].Path == "" {
		t.Fatalf("failed = %+v, want feat/x named, with its path, as cancelled", outcome.Failed)
	}
	if _, statErr := os.Stat(outcome.Failed[0].Path); statErr != nil {
		t.Errorf("worktree not on disk: %v", statErr)
	}
	if len(presenter.Hooks) != 0 {
		t.Errorf("hook phases = %v, want none after the interrupt", presenter.Hooks)
	}
	want := []domain.EventType{domain.EventWorktreeCreated, domain.EventWorktreeProvisioned}
	if got := presenter.PublishedTypes(); !slices.Equal(got, want) {
		t.Errorf("published %v, want %v", got, want)
	}
}

// The branches an interrupt stopped the batch before are skipped, as clean and
// prune report them, not failed: nothing was attempted for them.
func TestAnInterruptedBatchSkipsTheBranchesItNeverReached(t *testing.T) {
	ctx := testContext(t)
	presenter := interruptingRecorder{recorder: newRecorder()}
	runCtx, cancel := context.WithCancel(t.Context())
	presenter.cancel = cancel

	outcome, err := Run(runCtx, Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/x", "feat/y"}, From: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyEnv: "", KeyRecap: confirmCreate}},
		Presenter: presenter,
	})

	if !errors.Is(err, domain.ErrCancelled) {
		t.Fatalf("err = %v, want the run read as cancelled", err)
	}
	if len(outcome.Failed) != 1 || outcome.Failed[0].Branch != "feat/x" {
		t.Errorf("failed = %+v, want feat/x alone, created but not set up", outcome.Failed)
	}
	if len(outcome.Skipped) != 1 || outcome.Skipped[0] != (domain.PruneSkip{Branch: "feat/y", Reason: domain.PruneSkipInterrupted}) {
		t.Errorf("skipped = %+v, want feat/y skipped as interrupted", outcome.Skipped)
	}
}

type interruptingHooks struct {
	*recorder
	cancel context.CancelFunc
}

func (r interruptingHooks) HookPhase(params flow.HookPhaseParams) error {
	time.AfterFunc(300*time.Millisecond, r.cancel)
	return r.recorder.HookPhase(params)
}

// An interrupt that lands while the hooks run stops them, keeps the worktree,
// and says so: the reader is left with a checkout whose setup did not finish.
func TestAnInterruptDuringTheHooksNamesTheWorktreeLeftBehind(t *testing.T) {
	ctx := testContext(t)
	ctx.Config.Project.Hooks.OnCreate = []domain.HookCommand{{Cmd: "sleep 30"}, {Cmd: "echo never"}}
	runCtx, cancel := context.WithCancel(t.Context())

	outcome, err := Run(runCtx, Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/x"}, From: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyEnv: "", KeyRecap: confirmCreate}},
		Presenter: interruptingHooks{recorder: newRecorder(), cancel: cancel},
	})

	if !errors.Is(err, domain.ErrLeftBehind) || !strings.Contains(err.Error(), "hooks did not finish") {
		t.Fatalf("err = %v, want the worktree named as left with its hooks unfinished", err)
	}
	if len(outcome.Failed) != 1 || outcome.Failed[0].ExitCode != domain.ExitCodeCancelled {
		t.Fatalf("failed = %+v, want feat/x as cancelled", outcome.Failed)
	}
	if _, statErr := os.Stat(outcome.Failed[0].Path); statErr != nil {
		t.Errorf("worktree not on disk: %v", statErr)
	}
}

type cancelOnUpdate struct {
	*flowtest.Recorder
	cancel context.CancelFunc
}

func (p cancelOnUpdate) Publish(ctx context.Context, event domain.Event) {
	if event.Type == domain.EventWorktreeUpdated {
		p.cancel()
	}
	p.Recorder.Publish(ctx, event)
}

// The ports settle on the cancellable context, and with no on_create hook
// nothing came after them to notice an interrupt: the run read as a success.
func TestAnInterruptWhileThePortsSettleNamesTheWorktreeLeftBehind(t *testing.T) {
	ctx := testContext(t)
	ctx.Config.Project.Env.Files = []domain.EnvFile{{Target: ".env"}}
	linkedRunConfig(t, ctx, ".env")
	presenter := newRecorder()
	runCtx, cancel := context.WithCancel(t.Context())
	ctx.Publisher = cancelOnUpdate{Recorder: presenter.Recorder, cancel: cancel}

	outcome, err := Run(runCtx, Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/x"}, From: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyEnv: "", KeyIsolation: string(domain.IsolationIsolated), KeyRecap: confirmCreate}},
		Presenter: presenter,
	})

	if !errors.Is(err, domain.ErrLeftBehind) || !strings.Contains(err.Error(), "not set up") {
		t.Fatalf("err = %v, want the worktree named as created but not set up", err)
	}
	if len(outcome.Results) != 0 || len(outcome.Failed) != 1 || outcome.Failed[0].ExitCode != domain.ExitCodeCancelled {
		t.Fatalf("outcome = %+v, want feat/x failed as cancelled", outcome)
	}
	provisioned := presenter.Published[len(presenter.Published)-1]
	if provisioned.Type != domain.EventWorktreeProvisioned || provisioned.OK == nil || *provisioned.OK {
		t.Errorf("last event = %+v, want worktree.provisioned with ok false", provisioned)
	}
	for _, status := range presenter.Statuses {
		if strings.Contains(status.Text, "ports not settled") {
			t.Errorf("warned %q: the error already says the ports were not set up", status.Text)
		}
	}
}
