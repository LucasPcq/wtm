package dashboard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

// landPreview runs what opening the logs view asked for until the board lands,
// and applies it: a board is opened off the UI goroutine now, so a test has to
// play the part of bubbletea's command runner.
func landPreview(t *testing.T, model Model, cmd tea.Cmd) Model {
	t.Helper()
	for _, msg := range runAll(cmd) {
		if board, ok := msg.(previewBoardMsg); ok {
			return update(model, board)
		}
	}
	return model
}

// runAll runs a command and every command a batch holds, but never what a
// landed board asks for: a preview's Init opens a stream that never ends.
func runAll(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var msgs []tea.Msg
	for _, inner := range batch {
		msgs = append(msgs, runAll(inner)...)
	}
	return msgs
}

// Opening a board resolves the worktree's environment and dials the daemon:
// done inside Update, it froze the dashboard for as long as both took.
func TestTheBoardIsOpenedOffTheUIGoroutine(t *testing.T) {
	opened := 0
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{{Name: "web", Kind: domain.JobKindService, Status: domain.JobStatusStopped}},
	})
	model := logsModel(t, RunParams{
		BoardLoader: func(logsRequest) runlogs.Board { opened++; return board },
		LogsLoader:  func(logsRequest) ([]string, error) { return nil, nil },
	}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})

	model, cmd := model.openLogsTabOn("web")
	if opened != 0 || model.previewOn {
		t.Fatal("the board was opened inside Update")
	}
	model = landPreview(t, model, cmd)
	if opened != 1 || !model.previewOn {
		t.Fatalf("opened = %d, previewOn = %v: want the board landed as a message", opened, model.previewOn)
	}
}

// A board resolves the worktree's environment, which allocates it an ordinal:
// reading the logs of a worktree that never ran anything must not give it one.
func TestAWorktreeWithoutJobOrTraceOpensNoBoard(t *testing.T) {
	model := logsModel(t, RunParams{
		BoardLoader: func(logsRequest) runlogs.Board {
			t.Error("board opened for a worktree with nothing to show")
			return nil
		},
		LogsLoader: func(logsRequest) ([]string, error) {
			t.Error("tail read for a worktree with nothing to show")
			return nil, nil
		},
	}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}

	model, cmd := model.openLogsTabOn("web")
	landPreview(t, model, cmd)
}

// A board landing after the reader moved on is dropped rather than drawn under
// another worktree's name.
func TestALateBoardIsDropped(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{{Name: "web", Kind: domain.JobKindService, Status: domain.JobStatusStopped}},
	})
	model := logsModel(t, RunParams{LogsLoader: func(logsRequest) ([]string, error) { return nil, nil }}, "a", "b")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "b", "web"

	model = update(model, previewBoardMsg{branch: "a", board: board})
	if model.previewOn {
		t.Error("a board for a worktree no longer on screen was drawn")
	}
}
