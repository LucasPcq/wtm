package teardown_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/teardown"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

// interruptingPresenter cancels the run as the stage named Message begins, the
// way a Ctrl-C pressed under its spinner would.
type interruptingPresenter struct {
	*flowtest.Recorder
	Message string
	Cancel  context.CancelFunc
}

func (p *interruptingPresenter) Stage(ctx context.Context, params flow.StageParams) error {
	if strings.HasPrefix(params.Message, p.Message) {
		p.Cancel()
	}
	return p.Recorder.Stage(ctx, params)
}

func branchExists(t *testing.T, dir, branch string) bool {
	t.Helper()
	return exec.Command("git", "-C", dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch).Run() == nil
}

// An interrupt during a removal lets it finish — worktree, branch, event —
// and stops the batch before the next one, which stays whole.
func TestAnInterruptedBatchFinishesTheRemovalUnderWayAndStopsThere(t *testing.T) {
	globaldir.Isolate(t)
	processtest.Serve(t, nil)
	ctx := repoContext(t)
	targets := []teardown.Target{makeTarget(t, ctx, "feat/a"), makeTarget(t, ctx, "feat/b")}
	runCtx, cancel := context.WithCancel(t.Context())
	presenter := &interruptingPresenter{Recorder: &flowtest.Recorder{}, Message: "Removing", Cancel: cancel}
	ctx.Publisher = presenter.Recorder

	removals := teardown.Batch(runCtx, teardown.BatchParams{Context: ctx, Presenter: presenter, Targets: targets, ForceRemoval: true})

	if len(removals) != 2 || !removals[0].Removed() || !removals[1].NotReached {
		t.Fatalf("removals = %+v, want feat/a removed and feat/b not reached", removals)
	}
	if _, err := os.Stat(targets[0].Path); !os.IsNotExist(err) {
		t.Errorf("feat/a stat = %v, want it gone", err)
	}
	if branchExists(t, ctx.ProjectDir, "feat/a") {
		t.Error("feat/a's branch outlived its worktree")
	}
	if !slices.Contains(presenter.PublishedTypes(), domain.EventWorktreeRemoved) {
		t.Errorf("published %v, want the removal published", presenter.PublishedTypes())
	}
	if _, err := os.Stat(targets[1].Path); err != nil || !branchExists(t, ctx.ProjectDir, "feat/b") {
		t.Errorf("feat/b stat = %v, want it untouched", err)
	}
}

// Ctrl-C during an on_clean hook interrupts it and refuses the removal: the
// worktree is kept whole, and the item reads as cancelled, not failed.
func TestAnInterruptedCleanHookKeepsTheWorktree(t *testing.T) {
	globaldir.Isolate(t)
	processtest.Serve(t, nil)
	ctx := repoContext(t)
	ctx.Config.Project.Hooks.OnClean = []domain.HookCommand{{Cmd: "sleep 30"}}
	targets := []teardown.Target{makeTarget(t, ctx, "feat/a"), makeTarget(t, ctx, "feat/b")}
	runCtx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(300*time.Millisecond, cancel)

	removals := teardown.Batch(runCtx, teardown.BatchParams{Context: ctx, Presenter: &flowtest.Recorder{}, Targets: targets, ForceRemoval: true})

	if len(removals) != 2 || removals[0].Err == nil || !removals[1].NotReached {
		t.Fatalf("removals = %+v, want feat/a refused and feat/b not reached", removals)
	}
	if !errors.Is(removals[0].Err, domain.ErrCancelled) || rules.ExitCode(removals[0].Err) != domain.ExitCodeCancelled {
		t.Errorf("err = %v (exit %d), want it read as cancelled", removals[0].Err, rules.ExitCode(removals[0].Err))
	}
	if _, err := os.Stat(targets[0].Path); err != nil || !branchExists(t, ctx.ProjectDir, "feat/a") {
		t.Errorf("feat/a stat = %v, want it kept", err)
	}
}
