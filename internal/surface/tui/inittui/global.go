package inittui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RunGlobalWizard presents the global config setup wizard.
// Returns ErrUserAborted if the user presses Esc at the first step.
func RunGlobalWizard() (domain.InitGlobalAnswers, error) {
	steps := []components.Step{
		{
			Name: "Shell",
			Model: components.NewSelectList(components.NewSelectListParams{
				Title:       "Shell",
				Description: "wtm provides a shell function (via `wtm shell-init`) to jump between worktrees. Pick your primary shell so the integration matches.",
				Items: []components.SelectItem{
					{Label: "zsh", Value: string(domain.ShellZsh)},
					{Label: "bash", Value: string(domain.ShellBash)},
					{Label: "fish", Value: string(domain.ShellFish)},
				},
			}),
			Summary: func(model any) string {
				sl, ok := model.(components.SelectListModel)
				if !ok {
					return ""
				}
				return sl.Value()
			},
			Callout: true,
		},
	}

	wiz := components.NewWizard(steps)
	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return domain.InitGlobalAnswers{}, fmt.Errorf("global wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok {
		return domain.InitGlobalAnswers{}, domain.ErrUserAborted
	}
	if final.Aborted() {
		return domain.InitGlobalAnswers{}, domain.ErrUserAborted
	}

	shellStep, ok := final.Steps()[0].Model.(components.SelectListModel)
	if !ok {
		return domain.InitGlobalAnswers{}, domain.ErrUserAborted
	}

	return domain.InitGlobalAnswers{
		Shell: domain.ShellType(shellStep.Value()),
	}, nil
}
