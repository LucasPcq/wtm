package run

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

type viewCall struct {
	Job string
	// Attached reports that the view was given a start sequence to drive, as
	// opposed to opening on jobs that are already running.
	Attached bool
	Outcome  runlogs.Outcome
}

// viewRecorder stands in for the full-screen surface and drives the start
// sequence the way the view does, so a test reads both the handover and what
// the run went on to do.
type viewRecorder struct {
	calls []viewCall
	sink  runlogstest.Sink
}

func captureRunView(t *testing.T) *viewRecorder {
	t.Helper()

	recorder := &viewRecorder{}
	original := showRunView
	showRunView = recorder.show
	t.Cleanup(func() { showRunView = original })
	return recorder
}

func (r *viewRecorder) show(params viewParams) (runlogs.Outcomes, error) {
	call := viewCall{Job: params.Job, Attached: params.Start != nil}
	if params.Start != nil {
		outcomes, err := params.Start(context.Background(), &r.sink)
		if err != nil {
			return nil, err
		}
		call.Outcome = outcomes.One()
	}
	r.calls = append(r.calls, call)
	return runlogs.Outcomes{call.Outcome}, nil
}

func (r *viewRecorder) only(t *testing.T) viewCall {
	t.Helper()
	if len(r.calls) != 1 {
		t.Fatalf("the view was opened %d times, want once", len(r.calls))
	}
	return r.calls[0]
}

// fakeTTY answers the terminal question for the length of the test: nothing
// else makes the pipe a test runs on look like a terminal.
func fakeTTY(t *testing.T, terminal bool) {
	t.Helper()

	original := runctx.IsTTY
	runctx.IsTTY = func() bool { return terminal }
	t.Cleanup(func() { runctx.IsTTY = original })
}
