package components

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

// manyParentSteps builds a relocate-like wizard: one SelectList parent picker per
// worktree, each over a long candidate list. Mirrors internal/tui/relocate.
func manyParentSteps(stepCount, branchCount int) []Step {
	items := make([]SelectItem, branchCount)
	for i := range items {
		items[i] = SelectItem{Label: fmt.Sprintf("branch-%d", i), Value: fmt.Sprintf("branch-%d", i)}
	}
	steps := make([]Step, stepCount)
	for i := range steps {
		steps[i] = Step{
			Name: fmt.Sprintf("Parent for wt-%d", i),
			Model: NewSelectList(NewSelectListParams{
				Title:       fmt.Sprintf("Parent for wt-%d", i),
				Description: "Recorded as the worktree's parent so `wtm sync` can rebase it.",
				Items:       items,
			}),
			Summary: SelectSummary,
		}
	}
	return steps
}

// renderHeight is the number of terminal rows the wizard render occupies.
func renderHeight(view string) int {
	if view == "" {
		return 0
	}
	return strings.Count(view, "\n") + 1
}

// TestWizardRenderFitsTerminalHeight is the LUC-85 regression: deep into a long
// relocate run, the render must stay within the terminal height (otherwise the
// inline renderer scrolls the breadcrumb off-screen) and the breadcrumb naming
// the current worktree must remain visible.
func TestWizardRenderFitsTerminalHeight(t *testing.T) {
	const termHeight = 24
	m := NewWizard(manyParentSteps(12, 40))
	m.current = 10 // simulate having already chosen parents for 10 worktrees
	m = updateWizard(m, tea.WindowSizeMsg{Width: 100, Height: termHeight})

	view := m.View()

	if h := renderHeight(view); h > termHeight {
		t.Fatalf("render height = %d, want <= %d (breadcrumb would scroll off-screen)", h, termHeight)
	}
	if !strings.Contains(view, "Parent for wt-10") {
		t.Errorf("breadcrumb for the current worktree (wt-10) is not visible in the render")
	}
}

// TestWizardCollapsesOldSummaries verifies that with far more completed steps
// than fit, older summaries collapse into a single "… (N earlier steps)" line so
// the block stays bounded.
func TestWizardCollapsesOldSummaries(t *testing.T) {
	const termHeight = 24
	m := NewWizard(manyParentSteps(40, 10))
	m.current = 38
	m = updateWizard(m, tea.WindowSizeMsg{Width: 100, Height: termHeight})

	view := m.View()

	if renderHeight(view) > termHeight {
		t.Fatalf("render height = %d, want <= %d", renderHeight(view), termHeight)
	}
	if !strings.Contains(view, "earlier steps") {
		t.Error("expected a collapsed summary line with many completed steps")
	}
	if !strings.Contains(view, "Parent for wt-38") {
		t.Error("current worktree breadcrumb must stay visible even with the summaries collapsed")
	}
}

// TestWizardListKeepsMinHeight checks that on a normal terminal the summaries are
// bounded so the list keeps at least the reserved minimum height (instead of the
// old fixed -10 over-allocation that pushed the breadcrumb off-screen).
func TestWizardListKeepsMinHeight(t *testing.T) {
	m := NewWizard(manyParentSteps(12, 30))
	m.current = 10
	m = updateWizard(m, tea.WindowSizeMsg{Width: 100, Height: 24})

	sl, ok := m.steps[m.current].Model.(SelectListModel)
	if !ok {
		t.Fatalf("current step is not a SelectListModel")
	}
	if sl.height < domain.MinWizardListHeight {
		t.Errorf("list height = %d, want >= %d", sl.height, domain.MinWizardListHeight)
	}
}

// A description longer than the pane is wrapped, not cut off at its edge.
func TestADescriptionWrapsAtThePaneWidth(t *testing.T) {
	const width = 40
	long := "Fast-forward this branch to origin before creating the worktree so it starts from the commits the remote already has."
	wizard := NewWizardWithParams(WizardParams{Steps: []Step{{
		Name:  "Source update",
		Model: NewSelectList(NewSelectListParams{Description: long, Items: []SelectItem{{Label: "keep", Value: "keep"}}}),
	}}})
	updated, _ := wizard.Update(tea.WindowSizeMsg{Width: width, Height: 30})
	view := updated.(WizardModel).View()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "navigate") {
			continue
		}
		if w := PrintableWidth(line); w > width {
			t.Errorf("line is %d columns wide on a %d-column pane: %q", w, width, line)
		}
	}
	if !strings.Contains(view, "remote") {
		t.Errorf("the end of the description was lost:\n%s", view)
	}
}

func TestAValidationErrorWrapsAtThePaneWidth(t *testing.T) {
	const width = 40
	banner := wrappedErrorBanner("worktree already exists: dst is checked out at /a/very/long/path/to/the/trees/dst", width)
	for _, line := range strings.Split(banner, "\n") {
		if w := PrintableWidth(line); w > width {
			t.Errorf("line is %d columns wide: %q", w, line)
		}
	}
	if strings.Count(banner, "\n") == 0 {
		t.Error("a long error did not wrap")
	}
}
