package form

import (
	"encoding/json"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

// pendingMsg starts the load the step just built asked for: a Build cannot
// return a command, the step's OnEnter (or the wizard's first one) can.
type pendingMsg struct{}

func pending() tea.Msg { return pendingMsg{} }

type choicesMsg struct {
	index   int
	key     string
	choices kernel.Choices
	err     error
}

type refreshedMsg struct {
	observed dispatch.Observed
	err      error
}

type previewMsg struct {
	reply dispatch.Reply
	err   error
}

func (s *session) handle(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
	if s.err != nil {
		return tea.Quit, true
	}
	switch m := msg.(type) {
	case pendingMsg:
		return s.start(w), true
	case tea.WindowSizeMsg:
		s.resized(w, m)
		return nil, false
	case choicesMsg:
		return s.chosen(w, m), true
	case refreshedMsg:
		return s.refreshed(w, m), true
	case previewMsg:
		s.previewed(w, m)
		return nil, true
	case tea.KeyMsg:
		if m.String() == domain.KeyRefresh && s.refreshable(w) {
			return tea.Batch(w.StartLoading(refreshText), s.refresh), true
		}
	}
	return nil, false
}

func (s *session) start(w *components.WizardModel) tea.Cmd {
	next := s.pending
	s.pending = nil
	if next == nil {
		return nil
	}
	return tea.Batch(w.StartLoading(next.text), next.run)
}

// choicesFor is the field's choices, from the cache keyed by what they depend
// on; a miss leaves the load pending and the field with no option yet.
func (s *session) choicesFor(params fieldParams) kernel.Choices {
	spec := s.fields[params.Index]
	if spec.Choices == kernel.ChoicesNone {
		return kernel.Choices{Options: options(kernel.Choices{}, spec.Constraints.Enum)}
	}
	key := choicesKey(spec, params.Form)
	if cached, found := s.choices[key]; found {
		return kernel.Choices{Options: options(cached, spec.Constraints.Enum), Pinned: cached.Pinned, Banner: cached.Banner}
	}
	request, facts := params.Request, s.facts
	s.pending = &load{text: choicesText, run: func() tea.Msg {
		choices, err := s.params.Entry.Choices(s.ctx, dispatch.FillChoicesParams{Path: spec.Path, Request: request, Facts: facts, IgnoreRemembered: s.params.Options.IgnoreRemembered})
		return choicesMsg{index: params.Index, key: key, choices: choices, err: err}
	}}
	return kernel.Choices{}
}

// choicesKey is the field and the values of what it depends on: choices are
// fetched again only when one of those changes.
func choicesKey(spec kernel.FieldSpec, form kernel.Form[json.RawMessage]) string {
	parts := []string{spec.Path}
	for _, path := range spec.DependsOn {
		encoded, _ := json.Marshal(stateAt(form, path).Value)
		parts = append(parts, string(encoded))
	}
	return strings.Join(parts, keySeparator)
}

func (s *session) chosen(w *components.WizardModel, m choicesMsg) tea.Cmd {
	w.SetLoading(false)
	if m.err != nil {
		s.fail(m.err)
		return tea.Quit
	}
	s.choices[m.key] = m.choices
	if m.choices.Banner != nil {
		w.SetBanner(components.WizardBanner{Title: s.message(m.choices.Banner.Code, m.choices.Banner.Params)})
	}
	if m.index == s.at {
		w.UpdateStepModel(m.index, func(any) any { return s.build(m.index, w.Steps()[:m.index]) })
	}
	return nil
}

// refreshable is a searched list not being filtered: there r types a letter.
func (s *session) refreshable(w *components.WizardModel) bool {
	list, isList := w.CurrentStepModel().(components.SelectListModel)
	return w.CurrentStepCanRefresh() && !w.Loading() && isList && !list.Filtering()
}

// refresh observes again: the facts the choices came from are stale, and so is
// every choice cached on them.
func (s *session) refresh() tea.Msg {
	observed, err := s.params.Entry.Observe(s.ctx)
	return refreshedMsg{observed: observed, err: err}
}

func (s *session) refreshed(w *components.WizardModel, m refreshedMsg) tea.Cmd {
	w.SetLoading(false)
	if m.err != nil {
		s.fail(m.err)
		return tea.Quit
	}
	s.setFacts(m.observed)
	w.RefreshCurrentStep()
	return s.start(w)
}
