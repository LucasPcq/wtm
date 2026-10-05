package runview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

const driftLine = "main still addresses ports in its .env — `wtm env main` aligns it"

func viewWithWarnings(t *testing.T, warnings []string) string {
	t.Helper()
	board := runlogstest.NewBoard(runlogstest.BoardParams{Views: []runlogs.JobView{
		inWorktree(running("web"), "/work/main", "main"),
	}})
	model := New(t.Context(), Params{Board: board, Warnings: warnings})
	t.Cleanup(func() { model.panes.closeAll() })
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	return update(model, exec(t, model.refreshCmd())).View()
}

// `run logs` starts nothing, so every other band stays empty — and it is the
// surface where a reader would otherwise only learn this on the way out.
func TestTheBandCarriesTheAddressingWarningWithNoRun(t *testing.T) {
	view := viewWithWarnings(t, []string{driftLine})

	if !strings.Contains(view, domain.AddressingDriftTitle) {
		t.Fatalf("the notice band shows no addressing title:\n%s", view)
	}
	if !strings.Contains(view, "wtm env main") {
		t.Errorf("the band names no command to run:\n%s", view)
	}
}

func TestTheBandStaysEmptyWithoutAWarning(t *testing.T) {
	if view := viewWithWarnings(t, nil); strings.Contains(view, domain.AddressingDriftTitle) {
		t.Errorf("an aligned run drew a band anyway:\n%s", view)
	}
}

// The dismiss key clears it like every other report the band holds.
func TestTheAddressingBandIsDismissable(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{Views: []runlogs.JobView{
		inWorktree(running("web"), "/work/main", "main"),
	}})
	model := New(t.Context(), Params{Board: board, Warnings: []string{driftLine}})
	t.Cleanup(func() { model.panes.closeAll() })
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, exec(t, model.refreshCmd()))

	model.dismissed = true

	if strings.Contains(model.View(), domain.AddressingDriftTitle) {
		t.Errorf("the band survived a dismissal:\n%s", model.View())
	}
}
