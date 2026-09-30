package runview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

// previewHarness drives the hosted form of the view the way a panel does: it
// gives it a size, hands it a job list, and never sends it a key.
func previewHarness(t *testing.T, job string, views []runlogs.JobView, lines map[string][]string) *testHarness {
	t.Helper()
	board := runlogstest.NewBoard(runlogstest.BoardParams{Views: views, Lines: lines})
	model := NewPreview(PreviewParams{Board: board, Job: job})
	t.Cleanup(model.Close)

	sized, _ := model.SetSize(60, 12)
	h := &testHarness{model: sized, board: board}
	h.load(t, views)
	return h
}

// The panel shows the run view itself: the same pane, the same emulator, the
// same history — at a panel's size, with none of the full view's chrome.
func TestPreviewRendersThePaneAndNothingElse(t *testing.T) {
	h := previewHarness(t, "migrate",
		[]runlogs.JobView{stopped("migrate"), stopped("seed")},
		map[string][]string{"migrate": {"applied 3 migrations"}})
	h.waitForPane(t, h.model.selected, "applied 3 migrations")

	frame := ansi.Strip(h.model.View())
	if !strings.Contains(frame, "applied 3 migrations") {
		t.Errorf("frame does not hold the job's output:\n%s", frame)
	}
	if !strings.Contains(frame, "migrate") {
		t.Errorf("frame does not name the job:\n%s", frame)
	}
	for _, chrome := range []string{domain.RunViewJobsTitle, domain.RunViewHelpBrowse} {
		if strings.Contains(frame, chrome) {
			t.Errorf("frame carries the full view's %q, which its host draws itself:\n%s", chrome, frame)
		}
	}
	if got := len(strings.Split(frame, "\n")); got != 12 {
		t.Errorf("frame is %d rows, want the 12 its host gave it", got)
	}
}

// A host's cursor reaches the preview through ShowJob: it reads no key of its
// own, so this is the only way its pane ever changes.
func TestPreviewFollowsItsHostsCursor(t *testing.T) {
	h := previewHarness(t, "migrate",
		[]runlogs.JobView{stopped("migrate"), stopped("seed")},
		map[string][]string{"migrate": {"applied 3 migrations"}, "seed": {"seeded 12 rows"}})

	model, cmd := h.model.ShowJob("seed")
	h.model = h.follow(t, model, cmd)
	h.waitForPane(t, h.model.selected, "seeded 12 rows")

	if !strings.Contains(ansi.Strip(h.model.View()), "seeded 12 rows") {
		t.Errorf("the preview did not follow its host onto seed:\n%s", ansi.Strip(h.model.View()))
	}
}

// Keys belong to the host. A preview that answered them would fight the panel
// holding it for the same arrows.
func TestPreviewIgnoresKeys(t *testing.T) {
	h := previewHarness(t, "migrate",
		[]runlogs.JobView{stopped("migrate"), stopped("seed")}, nil)
	before := h.model.selected

	next, _ := h.model.Update(namedKey(tea.KeyDown))
	model, _ := next.(Model)

	if model.selected != before {
		t.Errorf("selection moved to %q on a key the host owns", model.selected)
	}
}
