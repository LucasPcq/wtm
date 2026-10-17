package output

import (
	"bytes"
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/stretchr/testify/assert"
)

func hookRun(progress *Progress, lines ...string) {
	progress.Report(wtm.UnitStarted{Subject: "feat/x"})
	progress.Report(wtm.PhaseStarted{Subject: "feat/x", Phase: "worktree"})
	progress.Report(wtm.PhaseFinished{Subject: "feat/x", Phase: "worktree"})
	progress.Report(wtm.PhaseStarted{Subject: "feat/x", Phase: "on_create", Log: "/state/hooks/on_create-feat%2Fx.log"})
	for _, line := range lines {
		progress.Report(wtm.Output{Subject: "feat/x", Line: line})
	}
	progress.Report(wtm.PhaseFinished{Subject: "feat/x", Phase: "on_create"})
	progress.Report(wtm.UnitFinished{Subject: "feat/x", Status: wtm.StatusDone})
}

func TestOffATerminalAHookPhaseIsTitledBeforeItsFirstLine(t *testing.T) {
	var out bytes.Buffer
	progress := NewProgress(t.Context(), ProgressParams{W: &out, Human: true})

	hookRun(progress, "installing", "done")
	progress.Close()

	assert.Equal(t, "  on_create · feat/x\ninstalling\ndone\n", out.String())
}

func TestAPhaseThatPrintsNothingShowsNothing(t *testing.T) {
	var out bytes.Buffer
	progress := NewProgress(t.Context(), ProgressParams{W: &out, Human: true})

	hookRun(progress)
	progress.Close()

	assert.Empty(t, out.String())
}

func TestForJSONTheHookLinesAloneReachTheStream(t *testing.T) {
	var out bytes.Buffer
	progress := NewProgress(t.Context(), ProgressParams{W: &out})

	hookRun(progress, "installing")
	progress.Close()

	assert.Equal(t, "installing\n", out.String())
}

func TestAnInterruptSaysWhatIsBeingFinished(t *testing.T) {
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	progress := NewProgress(ctx, ProgressParams{W: &out, Human: true})
	progress.Report(wtm.UnitStarted{Subject: "feat/x"})
	progress.Report(wtm.PhaseStarted{Subject: "feat/x", Phase: "env"})

	cancel()
	assert.Eventually(t, func() bool {
		progress.mu.Lock()
		defer progress.mu.Unlock()
		return out.Len() > 0
	}, time2s, tick)
	progress.Close()

	assert.Equal(t, "  Cancelling — finishing env of feat/x; Ctrl+C again to stop now\n", out.String())
}

func TestAFinishedRunPrintsNoCancelling(t *testing.T) {
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	progress := NewProgress(ctx, ProgressParams{W: &out, Human: true})

	progress.Close()
	cancel()

	assert.Empty(t, out.String())
}

func TestTheCancellingMessageNamesWhatIsKnown(t *testing.T) {
	assert.Equal(t, "Cancelling — finishing feat/x; Ctrl+C again to stop now", (&Progress{unit: "feat/x"}).cancellingMessage())
	assert.Equal(t, "Cancelling — Ctrl+C again to stop now", (&Progress{}).cancellingMessage())
}
