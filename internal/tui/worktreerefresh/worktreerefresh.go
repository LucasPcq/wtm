// Package worktreerefresh wires the "r" refresh key into interactive worktree
// lists: it re-fetches origin, recomputes the worktree statuses (origin
// divergence, dirty, base-ahead), and rebuilds the current wizard step in place.
// The worktree step must read its rows from the shared holder (so a rebuild picks
// up the fresh statuses) and set Step.CanRefresh so the key is gated to it only.
// It mirrors branchrefresh, typed to worktree statuses instead of branch candidates.
package worktreerefresh

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RefreshedMsg carries the re-listed worktree statuses back into the wizard.
type RefreshedMsg struct {
	Statuses []domain.WorktreeStatus
}

// Cmd fetches origin and re-lists worktrees off the UI thread. A list error
// yields nil statuses so Handle leaves the holder unchanged.
func Cmd(params domain.ListParams) tea.Cmd {
	return func() tea.Msg {
		statuses, _ := worktree.Refresh(params)
		return RefreshedMsg{Statuses: statuses}
	}
}

// HandleParams holds inputs for Handle.
type HandleParams struct {
	Wizard     *components.WizardModel
	Msg        tea.Msg
	ListParams domain.ListParams
	// Holder is the status slice the worktree step's Build hook reads from; Handle
	// overwrites it with the fresh statuses before rebuilding the current step.
	Holder *[]domain.WorktreeStatus
}

// Handle processes the refresh key and RefreshedMsg for a worktree picker. Call
// it first from a picker's OnMsg: it returns handled=false for messages it does
// not own (the refresh key on a non-worktree/filtering step, or any unrelated
// message) so the picker can chain its own handling. A refresh that returns no
// statuses (list error) leaves the holder unchanged.
func Handle(params HandleParams) (tea.Cmd, bool) {
	w := params.Wizard

	if key, ok := params.Msg.(tea.KeyMsg); ok && key.String() == domain.KeyRefresh {
		if !w.CurrentStepCanRefresh() || w.Loading() {
			return nil, false
		}
		sl, ok := w.CurrentStepModel().(components.SelectListModel)
		if !ok || sl.Filtering() {
			return nil, false
		}
		return tea.Batch(w.StartLoading(domain.LoadingWorktreesText), Cmd(params.ListParams)), true
	}

	if msg, ok := params.Msg.(RefreshedMsg); ok {
		if len(msg.Statuses) > 0 {
			*params.Holder = msg.Statuses
		}
		w.RebuildCurrentStep()
		w.SetLoading(false)
		return nil, true
	}

	return nil, false
}
