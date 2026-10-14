package render_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

func TestAMachineRunGetsTheHooksOutputAndNothingElse(t *testing.T) {
	var out bytes.Buffer
	view := render.NewProgressView(t.Context(), render.ProgressViewParams{Catalog: catalog, W: &out})

	emitRun(view)
	view.Close()

	assert.Equal(t, "installing\n", out.String())
}

func TestAHumanRunOffATerminalTitlesEachPhase(t *testing.T) {
	var out bytes.Buffer
	view := render.NewProgressView(t.Context(), render.ProgressViewParams{Catalog: catalog, W: &out, Human: true})

	emitRun(view)
	view.Close()

	assert.Equal(t, lines("  on_create · feat/a", "installing"), plain(out.String()))
}

func TestALiveRunErasesWhatItDrewAndKeepsTheLineOfEachPhase(t *testing.T) {
	var out bytes.Buffer
	view := render.NewProgressView(t.Context(), render.ProgressViewParams{Catalog: catalog, W: &out, Human: true, Live: true})

	emitRun(view)
	view.Close()

	screen := plain(out.String())
	assert.Contains(t, screen, "› feat/a")
	assert.Contains(t, screen, "› feat/a · ran without operation locks")
	assert.Contains(t, screen, "✓ on_create · feat/a (")
	assert.Contains(t, screen, "installing")
}

func TestACancelledRunSaysItIsCancelling(t *testing.T) {
	var out syncBuffer
	ctx, cancel := context.WithCancel(t.Context())
	view := render.NewProgressView(ctx, render.ProgressViewParams{Catalog: catalog, W: &out, Human: true, Live: true})
	view.Emit(&kernel.UnitStarted{Of: kernel.Of{Subject: "feat/a"}})

	cancel()

	assert.Eventually(t, func() bool { return bytes.Contains(out.Bytes(), []byte("Cancelling…")) }, waitFor, tick)
	view.Close()
}

func TestAClosedViewDrawsNothingMore(t *testing.T) {
	var out syncBuffer
	ctx, cancel := context.WithCancel(t.Context())
	view := render.NewProgressView(ctx, render.ProgressViewParams{Catalog: catalog, W: &out, Human: true, Live: true})
	view.Close()

	cancel()

	assert.Never(t, func() bool { return out.Len() > 0 }, waitFor, tick)
}

func TestAPhaseCutShortLeavesNoTailBehind(t *testing.T) {
	var out bytes.Buffer
	view := render.NewProgressView(t.Context(), render.ProgressViewParams{Catalog: catalog, W: &out, Human: true, Live: true})
	report := kernel.Report(view, "feat/a")
	report.UnitStarted()
	report.Phase("on_create")
	report.Output("installing")

	view.Close()

	assert.True(t, strings.HasSuffix(out.String(), domain.AnsiClearBelow), "%q", out.String())
	assert.NotContains(t, plain(out.String()), "✓")
}

func TestAPhaseThatPrintsNothingShowsNothing(t *testing.T) {
	for _, live := range []bool{false, true} {
		var out bytes.Buffer
		view := render.NewProgressView(t.Context(), render.ProgressViewParams{Catalog: catalog, W: &out, Human: true, Live: live})
		report := kernel.Report(view, "feat/a")

		report.Phase("worktree")()
		view.Close()

		assert.Empty(t, out.String(), "live: %v", live)
	}
}
