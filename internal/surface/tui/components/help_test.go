package components

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// The bar is composed in one place so the same gesture is named the same way on
// every screen. These read the composition, not the strings.
func TestEveryStepBarNavigatesConfirmsAndOffersAWayOut(t *testing.T) {
	for _, step := range everyStepModel() {
		t.Run(step.Name, func(t *testing.T) {
			m := NewWizard([]Step{step})

			bar := m.helpLine()
			_, rowless := step.Model.(rowless)
			if !rowless && !strings.Contains(bar, domain.HelpNavigate) {
				t.Errorf("bar = %q, want it to say how to move", bar)
			}
			if !strings.Contains(bar, domain.HelpConfirm) && !strings.Contains(bar, domain.HelpSelect) {
				t.Errorf("bar = %q, want it to name the enter gesture", bar)
			}
			if !strings.Contains(bar, domain.HelpCancel) && !strings.Contains(bar, domain.HelpBack) {
				t.Errorf("bar = %q, want it to name the way out", bar)
			}
		})
	}
}

func TestTheEnterWordFollowsWhetherTheStepHasADoneRow(t *testing.T) {
	for _, step := range everyStepModel() {
		t.Run(step.Name, func(t *testing.T) {
			m := NewWizard([]Step{step})
			_, onDoneRow := step.Model.(doneRower)

			bar := m.helpLine()
			if onDoneRow && !strings.Contains(bar, domain.HelpSelect) {
				t.Errorf("bar = %q: enter acts on the row here, the last one confirms", bar)
			}
			if !onDoneRow && !strings.Contains(bar, domain.HelpConfirm) {
				t.Errorf("bar = %q: enter ends the step here", bar)
			}
		})
	}
}

func TestTheWayOutSaysBackOnceThereIsSomewhereToGoBackTo(t *testing.T) {
	first := Step{Name: "first", Build: func([]Step) any { return NewConfirm(NewConfirmParams{Title: "one"}) }}
	second := Step{Name: "second", Build: func([]Step) any { return NewConfirm(NewConfirmParams{Title: "two"}) }}

	m := NewWizard([]Step{first, second})
	if got := m.helpLine(); !strings.Contains(got, domain.HelpCancel) {
		t.Fatalf("on the first step there is nothing behind: %q", got)
	}

	m.current = 1
	if got := m.helpLine(); !strings.Contains(got, domain.HelpBack) {
		t.Fatalf("on a later step esc steps back: %q", got)
	}
}

func TestEveryDistinctiveGestureStaysOnItsBar(t *testing.T) {
	cases := map[string]string{
		"reorder": domain.HelpReorder,
		"ports":   domain.HelpBindsNoPort,
		"routes":  domain.HelpSwitchRoute,
		"runners": domain.HelpSetRunner,
		"kinds":   domain.HelpSetKind,
		"hooks":   domain.HelpDelete,
	}

	for _, step := range everyStepModel() {
		want, named := cases[step.Name]
		if !named {
			continue
		}
		t.Run(step.Name, func(t *testing.T) {
			m := NewWizard([]Step{step})
			if got := m.helpLine(); !strings.Contains(got, want) {
				t.Fatalf("bar = %q, want it to name %q — a gesture only this step has", got, want)
			}
		})
	}
}

func TestAStepWithNoRowsDoesNotOfferToMoveBetweenThem(t *testing.T) {
	m := NewWizard([]Step{{Name: "text", Model: NewTextInput(NewTextInputParams{Title: "t", Description: "d"})}})

	if got := m.helpLine(); strings.Contains(got, domain.HelpNavigate) {
		t.Fatalf("bar = %q: a text input has nothing to navigate", got)
	}
}
