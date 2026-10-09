package dashboard

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

// The git clock is slow on purpose, so the moments a stale row would show are
// the ones it is re-read on: coming back to the terminal, and an action ending.
func TestRegainingFocusRereadsTheRows(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")

	next, cmd := updateCmd(model, tea.FocusMsg{})
	if cmd == nil || !next.loading {
		t.Fatal("regaining focus must re-read the worktrees")
	}
}

// A run that failed or was abandoned sends no "done" of its own, and the row it
// held may have moved all the same.
func TestAnEndedActionRereadsTheRows(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model.params.JobsLoader = func(bool) ([]domain.JobInfo, bool) { return nil, true }
	model, id := model.beginOp(beginParams{Operation: flow.Operation{Kind: domain.OpKindSync, Mode: flow.ModeBlocking}, Target: "a"})

	next, cmd := model.finishOp(opDoneMsg{id: id, err: errors.New("rebase stopped")})
	if cmd == nil || !next.loading {
		t.Fatal("an action that ended must re-read the worktrees")
	}
}
