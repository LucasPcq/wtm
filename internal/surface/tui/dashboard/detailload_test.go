package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestSelectionSchedulesDetailLoad(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a", "feat/b")

	model, cmd := updateCmd(model, key("j"))
	if cmd == nil {
		t.Fatal("changer de sélection doit programmer un chargement de détail")
	}
	if model.detailLoading != "feat/a" {
		t.Errorf("detailLoading = %q, want feat/a", model.detailLoading)
	}
}

func TestCachedDetailSurvivesReload(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model = update(model, detailMsg{branch: "main", detail: domain.WorktreeDetail{
		Branch:  "main",
		Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: seed"}},
	}})

	model = update(model, pollMsg{})

	got, ok := model.details["main"]
	if !ok {
		t.Fatal("le poll ne doit pas vider le cache : le panneau afficherait du vide")
	}
	if len(got.Commits) != 1 {
		t.Errorf("Commits = %d, want 1", len(got.Commits))
	}
}

// TestParentPathIgnoresDetachedWorktree pins the statusFor("") guard: a
// detached worktree can carry an empty Branch through service/worktree.List,
// and a branch with no recorded parent must not resolve to that worktree's
// path. Getting this wrong feeds the wrong ParentPath into ComputeEnvDiff,
// silently — wrong .env drift counters, with no error anywhere.
func TestParentPathIgnoresDetachedWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "feat/a")
	model.statuses = append(model.statuses, domain.WorktreeStatus{Branch: "", Path: "/tmp/detached"})
	model.parents = map[string]string{"feat/a": ""}

	params := model.detailParams("feat/a")
	if params.ParentPath != "" {
		t.Errorf("ParentPath = %q, want empty: a parentless branch must not borrow a detached worktree's path", params.ParentPath)
	}
}

// TestChildrenAreSorted pins that childrenOf never returns map iteration
// order: the selected branch's detail reloads regularly, and an unsorted
// LINKS "Children" line would permute between refreshes — content moving
// under state 2 (§8), which must hold pixel-still.
func TestChildrenAreSorted(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model.parents = map[string]string{
		"feat/z": "main",
		"feat/a": "main",
		"feat/m": "main",
	}

	got := model.childrenOf("main")
	want := []string{"feat/a", "feat/m", "feat/z"}
	if len(got) != len(want) {
		t.Fatalf("childrenOf = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("childrenOf = %v, want %v", got, want)
			break
		}
	}
}

func TestStaleDetailIsMarkedNotErrored(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model = update(model, detailMsg{branch: "main", detail: domain.WorktreeDetail{Branch: "main"}})
	model.detailLoading = "main"
	model.detailSince = time.Now().Add(-time.Second)

	view := model.View()
	if !strings.Contains(view, domain.DashboardRefreshing) {
		t.Error("un détail en cours de rechargement doit porter son marqueur")
	}
}

func TestFreshDetailShowsNoMarker(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model = update(model, detailMsg{branch: "main", detail: domain.WorktreeDetail{Branch: "main"}})

	// Asserted directly, not just through the view: a detailLoading that
	// applyDetail forgot to clear would still pass the view-only check below
	// for up to DashboardSpinnerGrace, making this test vacuous.
	if model.detailLoading != "" {
		t.Fatalf("detailLoading = %q, want empty: a landed load must clear it", model.detailLoading)
	}
	if strings.Contains(model.View(), domain.DashboardRefreshing) {
		t.Error("l'état frais ne se signale pas")
	}
}

// TestSupersededTickIsDropped pins the one line that makes scheduleDetail a
// debounce rather than a plain delay: a tick landing for a branch that is no
// longer selected must start no load at all.
func TestSupersededTickIsDropped(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a", "feat/b")

	model, cmd := updateCmd(model, key("j"))
	if cmd == nil {
		t.Fatal("changer de sélection doit programmer un chargement de détail")
	}
	model = update(model, key("j"))
	if model.detailLoading != "feat/b" {
		t.Fatalf("detailLoading = %q, want feat/b", model.detailLoading)
	}

	_, tickCmd := updateCmd(model, detailTickMsg{branch: "feat/a"})
	if tickCmd != nil {
		t.Error("un tick pour une branche qui n'est plus sélectionnée ne doit lancer aucun chargement")
	}
}

// TestPollNeverReloadsTheDetail pins the panel out of the poll entirely: a
// timer-driven reload mutes the whole panel behind a "refreshing" marker every
// few seconds while the user is reading it. The detail reloads on a selection
// change, on an operation touching its branch, and on KeyRefresh — not on time.
func TestPollNeverReloadsTheDetail(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	if !model.layout().DetailVisible {
		t.Fatal("setup: the detail panel must be on screen for this to prove anything")
	}
	model = update(model, detailMsg{branch: "main", detail: domain.WorktreeDetail{Branch: "main"}})

	model = update(model, pollMsg{})
	if model.detailLoading != "" {
		t.Errorf("detailLoading = %q, want empty: the poll must never reload the detail panel", model.detailLoading)
	}
	if model.detailIsStale() {
		t.Error("le poll ne doit jamais griser le panneau détail ni afficher son marqueur de rafraîchissement")
	}
}

