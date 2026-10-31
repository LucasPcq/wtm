package output

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestOffATerminalEachUnitIsALineAsItEndsItsOutputLeftOut(t *testing.T) {
	var out bytes.Buffer
	p := NewConcurrent(t.Context(), ConcurrentParams{ProgressParams: ProgressParams{W: &out, Human: true}})

	p.Report(domain.UnitStarted{Subject: "feat/a"})
	p.Report(domain.UnitStarted{Subject: "feat/b"})
	p.Report(domain.Output{Subject: "feat/b", Line: "boom"})
	p.Report(domain.UnitFinished{Subject: "feat/b", Status: domain.StatusFailed})
	p.Report(domain.UnitFinished{Subject: "feat/a", Status: domain.StatusDone})
	p.Close()

	assert.Regexp(t, `^  ✗ feat/b \(\d+ms\)\n  ✓ feat/a \(\d+ms\)\n$`, out.String())
}

func TestForJSONTheProgressDrawsNothing(t *testing.T) {
	var out bytes.Buffer
	p := NewConcurrent(t.Context(), ConcurrentParams{ProgressParams: ProgressParams{W: &out}})

	p.Report(domain.UnitStarted{Subject: "feat/a"})
	p.Report(domain.Output{Subject: "feat/a", Line: "hi"})
	p.Report(domain.UnitFinished{Subject: "feat/a", Status: domain.StatusDone})
	p.Close()

	assert.Empty(t, out.String())
}

func TestAnInterruptSaysWhichUnitsAreBeingFinished(t *testing.T) {
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	p := NewConcurrent(ctx, ConcurrentParams{ProgressParams: ProgressParams{W: &out, Human: true}})
	p.Report(domain.UnitStarted{Subject: "feat/a"})
	p.Report(domain.UnitStarted{Subject: "feat/b"})

	cancel()
	assert.Eventually(t, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return out.Len() > 0
	}, 2*time.Second, 10*time.Millisecond)
	p.Close()

	assert.Equal(t, "  Cancelling — finishing feat/a, feat/b; Ctrl+C again to stop now\n", out.String())
}

func TestLiveTheUnitsRunningAreRowsErasedOnceTheRunEnds(t *testing.T) {
	var out bytes.Buffer
	p := NewConcurrent(t.Context(), ConcurrentParams{ProgressParams: ProgressParams{W: &out, Human: true, Live: true}})

	p.Report(domain.UnitStarted{Subject: "feat/a"})
	p.Report(domain.UnitFinished{Subject: "feat/a", Status: domain.StatusDone})
	p.Close()

	assert.Regexp(t, `^  › feat/a\n\x1b\[1F\x1b\[J  ✓ feat/a \(\d+ms\)\n$`, out.String())
}

func TestLiveTheRowsKeepTheirTimeWhileCancellingAndStopOnClose(t *testing.T) {
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	p := NewConcurrent(ctx, ConcurrentParams{ProgressParams: ProgressParams{W: &out, Human: true, Live: true}, Tick: 10 * time.Millisecond})
	p.Report(domain.UnitStarted{Subject: "feat/a"})

	cancel()
	assert.Eventually(t, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return strings.Count(out.String(), "Cancelling") >= 3
	}, 2*time.Second, 10*time.Millisecond, "the notice and the rows are redrawn while cancelling")
	p.Close()
	p.mu.Lock()
	closed := out.Len()
	p.mu.Unlock()
	time.Sleep(50 * time.Millisecond)

	p.mu.Lock()
	defer p.mu.Unlock()
	assert.Equal(t, closed, out.Len(), "nothing is drawn once closed")
}

func TestARowPerJobEndsAsItsPhaseEndsTheWorktreeLeftOut(t *testing.T) {
	var out bytes.Buffer
	p := NewConcurrent(t.Context(), ConcurrentParams{ProgressParams: ProgressParams{W: &out, Human: true}, Rows: RowPerJob})

	p.Report(domain.UnitStarted{Subject: "feat/a"})
	p.Report(domain.PhaseStarted{Subject: "feat/a", Phase: domain.PhaseJob, Job: "migrate"})
	p.Report(domain.Output{Subject: "feat/a", Job: "migrate", Line: "migrating"})
	p.Report(domain.PhaseFinished{Subject: "feat/a", Phase: domain.PhaseJob, Job: "migrate"})
	p.Report(domain.PhaseStarted{Subject: "feat/a", Phase: domain.PhaseJob, Job: "web"})
	p.Report(domain.PhaseFinished{Subject: "feat/a", Phase: domain.PhaseJob, Job: "web", Failed: true})
	p.Report(domain.UnitFinished{Subject: "feat/a", Status: domain.StatusFailed})
	p.Close()

	assert.Regexp(t, `^  ✓ feat/a · migrate \(\d+ms\)\n  ✗ feat/a · web \(\d+ms\)\n$`, out.String())
}
