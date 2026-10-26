package output

import (
	"bytes"
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestOffATerminalEachUnitIsALineAsItEndsItsOutputLeftOut(t *testing.T) {
	var out bytes.Buffer
	progress := NewParallel(t.Context(), ProgressParams{W: &out, Human: true})

	progress.Report(domain.UnitStarted{Subject: "feat/a"})
	progress.Report(domain.UnitStarted{Subject: "feat/b"})
	progress.Report(domain.Output{Subject: "feat/b", Line: "boom"})
	progress.Report(domain.UnitFinished{Subject: "feat/b", Status: domain.StatusFailed})
	progress.Report(domain.UnitFinished{Subject: "feat/a", Status: domain.StatusDone})
	progress.Close()

	assert.Regexp(t, `^  ✗ feat/b \(\d+ms\)\n  ✓ feat/a \(\d+ms\)\n$`, out.String())
}

func TestForJSONAParallelRunDrawsNothing(t *testing.T) {
	var out bytes.Buffer
	progress := NewParallel(t.Context(), ProgressParams{W: &out})

	progress.Report(domain.UnitStarted{Subject: "feat/a"})
	progress.Report(domain.Output{Subject: "feat/a", Line: "hi"})
	progress.Report(domain.UnitFinished{Subject: "feat/a", Status: domain.StatusDone})
	progress.Close()

	assert.Empty(t, out.String())
}

func TestAnInterruptSaysWhichUnitsAreBeingFinished(t *testing.T) {
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	progress := NewParallel(ctx, ProgressParams{W: &out, Human: true})
	progress.Report(domain.UnitStarted{Subject: "feat/a"})
	progress.Report(domain.UnitStarted{Subject: "feat/b"})

	cancel()
	assert.Eventually(t, func() bool {
		progress.mu.Lock()
		defer progress.mu.Unlock()
		return out.Len() > 0
	}, time2s, tick)
	progress.Close()

	assert.Equal(t, "  Cancelling — finishing feat/a, feat/b; Ctrl+C again to stop now\n", out.String())
}

func TestLiveTheUnitsRunningAreRowsErasedOnceTheRunEnds(t *testing.T) {
	var out bytes.Buffer
	progress := NewParallel(t.Context(), ProgressParams{W: &out, Human: true, Live: true})

	progress.Report(domain.UnitStarted{Subject: "feat/a"})
	progress.Report(domain.UnitFinished{Subject: "feat/a", Status: domain.StatusDone})
	progress.Close()

	assert.Regexp(t, `^  › feat/a\n\x1b\[1F\x1b\[J  ✓ feat/a \(\d+ms\)\n$`, out.String())
}
