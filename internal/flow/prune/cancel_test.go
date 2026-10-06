package prune

import (
	"context"
	"errors"
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
	if strings.HasPrefix(params.Message, "Removing") {
		r.cancel()
	}
	return r.recorder.Stage(ctx, params)
}

// An interrupt during a removal still drops that worktree's data — nothing
// would reclaim it later — and leaves the next worktree and its data whole.
func TestAnInterruptedPruneReclaimsWhatItRemovedAndNamesTheRest(t *testing.T) {
	p := newPruneFixture(t, "feat/a", "feat/b")
	runCtx, cancel := context.WithCancel(t.Context())
	f := p.flow("feat/a", "feat/b")
	f.runCtx = runCtx
	f.presenter = interruptingRecorder{recorder: &recorder{Recorder: &flowtest.Recorder{}}, cancel: cancel}

	outcome, err := f.remove(removeParams{})

	if !errors.Is(err, domain.ErrCancelled) {
		t.Fatalf("err = %v, want the run read as cancelled", err)
	}
	if got := p.dropped(); got != "app_feat-a" {
		t.Errorf("dropped %q, want feat/a's alone", got)
	}
	if exists(p.paths["feat/a"]) || !exists(p.paths["feat/b"]) {
		t.Errorf("a=%v b=%v, want only feat/a removed", exists(p.paths["feat/a"]), exists(p.paths["feat/b"]))
	}
	result := outcome.Result
	if len(result.Pruned) != 1 || len(result.Skipped) != 1 || result.Skipped[0].Reason != domain.PruneSkipInterrupted {
		t.Errorf("result = %+v, want feat/a pruned and feat/b skipped as interrupted", result)
	}
}
