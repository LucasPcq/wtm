package dashboard

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

func sequenceLines(t *testing.T, emit func(runlogs.Sink)) string {
	t.Helper()
	msgs := make(chan tea.Msg, 64)
	watcher := detachedWatcher{send: func(msg tea.Msg) { msgs <- msg }, id: 1}
	if _, err := watcher.Sequence(seam.SequenceParams{
		Start: func(_ context.Context, sink runlogs.Sink) (runlogs.Outcomes, error) {
			emit(sink)
			return runlogs.Outcomes{{}}, nil
		},
	}); err != nil {
		t.Fatalf("Sequence: %v", err)
	}
	lines, _ := drain(msgs)
	return strings.Join(lines, "\n")
}

// A ✓ over a dead process is the one line that must never stand alone: the CLI
// corrects it, and so must the panel.
func TestTheDetachedSinkReportsACrash(t *testing.T) {
	body := sequenceLines(t, func(sink runlogs.Sink) {
		sink.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "web"})
		sink.Emit(runlogs.Event{Phase: runlogs.PhaseCrashed, Job: "web", Reason: "crashed"})
	})
	if !strings.Contains(body, "web exited right after starting") {
		t.Errorf("output = %q, want the crash named", body)
	}
}

func TestTheDetachedSinkReportsAWarning(t *testing.T) {
	body := sequenceLines(t, func(sink runlogs.Sink) {
		sink.Emit(runlogs.Event{Phase: runlogs.PhaseWarning, Job: "postgres", Notice: "postgres: could not record"})
	})
	if !strings.Contains(body, domain.GlyphAttention+" postgres: could not record") {
		t.Errorf("output = %q, want the warning in the attention register", body)
	}
}

func TestTheDetachedSinkReportsPortsNobodyBound(t *testing.T) {
	body := sequenceLines(t, func(sink runlogs.Sink) {
		sink.Emit(runlogs.Event{Phase: runlogs.PhaseProbed, Job: "web", Probes: []domain.PortProbe{
			{Job: "web", Name: "PORT", Port: 4001, Status: domain.PortSilent},
		}})
	})
	if !strings.Contains(body, domain.PortProbeTitle) || !strings.Contains(body, "4001") {
		t.Errorf("output = %q, want the silent port reported", body)
	}
}

func TestTheDetachedSinkSaysWhatAnAbortLeftBehind(t *testing.T) {
	body := sequenceLines(t, func(sink runlogs.Sink) {
		sink.Emit(runlogs.Event{Phase: runlogs.PhaseAborted, Outcome: runlogs.Outcome{
			Failed: "migrate", FailedStep: 2, Steps: 3, Started: []string{"db"}, NotStarted: []string{"web"},
		}})
	})
	if !strings.Contains(body, "db") || !strings.Contains(body, "web") {
		t.Errorf("output = %q, want what is left running and what never started", body)
	}
}

// The panel held the steps and nothing after them: a run that ended read the
// same as one still going.
func TestARunConcludesInThePanel(t *testing.T) {
	cases := []struct {
		name    string
		outcome runlogs.Outcome
		want    string
	}{
		{"up", runlogs.Outcome{Worktree: "feat/x", Steps: 2, Started: []string{"web"}}, "✓ run-up feat/x"},
		{"aborted", runlogs.Outcome{Worktree: "feat/x", Steps: 3, Failed: "migrate", FailedStep: 2}, "aborted at step 2/3 (migrate)"},
		{"crashed", runlogs.Outcome{Worktree: "feat/x", Steps: 1, Crashed: []domain.JobExit{{Job: "web"}}}, "web exited"},
	}
	for _, tc := range cases {
		got := strings.Join(runConclusion(runConclusionParams{Kind: domain.OpKindRunUp, Outcomes: runlogs.Outcomes{tc.outcome}}), "\n")
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: conclusion = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A run that never got as far as a job — refused, or answered with esc — has
// already said so, or has nothing to say.
func TestARunThatNeverStartedConcludesNothing(t *testing.T) {
	if got := runConclusion(runConclusionParams{Kind: domain.OpKindRunUp, Outcomes: runlogs.Outcomes{{}}}); len(got) != 0 {
		t.Errorf("conclusion = %q, want none", got)
	}
}
