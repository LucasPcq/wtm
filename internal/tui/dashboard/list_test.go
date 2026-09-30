package dashboard

import (
	"github.com/charmbracelet/bubbles/spinner"

	"fmt"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/flow"

	"github.com/LucasPcq/wtm/internal/domain"
)

// lockOperation puts a background create in flight, lets it name its target the
// way the prompter does once the wizard is answered, and posts the stage a
// running flow's Presenter would send from Stage/HookPhase.
func lockOperation(t *testing.T, model Model, branch, stage string) Model {
	t.Helper()
	model, cmd := updateCmd(model, key(domain.KeyNew))
	if cmd == nil {
		t.Fatal("the create run did not start")
	}
	id := model.ops.running[0].id
	model, _ = model.applyFlow(opTargetMsg{id: id, targets: []string{branch}})
	model, _ = model.applyFlow(opStageMsg{id: id, stage: stage})
	if _, held := model.ops.holding(branch); !held {
		t.Fatalf("the run does not hold %q", branch)
	}
	return model
}

// TestActiveRowIsMarked pins that the active worktree is named in the
// row's meta line, in the same tag vocabulary as "from main" or "PR #67" —
// not a bare glyph glued to the branch name, which communicated nothing on
// its own and appeared nowhere else in the row.
func TestActiveRowIsMarked(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model.activeBranch = "feat/a"

	// listBody isolates the list panel from the header, which carries its own
	// "● <branch>" segment (untouched here) — a raw model.View() line would
	// find that one first and prove nothing about the row itself. Each entry
	// still embeds its own two physical lines joined by "\n", so split again
	// before searching one line at a time.
	lines := strings.Split(strings.Join(model.listBody(model.layout()), "\n"), "\n")
	nameRow := lineContaining(t, lines, "feat/a")
	if strings.Contains(nameRow, domain.DashboardActiveGlyph) {
		t.Error("the branch name must not carry a bare glyph — the marker moved to the meta line as a text tag")
	}

	metaRow := lineContaining(t, lines, domain.DetailYouAreHere)
	if !strings.Contains(metaRow, domain.DetailYouAreHere) {
		t.Error("la ligne meta du worktree actif doit porter le tag \"you are here\"")
	}
}

func TestLockedRowShowsProgressNotState(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model = lockOperation(t, model, "feat/a", "running on_create hook…")

	// Read the list panel's own body, not the full joined view: at this width
	// the detail panel sits on the same terminal rows, and "main" (still the
	// cursor) legitimately shows "clean" there — nothing to do with feat/a's row.
	body := strings.Join(model.listBody(model.layout()), "\n")
	nameLine := lineContaining(t, strings.Split(body, "\n"), "feat/a")
	if strings.Contains(nameLine, "clean") || strings.Contains(nameLine, "dirty") {
		t.Error("la pastille d'état cède sa place au spinner pendant l'opération")
	}
	if !strings.Contains(body, "running on_create hook…") {
		t.Error("une ligne verrouillée montre l'étape en cours")
	}
}

// TestEmptySelectionAndOutputNameTheNextAction covers the two empty states
// that are genuinely reachable and persistent: no worktree selected, and no
// operation has produced output yet. DashboardEmptyList is deliberately not
// asserted here — the list can only be empty when the listing itself did not
// go as expected (a valid repository always has its main worktree), so it
// keeps neutral wording rather than naming an action.
func TestEmptySelectionAndOutputNameTheNextAction(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	model.outputExpanded = true

	if !strings.Contains(strings.Join(model.detailBody(model.layout()), "\n"), domain.DashboardEmptySelection) {
		t.Error("aucune sélection : le panneau détail doit nommer l'action suivante")
	}
	if !strings.Contains(strings.Join(model.outputBody(model.layout(), testWidth), "\n"), domain.DashboardEmptyOutput) {
		t.Error("aucune sortie : le panneau output doit nommer l'action suivante")
	}
}