// TestRefreshKeyReloadsTheDetail is the other half of the rule above: taking
// the panel out of the poll only holds if `r` still brings it back.
func TestRefreshKeyReloadsTheDetail(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model = update(model, detailMsg{branch: "main", detail: domain.WorktreeDetail{Branch: "main"}})

	model, cmd := updateCmd(model, key(keyRefresh))
	if cmd == nil {
		t.Fatal("la touche refresh doit programmer un rechargement")
	}
	if model.detailLoading != "main" {
		t.Errorf("detailLoading = %q, want main: refresh must reload the detail panel", model.detailLoading)
	}
}

// TestDebounceIsNotCountedAgainstTheGraceDelay pins the field semantics that
// fully determine when the grace delay starts, rather than timing behaviour:
// a real-elapsed-time assertion cannot tell "grace measured from load start"
// apart from "grace measured from schedule time", since 150 ms of debounce
// plus a fast git log both land well under 200 ms in a test run either way.
// Each assertion below is chosen to fail against the bug it pins: the first
// against detailSince set at schedule time, the second against a missing
// IsZero guard in detailIsStale.
func TestDebounceIsNotCountedAgainstTheGraceDelay(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")

	model, cmd := updateCmd(model, key("j"))
	if cmd == nil {
		t.Fatal("changer de sélection doit programmer un chargement de détail")
	}
	// detailSince stays the zero value while only scheduled — a structural
	// guarantee, not a timing one: IsZero is checked by detailIsStale, so it
	// stays false regardless of how much real time the debounce or the test
	// itself takes, without needing to fake elapsed time to prove it.
	if !model.detailSince.IsZero() {
		t.Fatal("detailSince must stay zero while only scheduled: it must not be set at debounce time")
	}
	if model.detailIsStale() {
		t.Fatal("detailIsStale must be false while only scheduled, no matter how much time has notionally passed")
	}

	model = update(model, detailTickMsg{branch: "feat/a"})
	if model.detailSince.IsZero() {
		t.Fatal("detailSince must be set once the debounce lands and the load actually starts")
	}

	model.detailSince = time.Now().Add(-domain.DashboardSpinnerGrace - time.Second)
	if !model.detailIsStale() {
		t.Error("detailIsStale must be true once the load has run longer than the grace delay")
	}
	model.detailSince = time.Now()
	if model.detailIsStale() {
		t.Error("detailIsStale must be false the instant the load starts")
	}
}

// TestSpinnerTicksOnlyWhileLoading pins the spinner lifecycle end to end: the
// loop must terminate once nothing is loading (or it ticks at 12fps for the
// life of the program, idle included), and it must still be running while a
// load actually is in flight (or the marker freezes the moment it appears —
// this second assertion is what stops a future "fix" from silencing the tick
// loop by never starting it).
func TestSpinnerTicksOnlyWhileLoading(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	// newTestModel's own worktreesMsg already schedules a load for the first
	// selection: reset to the genuinely idle state this assertion is about.
	model.detailLoading = ""

	_, idleCmd := updateCmd(model, spinner.TickMsg{})
	if idleCmd != nil {
		t.Error("a spinner tick while nothing is loading must not re-arm: the loop must terminate")
	}

	model.detailLoading = "main"
	_, loadingCmd := updateCmd(model, spinner.TickMsg{})
	if loadingCmd == nil {
		t.Error("a spinner tick while a load is in flight must re-arm, or the marker freezes")
	}
}

func TestMarkerWaitsForTheGraceDelay(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model.detailLoading = "main"
	model.detailSince = time.Now()

	if strings.Contains(model.View(), domain.DashboardRefreshing) {
		t.Error("sous le délai d'apparition, aucun marqueur : ce serait du flash déguisé en feedback")
	}
}

// A removal changes rows it never targeted: the parent on screen loses a
// child, which only the reloaded list says, so that is when its detail reloads.
func TestAListThatChangesTheChildrenOnScreenReloadsTheDetail(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a", "feat/b")
	model.parents = map[string]string{"feat/a": "main", "feat/b": "main"}
	model.detailLoading = ""

	model = update(model, worktreesMsg{statuses: model.statuses[:2], parents: map[string]string{"feat/a": "main"}})

	if model.detailLoading != "main" {
		t.Errorf("detailLoading = %q, want main's detail reloaded once its children changed", model.detailLoading)
	}
}

func TestAListThatChangesNothingOnScreenKeepsTheDetail(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model.parents = map[string]string{"feat/a": "main"}
	model.detailLoading = ""

	model = update(model, worktreesMsg{statuses: model.statuses, parents: map[string]string{"feat/a": "main"}})

	if model.detailLoading != "" {
		t.Errorf("detailLoading = %q, want no reload: nothing the panel shows changed", model.detailLoading)
	}
}
