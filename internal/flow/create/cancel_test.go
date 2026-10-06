package create

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

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
