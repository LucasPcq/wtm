package dashboard

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
)

func drain(msgs chan tea.Msg) (lines []string, askedForTerminal bool) {
	close(msgs)
	for msg := range msgs {
		if _, asked := msg.(handoffMsg); asked {
			askedForTerminal = true
		}
		if line, ok := msg.(OutputLineMsg); ok {
			lines = append(lines, line.Text)
		}
	}
	return lines, askedForTerminal
}

func TestTheDetachedWatcherNeverAsksForTheTerminal(t *testing.T) {
	msgs := make(chan tea.Msg, 32)
	watcher := detachedWatcher{send: func(msg tea.Msg) { msgs <- msg }, id: 1}

	if _, err := watcher.Sequence(seam.SequenceParams{
		Profile: "dev",
		Start: func(_ context.Context, sink runlogs.Sink) (runlogs.Outcomes, error) {
			sink.Emit(runlogs.Event{Phase: runlogs.PhaseStarting, Job: "web", Step: 1, Steps: 2})
			sink.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "web", URL: "http://web.wtm"})
			return runlogs.Outcomes{{}}, nil
		},
	}); err != nil {
		t.Fatalf("Sequence: %v", err)
	}

	lines, asked := drain(msgs)
	if asked {
		t.Fatal("the detached watcher asked for the terminal")
	}
	body := strings.Join(lines, "\n")
	if !strings.Contains(body, "web") {
		t.Errorf("output = %q, want each job named as it starts", body)
	}
	if !strings.Contains(body, "http://web.wtm") {
		t.Errorf("output = %q, want the address reported: it is what one starts a job for", body)
	}
}

func TestTheDetachedWatcherReportsAFailedJob(t *testing.T) {
	msgs := make(chan tea.Msg, 8)
	watcher := detachedWatcher{send: func(msg tea.Msg) { msgs <- msg }, id: 1}

	if _, err := watcher.Sequence(seam.SequenceParams{
		Job: "web",
		Start: func(_ context.Context, sink runlogs.Sink) (runlogs.Outcomes, error) {
			sink.Emit(runlogs.Event{Phase: runlogs.PhaseFailed, Job: "web", Reason: "port already bound"})
			return runlogs.Outcomes{{}}, nil
		},
	}); err != nil {
		t.Fatalf("Sequence: %v", err)
	}

	lines, _ := drain(msgs)
	if body := strings.Join(lines, "\n"); !strings.Contains(body, "port already bound") {
		t.Errorf("output = %q, want the daemon's reason quoted: a silent failure reads as a success", body)
	}
}

// A raw output chunk belongs to the logs view, not to a three-line output panel.
func TestTheDetachedWatcherKeepsRawOutputOffTheOutputPanel(t *testing.T) {
	msgs := make(chan tea.Msg, 8)
	watcher := detachedWatcher{send: func(msg tea.Msg) { msgs <- msg }, id: 1}

	if _, err := watcher.Sequence(seam.SequenceParams{
		Start: func(_ context.Context, sink runlogs.Sink) (runlogs.Outcomes, error) {
			sink.Emit(runlogs.Event{Phase: runlogs.PhaseOutput, Job: "web", Chunk: []byte("noisy build log\n")})
			return runlogs.Outcomes{{}}, nil
		},
	}); err != nil {
		t.Fatalf("Sequence: %v", err)
	}

	lines, _ := drain(msgs)
	if body := strings.Join(lines, "\n"); strings.Contains(body, "noisy build log") {
		t.Errorf("output = %q, want raw chunks left to the logs view", body)
	}
}

func TestTheDetachedWatcherReturnsTheOutcomeToTheFlow(t *testing.T) {
	watcher := detachedWatcher{send: func(tea.Msg) {}, id: 1}

	outcomes, err := watcher.Sequence(seam.SequenceParams{
		Start: func(context.Context, runlogs.Sink) (runlogs.Outcomes, error) {
			return runlogs.Outcomes{runlogs.Outcome{Started: []string{"web"}}}, nil
		},
	})
	if err != nil {
		t.Fatalf("Sequence: %v", err)
	}
	outcome := outcomes.One()
	if len(outcome.Started) != 1 || outcome.Started[0] != "web" {
		t.Errorf("outcome = %+v, want what the sequence concluded handed back to the flow", outcome)
	}
}

