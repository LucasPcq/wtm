package checkout

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	if params.Message == fmt.Sprintf(domain.CreateLoadingFmt, "feat/thing") {
		r.cancel()
	}
	return r.recorder.Stage(ctx, params)
}

// Ctrl-C while the PR's worktree is being created lets it be created whole and
// runs nothing after it, its hooks included.
func TestAnInterruptedCheckoutKeepsTheWorktreeWhole(t *testing.T) {
	ctx := testContext(t)
	witness := t.TempDir() + "/hooked"
	ctx.Config.Project.Hooks.OnCreate = []domain.HookCommand{{Cmd: "touch " + witness}}
	runCtx, cancel := context.WithCancel(t.Context())
	presenter := interruptingRecorder{recorder: newRecorder(), cancel: cancel}

	_, err := Run(runCtx, Params{
		Context:   ctx,
		Request:   Request{Number: 42},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyParent: "main", KeyEnv: "", KeyRecap: confirmCheckout}},
		Presenter: presenter,
	})

	if !errors.Is(err, domain.ErrCancelled) || !strings.Contains(err.Error(), "not set up") {
		t.Fatalf("err = %v, want the cancellation naming the worktree left unset", err)
	}
	if _, statErr := os.Stat(witness); !os.IsNotExist(statErr) {
		t.Error("the on_create hook ran after the interrupt")
	}
}
