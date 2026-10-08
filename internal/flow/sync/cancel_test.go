package sync

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type interruptedAfterRebase struct {
	*recordingPresenter
	cancel context.CancelFunc
}

func (p interruptedAfterRebase) Rebased(result domain.SyncResult) {
	p.cancel()
	p.recordingPresenter.Rebased(result)
}

// An interrupt that lands once the rebases are done still stops the run there:
// nothing is pushed, --push or not, and the run exits cancelled.
func TestAnInterruptedSyncPushesNothing(t *testing.T) {
	ctx := oneStackRepo(t)
	gittest.Git(t, ctx.ProjectDir, "commit", "--allow-empty", "-m", "main moved")
	runCtx, cancel := context.WithCancel(t.Context())
	presenter := interruptedAfterRebase{recordingPresenter: &recordingPresenter{}, cancel: cancel}

	_, err := Run(runCtx, Params{
		Context:   ctx,
		Request:   Request{All: true, Push: true, BaseBranch: "main"},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})

	if !errors.Is(err, domain.ErrCancelled) {
		t.Fatalf("err = %v, want the run read as cancelled", err)
	}
	if got := strings.Join(presenter.order, ","); strings.Contains(got, domain.SyncPushing) {
		t.Errorf("order = %q: an interrupted run pushed", got)
	}
}
