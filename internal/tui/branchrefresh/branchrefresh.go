// Package branchrefresh wires the "r" refresh key into branch pickers: it
// re-fetches origin, recomputes the divergence-tagged candidates, and rebuilds
// the branch step in place when it is the one on screen. Each branch step must
// read its candidates from the shared holder (so a step entered later picks up the
// fresh list) and set Step.CanRefresh, which gates both the key and the rebuild.
package branchrefresh

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RefreshedMsg carries the re-fetched branch candidates back into the wizard.
type RefreshedMsg struct {
	Candidates []domain.BranchCandidate
}

// Cmd fetches origin and recomputes the branch candidates off the UI thread.
func Cmd(projectDir string) tea.Cmd {
	return CmdFunc(func() []domain.BranchCandidate {
		return branch.Refresh(branch.ListParams{ProjectDir: projectDir})
	})
}

// CmdFunc is Cmd for a caller that already holds the fetch itself.
func CmdFunc(fetch func() []domain.BranchCandidate) tea.Cmd {
	return func() tea.Msg { return RefreshedMsg{Candidates: fetch()} }
}

// HandleParams holds inputs for Handle.
type HandleParams struct {
	Wizard     *components.WizardModel
	Msg        tea.Msg
	ProjectDir string
	// Fetch takes precedence over ProjectDir.
	Fetch func() []domain.BranchCandidate
	// Holder is the candidate slice the branch steps' Build hooks read from; Handle
	// overwrites it with the fresh candidates before refreshing the current step.
	Holder *[]domain.BranchCandidate
}

func (p HandleParams) fetch() func() []domain.BranchCandidate {
	if p.Fetch != nil {
		return p.Fetch
	}
	return func() []domain.BranchCandidate {
		return branch.Refresh(branch.ListParams{ProjectDir: p.ProjectDir})
	}
}

// Handler returns a wizard message handler that wires the refresh key/message for
// a picker whose branch steps read from holder. Pickers that also handle async
// messages chain it first — it returns handled=false for messages it does not own.
func Handler(projectDir string, holder *[]domain.BranchCandidate) components.WizardMsgHandler {
	return HandlerFunc(func() []domain.BranchCandidate {
		return branch.Refresh(branch.ListParams{ProjectDir: projectDir})
	}, holder)
}

// HandlerFunc is Handler for a caller that already holds the fetch itself.
func HandlerFunc(fetch func() []domain.BranchCandidate, holder *[]domain.BranchCandidate) components.WizardMsgHandler {
	return func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
		return Handle(HandleParams{
			Wizard: w,
			Msg:    msg,
			Fetch:  fetch,
			Holder: holder,
		})
	}
}

// Handle processes the refresh key and the RefreshedMsg for a branch picker.
// Call it first from a picker's OnMsg: it returns handled=false for messages it
// does not own (the refresh key on a non-branch/filtering step, or any unrelated
// message) so the picker can chain its own handling.
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
		return tea.Batch(w.StartLoading(domain.LoadingBranchesText), CmdFunc(params.fetch())), true
	}

	if msg, ok := params.Msg.(RefreshedMsg); ok {
		*params.Holder = msg.Candidates
		w.RefreshCurrentStep()
		w.SetLoading(false)
		return nil, true
	}

	return nil, false
}