// TestEmptyListStaysNeutral pins the deliberate exception: an empty listing
// only happens when the listing itself did not go as expected (a valid
// repository always has its main worktree), so the placeholder must not
// offer confident advice ("press n…") about a situation it cannot explain.
func TestEmptyListStaysNeutral(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)

	body := strings.Join(model.listBody(model.layout()), "\n")
	if !strings.Contains(body, domain.DashboardEmptyList) {
		t.Fatal("l'état vide de la liste est absent")
	}
	if strings.Contains(body, "press n") {
		t.Error("une liste vide ne doit pas conseiller une action : on ignore pourquoi elle est vide")
	}
}

// A locked row draws the spinner instead of its state pill, so the spinner must
// keep ticking for the whole run — a frozen glyph reads as hung, which is the
// opposite of what the marker is for.
func TestSpinnerKeepsTickingWhileAnOperationRuns(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model.detailLoading = ""

	if _, cmd := updateCmd(model, spinner.TickMsg{}); cmd != nil {
		t.Fatal("idle: nothing loading and nothing running must stop the loop")
	}

	model, _ = model.beginOp(beginParams{Operation: flow.Operation{Kind: "create", Mode: flow.ModeBackground}, Target: "feat/a"})
	if _, cmd := updateCmd(model, spinner.TickMsg{}); cmd == nil {
		t.Error("a tick while a run is in flight must re-arm, or the locked row's spinner freezes")
	}
}

// The dashboard's whole promise here is telling you what is running where, so
// the count belongs on the row rather than one keystroke away.
func TestRowNamesTheJobsAWorktreeIsRunning(t *testing.T) {
	m := Model{running: map[string]int{"/wt/feature": 2}}
	status := domain.WorktreeStatus{Branch: "feature", Path: "/wt/feature"}

	if got := m.runningBadge(status); !strings.Contains(got, "2 running") {
		t.Errorf("badge = %q, want it to name the running jobs", got)
	}
}

func TestARowSaysNothingAboutAnotherWorktreesJobs(t *testing.T) {
	m := Model{running: map[string]int{"/wt/other": 2}}
	status := domain.WorktreeStatus{Branch: "feature", Path: "/wt/feature"}

	if got := m.runningBadge(status); got != "" {
		t.Errorf("badge = %q, want no mention of another worktree's jobs", got)
	}
}

// The base row has no parent, so a badge taking its rank among the tags landed
// first there and second everywhere else. Its place must not depend on what the
// meta happens to hold.
func TestTheRunningBadgeSitsAtTheSamePlaceOnEveryRow(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat")
	model.statuses[0].IsParent = true
	model.parents = map[string]string{"feat": "main"}
	model.running = map[string]int{"/tmp/main": 1, "/tmp/feat": 2}

	width := model.layout().List.Width - borderWidth - paddingWidth
	base := stripANSI(model.renderRow(0, width)[1])
	child := stripANSI(model.renderRow(1, width)[1])

	if want := fmt.Sprintf(domain.TreeBadgeRunningFmt, 1); !strings.HasSuffix(strings.TrimRight(base, " "), want) {
		t.Errorf("base meta = %q, want %q flush right", base, want)
	}
	if want := fmt.Sprintf(domain.TreeBadgeRunningFmt, 2); !strings.HasSuffix(strings.TrimRight(child, " "), want) {
		t.Errorf("child meta = %q, want %q flush right", child, want)
	}
	if !strings.Contains(child, domain.DashboardMetaFromPrefix+"main") {
		t.Errorf("child meta = %q, want the badge to leave the tags in place", child)
	}
}

func TestARowWithNothingUpCarriesNoBadge(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "feat")

	width := model.layout().List.Width - borderWidth - paddingWidth
	meta := stripANSI(model.renderRow(0, width)[1])

	if strings.Contains(meta, "running") {
		t.Errorf("meta = %q, want no badge at all when nothing is up", meta)
	}
}
