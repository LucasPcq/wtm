package components

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// RunWizardParams holds inputs for running a wizard to completion.
type RunWizardParams struct {
	Steps []Step
	// Stderr renders the program to stderr instead of stdout — used by pickers
	// reached through a shell wrapper that captures stdout.
	Stderr bool
	// ErrLabel prefixes a program failure (e.g. "sync picker").
	ErrLabel string
	// OnMsg, when set, intercepts every message before the current step (e.g. to
	// wire an in-place refresh). The wizard is then built with async support.
	OnMsg WizardMsgHandler
	// InitCmd runs once on start (e.g. a background branch fetch); Loading/LoadingText
	// show the shared loading callout while it runs.
	InitCmd     tea.Cmd
	Loading     bool
	LoadingText string
}

// RunWizard builds and runs a wizard as a standalone tea.Program and returns the
// completed model. A user Esc (abort) maps to domain.ErrUserAborted; a program
// failure is wrapped with ErrLabel. It centralises the program/assertion/abort
// boilerplate every picker would otherwise repeat.
func RunWizard(params RunWizardParams) (WizardModel, error) {
	wiz := NewWizardWithParams(WizardParams{
		Steps:       params.Steps,
		OnMsg:       params.OnMsg,
		InitCmd:     params.InitCmd,
		Loading:     params.Loading,
		LoadingText: params.LoadingText,
	})

	opts := []tea.ProgramOption{}
	if params.Stderr {
		opts = append(opts, tea.WithOutput(os.Stderr))
	}

	finalModel, err := tea.NewProgram(wiz, opts...).Run()
	if err != nil {
		return WizardModel{}, fmt.Errorf("%s: %w", params.ErrLabel, ProgramError(err))
	}

	final, ok := finalModel.(WizardModel)
	if !ok || final.Aborted() {
		return WizardModel{}, domain.ErrUserAborted
	}
	return final, nil
}

// standaloneModel wraps a child model for use as a standalone tea.Program.
// It adds a title, description, and help bar around the child's View.
type standaloneModel struct {
	child   any
	title   string
	desc    string
	width   int
	done    bool
	aborted bool
}

// RunStandaloneSelect runs a SelectList as a standalone tea.Program.
// Returns the selected value, or ErrAborted on Esc or Ctrl-C.
func RunStandaloneSelect(sl SelectListModel) (string, error) {
	m := standaloneModel{
		child: sl,
		title: sl.title,
		desc:  sl.desc,
		width: 80,
	}

	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("select: %w", ProgramError(err))
	}

	final, ok := finalModel.(standaloneModel)
	if !ok || final.aborted {
		return "", ErrAborted
	}

	child, ok := final.child.(SelectListModel)
	if !ok {
		return "", ErrAborted
	}

	return child.Value(), nil
}

// RunStandaloneConfirm runs a ConfirmModel as a standalone tea.Program.
// Returns true for Yes, false for No, or ErrAborted on Esc or Ctrl-C.
func RunStandaloneConfirm(cm ConfirmModel) (bool, error) {
	m := standaloneModel{
		child: cm,
		title: cm.title,
		desc:  cm.desc,
		width: 80,
	}

	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return false, fmt.Errorf("confirm: %w", ProgramError(err))
	}

	final, ok := finalModel.(standaloneModel)
	if !ok || final.aborted {
		return false, ErrAborted
	}

	child, ok := final.child.(ConfirmModel)
	if !ok {
		return false, ErrAborted
	}

	return child.Confirmed(), nil
}

// ErrAborted is returned when the user presses Esc or Ctrl-C in a standalone
// component: the user backing out, as a wizard reports it.
var ErrAborted = fmt.Errorf("%w", domain.ErrUserAborted)

func (m standaloneModel) Init() tea.Cmd {
	switch child := m.child.(type) {
	case SelectListModel:
		return child.Init()
	case ConfirmModel:
		return child.Init()
	}
	return nil
}

func (m standaloneModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = wsMsg.Width
		switch child := m.child.(type) {
		case SelectListModel:
			child.width = m.width
			child.height = max(1, wsMsg.Height-6)
			m.child = child
		case ConfirmModel:
			child.width = m.width
			m.child = child
		}
		return m, nil
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == domain.KeyInterrupt {
		m.aborted = true
		return m, tea.Quit
	}

	switch child := m.child.(type) {
	case SelectListModel:
		updated, cmd := child.Update(msg)
		m.child = updated
		if updated.Chosen() {
			m.done = true
			return m, tea.Quit
		}
		if updated.Aborted() {
			m.aborted = true
			return m, tea.Quit
		}
		return m, cmd
	case ConfirmModel:
		updated, cmd := child.Update(msg)
		m.child = updated
		if updated.Done() {
			m.done = true
			return m, tea.Quit
		}
		if updated.Aborted() {
			m.aborted = true
			return m, tea.Quit
		}
		return m, cmd
	}

	return m, nil
}

func (m standaloneModel) View() string {
	if m.done || m.aborted {
		return ""
	}

	out := "\n"

	if m.title != "" {
		out += styles.BreadcrumbActive.Render(styles.Indent + m.title)
		out += "\n\n"
	}

	if m.desc != "" {
		for _, line := range strings.Split(m.desc, "\n") {
			out += styles.Muted.Render(styles.Indent + line)
			out += "\n"
		}
		out += "\n"
	}

	switch child := m.child.(type) {
	case SelectListModel:
		out += child.View()
	case ConfirmModel:
		out += child.View()
	}

	out += "\n\n"

	if sl, ok := m.child.(SelectListModel); ok && sl.filtering {
		out += styles.HelpBar.Render(sl.filterHelpHint())
		out += "\n"
		return out
	}

	// Both standalone child types (SelectList, Confirm) navigate vertically.
	help := "  ↑↓ navigate • enter confirm"
	if _, ok := m.child.(SelectListModel); ok {
		help += " • / filter"
	}
	help += " • esc cancel"
	out += styles.HelpBar.Render(help)
	out += "\n"

	return out
}
