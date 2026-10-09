package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func kindList() KindListModel {
	return NewKindList(NewKindListParams{
		Title: "t",
		Entries: []domain.JobKindChoice{
			{Label: "root / build", Cmd: "turbo run build", Name: "build", Kind: domain.JobKindTask},
			{Label: "apps/web / preview", Cmd: "vite preview", Name: "preview", Kind: domain.JobKindService},
		},
	})
}

func updateKind(m KindListModel, k tea.KeyMsg) KindListModel {
	updated, _ := m.Update(k)
	return updated
}

func TestKindListShowsBothKindsWithTheCurrentOneFilled(t *testing.T) {
	view := kindList().View()

	if !strings.Contains(view, "● task") || !strings.Contains(view, "○ service") {
		t.Errorf("the task row must show both kinds, the current one filled:\n%s", view)
	}
	if !strings.Contains(view, "○ task") || !strings.Contains(view, "● service") {
		t.Errorf("the service row must show both kinds, the current one filled:\n%s", view)
	}
}

func TestKindListArrowsSetTheKindRatherThanToggle(t *testing.T) {
	m := kindList()

	// Right names service, left names task — pressing the same arrow twice must
	// not walk back, which is the whole difference with a checkbox.
	m = updateKind(m, key(tea.KeyRight))
	m = updateKind(m, key(tea.KeyRight))
	if got := m.Entries()[0].Kind; got != domain.JobKindService {
		t.Errorf("kind = %q after two rights, want service", got)
	}

	m = updateKind(m, key(tea.KeyLeft))
	m = updateKind(m, key(tea.KeyLeft))
	if got := m.Entries()[0].Kind; got != domain.JobKindTask {
		t.Errorf("kind = %q after two lefts, want task", got)
	}
}

func TestKindListSpaceSwitchesToTheOtherKind(t *testing.T) {
	m := updateKind(kindList(), key(tea.KeySpace))

	if got := m.Entries()[0].Kind; got != domain.JobKindService {
		t.Errorf("kind = %q, want space to switch a task to a service", got)
	}
}

func TestKindListEditsOnlyTheRowUnderTheCursor(t *testing.T) {
	m := updateKind(kindList(), key(tea.KeyRight))

	if got := m.Entries()[1].Kind; got != domain.JobKindService {
		t.Errorf("the second row changed to %q; it was not under the cursor", got)
	}
}

func TestKindListEnterConfirms(t *testing.T) {
	m := updateKind(kindList(), key(tea.KeyEnter))

	if !m.Done() {
		t.Error("enter must confirm the step")
	}
}

func TestKindListEscapeAborts(t *testing.T) {
	m := updateKind(kindList(), key(tea.KeyEsc))

	if !m.Aborted() {
		t.Error("escape must go back")
	}
}

func TestKindListNamesEveryJobItAsksAbout(t *testing.T) {
	// Two scripts can share a name across packages; the row must say which is
	// which, and each must be settled on its own.
	view := kindList().View()
	for _, want := range []string{"root / build", "apps/web / preview", "turbo run build"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
}
