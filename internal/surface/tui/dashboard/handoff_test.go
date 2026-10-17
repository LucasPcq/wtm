package dashboard

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

// The flow goroutine must not touch the model: it asks for the terminal through
// a message and blocks until the view is done, the same way a prompt does.
func TestTheWatcherBlocksUntilTheViewGivesTheTerminalBack(t *testing.T) {
	msgs := make(chan tea.Msg, 1)
	done := make(chan runlogs.Outcomes, 1)

	go func() {
		outcomes, err := watcher{send: func(msg tea.Msg) { msgs <- msg }}.Sequence(seam.SequenceParams{Job: "web"})
		if err != nil {
			t.Errorf("Sequence: %v", err)
		}
		done <- outcomes
	}()

	var asked handoffMsg
	select {
	case msg := <-msgs:
		got, ok := msg.(handoffMsg)
		if !ok {
			t.Fatalf("msg = %T, want a handoffMsg", msg)
		}
		asked = got
	case <-time.After(2 * time.Second):
		t.Fatal("the watcher never asked for the terminal")
	}
	if asked.params.Job != "web" {
		t.Errorf("the hand-over names %q, want the job the flow resolved", asked.params.Job)
	}

	select {
	case <-done:
		t.Fatal("the watcher returned before the view had run")
	case <-time.After(20 * time.Millisecond):
	}

	asked.reply <- handoffReply{outcomes: runlogs.Outcomes{{Steps: 3}}}
	select {
	case outcomes := <-done:
		if outcomes.One().Steps != 3 {
			t.Errorf("outcome = %+v, want what the view concluded", outcomes)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the watcher never came back")
	}
}

// bubbletea restores the alternate screen, the bracketed paste and the focus
// reporting after an Exec, but never the mouse tracking it turned off — and
// every click target on this surface depends on it.
func TestTheTerminalComesBackWithItsMouse(t *testing.T) {
	reply := make(chan handoffReply, 1)
	model := Model{}

	next, cmd := model.finishHandoff(handoffDoneMsg{
		reply:    reply,
		outcomes: runlogs.Outcomes{{Steps: 2}},
		recap:    "web started",
	})

	if cmd == nil {
		t.Fatal("the terminal came back without asking for the mouse again")
	}
	if msg := cmd(); msg == nil {
		t.Error("the mouse command produced nothing")
	}
	select {
	case answered := <-reply:
		if answered.outcomes.One().Steps != 2 {
			t.Errorf("the flow was unblocked with %+v, want the view's outcome", answered.outcomes)
		}
	default:
		t.Fatal("the flow was never unblocked")
	}
	if got := strings.Join(next.outputLines, "\n"); !strings.Contains(got, "web started") {
		t.Errorf("output = %q, want the view's recap kept", got)
	}
}

// A project with no run module has nothing to start, so the row says so rather
// than opening an empty picker.
func TestRunActionsRefuseWithoutARunModule(t *testing.T) {
	model := Model{}
	model.params.StateDir = t.TempDir()

	for name, start := range map[string]func(domain.WorktreeStatus) (Model, tea.Cmd){
		"up":   model.startRunUp,
		"down": model.startRunDown,
	} {
		next, cmd := start(domain.WorktreeStatus{Branch: "feature", Path: "/wt/feature"})
		if cmd != nil {
			t.Errorf("run %s started something without a run module", name)
		}
		if got := strings.Join(next.outputLines, "\n"); !strings.Contains(got, domain.DashboardRunNotConfigured) {
			t.Errorf("run %s said %q, want it to name the missing run module", name, got)
		}
	}

	// watchLogs reads the branch its logs view is on, so it takes no argument.
	next, cmd := model.watchLogs()
	if cmd != nil {
		t.Error("run logs started something without a run module")
	}
	if got := strings.Join(next.outputLines, "\n"); !strings.Contains(got, domain.DashboardRunNotConfigured) {
		t.Errorf("run logs said %q, want it to name the missing run module", got)
	}
}
