package output

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	progress.Report(domain.UnitStarted{Subject: "feat/x"})
	progress.Report(domain.PhaseStarted{Subject: "feat/x", Phase: "env"})

	cancel()
	assert.Eventually(t, func() bool {
		progress.mu.Lock()
		defer progress.mu.Unlock()
		return out.Len() > 0
	}, time2s, tick)
	progress.Close()

	assert.Equal(t, "  Cancelling — finishing env of feat/x; Ctrl+C again to stop now\n", out.String())
}

func TestAnInterruptIsSaidBeforeTheEventsThatFollowIt(t *testing.T) {
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	progress := NewProgress(ctx, ProgressParams{W: &out, Human: true})
	progress.Report(domain.UnitStarted{Subject: "feat/x"})
	progress.Report(domain.PhaseStarted{Subject: "feat/x", Phase: "on_create"})

	cancel()
	progress.Report(domain.PhaseFinished{Subject: "feat/x", Phase: "on_create", Failed: true})
	progress.Report(domain.UnitFinished{Subject: "feat/x", Status: domain.StatusCancelled})
	progress.Close()

	assert.Equal(t, "  Cancelling — finishing on_create of feat/x; Ctrl+C again to stop now\n", out.String())
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

func TestALiveInterruptDuringAHookIsSaidInsideItsViewAndLosesNoLine(t *testing.T) {
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	progress := NewProgress(ctx, ProgressParams{W: &out, Human: true, Live: true})
	progress.Report(domain.UnitStarted{Subject: "feat/x"})
	progress.Report(domain.PhaseStarted{Subject: "feat/x", Phase: "on_create", Log: "/state/hooks/x.log"})
	progress.Report(domain.Output{Subject: "feat/x", Line: "before"})

	cancel()
	assert.Eventually(t, func() bool {
		progress.mu.Lock()
		defer progress.mu.Unlock()
		return strings.Contains(out.String(), "Cancelling")
	}, time2s, tick)
	during := finalScreen(out.String())
	progress.Report(domain.Output{Subject: "feat/x", Line: "after"})
	progress.Report(domain.PhaseFinished{Subject: "feat/x", Phase: "on_create", Failed: true})
	progress.Report(domain.UnitFinished{Subject: "feat/x", Status: domain.StatusCancelled})
	progress.Close()

	assert.Equal(t, []string{"  → on_create · feat/x", "  Cancelling — finishing on_create of feat/x; Ctrl+C again to stop now", "    before"}, during)
	screen := finalScreen(out.String())
	require.Len(t, screen, 4)
	assert.True(t, strings.HasPrefix(screen[0], "  ✗ on_create · feat/x ("), screen)
	assert.Equal(t, []string{"    before", "    after", "  full output:  /state/hooks/x.log"}, screen[1:])
}

func TestALiveSilentHookThatFailsStillSaysWhereItsLogIs(t *testing.T) {
	var out bytes.Buffer
	progress := NewProgress(t.Context(), ProgressParams{W: &out, Human: true, Live: true})
	progress.Report(domain.UnitStarted{Subject: "feat/x"})
	progress.Report(domain.PhaseStarted{Subject: "feat/x", Phase: "env"})
	progress.Report(domain.PhaseFinished{Subject: "feat/x", Phase: "env", Failed: true})
	progress.Report(domain.PhaseStarted{Subject: "feat/x", Phase: "on_create", Log: "/state/hooks/x.log"})
	progress.Report(domain.PhaseFinished{Subject: "feat/x", Phase: "on_create", Failed: true})
	progress.Report(domain.UnitFinished{Subject: "feat/x", Status: domain.StatusFailed})
	progress.Close()

	screen := finalScreen(out.String())
	require.Len(t, screen, 2, "a phase without a log, as env, stays silent")
	assert.True(t, strings.HasPrefix(screen[0], "  ✗ on_create · feat/x ("), screen)
	assert.Equal(t, "  full output:  /state/hooks/x.log", screen[1])
}

func TestALiveInterruptBetweenPhasesIsTheOnlyLine(t *testing.T) {
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	progress := NewProgress(ctx, ProgressParams{W: &out, Human: true, Live: true})
	progress.Report(domain.UnitStarted{Subject: "feat/x"})

	cancel()
	assert.Eventually(t, func() bool {
		progress.mu.Lock()
		defer progress.mu.Unlock()
		return strings.Contains(out.String(), "Cancelling")
	}, time2s, tick)
	progress.mu.Lock()
	screen := finalScreen(out.String())
	progress.mu.Unlock()
	progress.Close()

	assert.Equal(t, []string{"  › Cancelling — finishing feat/x; Ctrl+C again to stop now"}, screen)
}

func TestALiveLineIsErasedAloneSoNoScreenIsClearedIntoTheScrollback(t *testing.T) {
	var out bytes.Buffer
	progress := NewProgress(t.Context(), ProgressParams{W: &out, Human: true, Live: true})

	progress.Report(domain.UnitStarted{Subject: "feat/x"})
	progress.Report(domain.UnitFinished{Subject: "feat/x", Status: domain.StatusDone})
	progress.Close()

	assert.True(t, strings.HasSuffix(out.String(), "\x1b[1F\x1b[2K"), "%q", out.String())
	assert.NotContains(t, out.String(), "\x1b[J")
}
