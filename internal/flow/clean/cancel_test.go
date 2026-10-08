package clean

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

type interruptingRecorder struct {
	*recorder
	cancel context.CancelFunc
}

func (r interruptingRecorder) Stage(ctx context.Context, params flow.StageParams) error {
	if strings.HasPrefix(params.Message, "Removing") {
		r.cancel()
	}
	return r.recorder.Stage(ctx, params)
}

// An interrupted clean says what it removed and what it never reached, and
// exits cancelled even though nothing it attempted failed.
func TestAnInterruptedCleanNamesWhatItNeverReached(t *testing.T) {
	ctx := testContext(t)
	first := makeWorktree(t, ctx, "feat/a")
	untouched := makeWorktree(t, ctx, "feat/b")
	runCtx, cancel := context.WithCancel(t.Context())

	outcome, err := Run(runCtx, Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/a", "feat/b"}, BaseBranch: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteYes}},
		Presenter: interruptingRecorder{recorder: newRecorder(), cancel: cancel},
	})

	if !errors.Is(err, domain.ErrCancelled) || rules.ExitCode(err) != domain.ExitCodeCancelled {
		t.Fatalf("err = %v, want the run read as cancelled", err)
	}
	if len(outcome.Results) != 1 || outcome.Results[0].Branch != "feat/a" {
		t.Errorf("results = %+v, want feat/a removed", outcome.Results)
	}
	if len(outcome.Skipped) != 1 || outcome.Skipped[0] != (domain.PruneSkip{Branch: "feat/b", Reason: domain.PruneSkipInterrupted}) {
		t.Errorf("skipped = %+v, want feat/b named as interrupted", outcome.Skipped)
	}
	if _, statErr := os.Stat(first); !os.IsNotExist(statErr) {
		t.Errorf("feat/a still on disk: %v", statErr)
	}
	if _, statErr := os.Stat(untouched); statErr != nil {
		t.Errorf("feat/b must be untouched: %v", statErr)
	}
}