// Starting three worktrees in a row is the case this serves: each one used to
// cost an open and an exit of the run view. Only `View logs` takes the
// terminal now, and it takes it on purpose.
func TestStartingAProfileFromTheDashboardNeverAsksForTheTerminal(t *testing.T) {
	stateDir := t.TempDir()
	if err := runconfig.Save(runconfig.SaveParams{
		StateDir: stateDir,
		Config: domain.RunConfig{Jobs: []domain.JobConfig{
			{Name: "web", Kind: domain.JobKindService, Cmd: "true"},
		}},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	model := New(RunParams{StateDir: stateDir})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{statuses: statuses("a"), parents: map[string]string{}})

	next, cmd := model.startRunUp(model.statuses[0])
	if cmd == nil {
		t.Fatal("cmd = nil, want the run started")
	}

	// The flow runs on its own goroutine and reaches the model only through
	// messages; a hand-over would show up as a handoffMsg on that channel, and
	// it would be the very first thing posted — the view has to be drawing
	// before the first job is asked for. There is no daemon here, so the run
	// itself never concludes: the window is what the assertion needs, not the
	// end of the run.
	go cmd()
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case msg := <-next.msgs:
			if _, asked := msg.(handoffMsg); asked {
				t.Fatal("starting a profile asked for the terminal, want it detached")
			}
			if _, done := msg.(opDoneMsg); done {
				return
			}
		case <-deadline:
			return
		}
	}
}

// Three worktrees starting into one panel are nine indistinguishable lines
// unless each says where it came from. Same rule as the CLI: named above
// several, left out above one.
func TestTheDetachedWatcherNamesTheWorktreeAboveSeveralOfThem(t *testing.T) {
	msgs := make(chan tea.Msg, 32)
	watcher := detachedWatcher{send: func(msg tea.Msg) { msgs <- msg }, id: 1}

	if _, err := watcher.Sequence(seam.SequenceParams{
		Profile:   "dev",
		Worktrees: []string{"feat-a", "feat-b"},
		Start: func(_ context.Context, sink runlogs.Sink) (runlogs.Outcomes, error) {
			sink.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "web", Worktree: "feat-a"})
			sink.Emit(runlogs.Event{Phase: runlogs.PhaseDone, Job: "web", Worktree: "feat-b"})
			return runlogs.Outcomes{{}}, nil
		},
	}); err != nil {
		t.Fatalf("Sequence: %v", err)
	}

	body := strings.Join(mustLines(t, msgs), "\n")
	if !strings.Contains(body, "feat-a") || !strings.Contains(body, "feat-b") {
		t.Errorf("output = %q, want each line to name the worktree it came from", body)
	}
}

func TestTheDetachedWatcherSaysNothingAboutASingleWorktree(t *testing.T) {
	msgs := make(chan tea.Msg, 32)
	watcher := detachedWatcher{send: func(msg tea.Msg) { msgs <- msg }, id: 1}

	if _, err := watcher.Sequence(seam.SequenceParams{
		Profile:   "dev",
		Worktrees: []string{"feat-a"},
		Start: func(_ context.Context, sink runlogs.Sink) (runlogs.Outcomes, error) {
			sink.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "web", Worktree: "feat-a"})
			return runlogs.Outcomes{{}}, nil
		},
	}); err != nil {
		t.Fatalf("Sequence: %v", err)
	}

	if body := strings.Join(mustLines(t, msgs), "\n"); strings.Contains(body, "feat-a") {
		t.Errorf("output = %q, want the worktree left out: naming it repeats what was just ticked", body)
	}
}

// The stage a locked row shows is posted against the worktree it came from: a
// run holding three rows would otherwise write the same text on all three.
func TestTheDetachedWatcherPostsItsStageAgainstItsWorktree(t *testing.T) {
	msgs := make(chan tea.Msg, 32)
	watcher := detachedWatcher{send: func(msg tea.Msg) { msgs <- msg }, id: 7}

	if _, err := watcher.Sequence(seam.SequenceParams{
		Worktrees: []string{"feat-a", "feat-b"},
		Start: func(_ context.Context, sink runlogs.Sink) (runlogs.Outcomes, error) {
			sink.Emit(runlogs.Event{Phase: runlogs.PhaseStarting, Job: "web", Worktree: "feat-b", Step: 1, Steps: 2})
			return runlogs.Outcomes{{}}, nil
		},
	}); err != nil {
		t.Fatalf("Sequence: %v", err)
	}

	close(msgs)
	for msg := range msgs {
		stage, ok := msg.(opStageMsg)
		if !ok {
			continue
		}
		if stage.target != "feat-b" {
			t.Errorf("stage target = %q, want the worktree the step happened in", stage.target)
		}
		return
	}
	t.Fatal("no stage was posted")
}

func mustLines(t *testing.T, msgs chan tea.Msg) []string {
	t.Helper()
	lines, _ := drain(msgs)
	if len(lines) == 0 {
		t.Fatal("nothing was reported")
	}
	return lines
}
