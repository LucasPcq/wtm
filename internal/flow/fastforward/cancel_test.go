package fastforward

import (
	"context"
	"errors"
	"path/filepath"
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
	if params.Message == domain.FastForwardStage {
		r.cancel()
	}
	return r.recorder.Stage(ctx, params)
}

// A branch an interrupt reached first is named as such and left where it was,
// not reported as a git failure, and the run exits cancelled.
func TestAnInterruptedFastForwardLeavesTheBranchAndSaysSo(t *testing.T) {
	dir := behindRepo(t)
	before := tipOf(t, dir, "main")
	runCtx, cancel := context.WithCancel(t.Context())

	outcome, err := Run(runCtx, Params{
		Context:   flow.Context{ProjectDir: dir, StateDir: filepath.Join(t.TempDir(), "state")},
		Request:   Request{Branches: []string{"main"}},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyConfirm: confirmYes}},
		Presenter: interruptingRecorder{recorder: &recorder{Recorder: &flowtest.Recorder{}}, cancel: cancel},
	})

	if !errors.Is(err, domain.ErrCancelled) {
		t.Fatalf("err = %v, want the run read as cancelled", err)
	}
	if len(outcome.Results) != 1 || outcome.Results[0].Status != domain.FFCancelled {
		t.Fatalf("results = %+v, want main named as interrupted", outcome.Results)
	}
	if tipOf(t, dir, "main") != before {
		t.Error("main moved after the interrupt")
	}
}
