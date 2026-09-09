package dashboard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

func rightClick(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonRight}
}

func TestRightClickSelectsTheRowAndOpensItsMenu(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(2))

	model = update(model, rightClick(rowTextX+2, rowY(2)))

	if model.cursor != 2 {
		t.Fatalf("cursor = %d, want the row the menu was opened on", model.cursor)
	}
	if !model.menuOpen {
		t.Fatal("a right click on a row opens its context menu")
	}
}

func TestRightClickOutsideTheListOpensNothing(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	renderAndWait(t, model, rowZone(0))

	model = update(model, rightClick(detailX, firstRowY))

	if model.menuOpen {
		t.Error("the menu belongs to a worktree row; nowhere else opens it")
	}
}

// The keyboard path is not a convenience: terminals that turn the right button
// into a paste never deliver it, and the dashboard has to stay fully usable.
func TestTheMenuKeyOpensTheSameMenuAsTheRightClick(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	model = update(model, key("j"))

	model = update(model, key(domain.KeyMenu))

	if !model.menuOpen {
		t.Fatal("m must open the context menu on the selected row")
	}
	if model.cursor != 1 {
		t.Errorf("cursor = %d, want the selection left where it was", model.cursor)
	}
	if len(model.menuItems()) == 0 {
		t.Fatal("the menu must offer something")
	}

	if model = update(model, key(domain.KeyMenu)); model.menuOpen {
		t.Error("m must close the menu it opened")
	}
}

func TestTheMenuNeedsAWorktreeToActOn(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)

	model = update(model, key(domain.KeyMenu))

	if model.menuOpen {
		t.Error("an empty list has no row to open a menu on")
	}
}

func TestEscClosesTheMenuAndOtherKeysFallThrough(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, key(domain.KeyMenu))

	model = update(model, namedKey(tea.KeyEsc))
	if model.menuOpen {
		t.Fatal("esc closes the menu")
	}

	model = update(model, key(domain.KeyMenu))
	model = update(model, key("G"))

	if model.menuOpen {
		t.Fatal("a key the menu does not use closes it rather than trapping the keyboard")
	}
	if model.cursor != 1 {
		t.Errorf("cursor = %d, want the key to have reached the list", model.cursor)
	}
}

// menuEntryPoint is where entry i is drawn, derived from the placement rule and
// the box's own composition: the border, the padding row, then the worktree's
// name and the rule that separates it from the actions.
func menuEntryPoint(t *testing.T, model Model, index int) (x, y int) {
	t.Helper()
	box, rect := model.menuBox()
	if box == "" {
		t.Fatal("the menu has nothing to draw")
	}
	return rect.X + menuBorder + menuPadding, rect.Y + menuBorder + menuHeaderRows + index
}

const (
	menuBorder  = 1
	menuPadding = 1
	// menuHeaderRows is the worktree's name and the rule under it.
	menuHeaderRows = 2
)

func TestTheMenuFloatsUnderTheCellItWasOpenedFrom(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	model = update(model, key("j"))
	model = update(model, key(domain.KeyMenu))
	renderAndWait(t, model, menuZone(0))

	_, rect := model.menuBox()
	if rect.Y != model.menuAnchor.Y+1 {
		t.Errorf("the menu sits at y=%d, want it hanging just under its row (%d)", rect.Y, model.menuAnchor.Y+1)
	}
	if want := rowY(1) + domain.DashboardRowHeight - 1; model.menuAnchor.Y != want {
		t.Errorf("the keyboard anchored the menu at y=%d, want the last line of the selected row %d",
			model.menuAnchor.Y, want)
	}

	entry := model.zones.Get(menuZone(0))
	wantX, wantY := menuEntryPoint(t, model, 0)
	if entry.StartY != wantY || entry.StartX != wantX {
		t.Errorf("entry 0 starts at (%d,%d), want (%d,%d) — inside the box the rule placed",
			entry.StartX, entry.StartY, wantX, wantY)
	}
}

func TestTheMenuFlipsAboveTheAnchorRatherThanRunningOffTheBottom(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, rightClick(rowTextX, testHeight-1))

	box, rect := model.menuBox()
	if rect.Y+lipgloss.Height(box) > testHeight {
		t.Errorf("the menu runs to y=%d, past the last row %d", rect.Y+lipgloss.Height(box), testHeight)
	}
	if rect.Y >= testHeight-1 {
		t.Errorf("the menu sits at y=%d, want it above an anchor on the last row", rect.Y)
	}
}

// menuIndexOf locates an entry by what it does, so a test does not break when
// the menu gains a neighbour above it.
func menuIndexOf(t *testing.T, model Model, action menuAction) int {
	t.Helper()
	for index, item := range model.menuItems() {
		if item.action == action {
			return index
		}
	}
	t.Fatalf("no menu entry for action %v", action)
	return -1
}

func TestClickingAnEntryStartsTheRemoval(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	model = update(model, key("j"))
	model = update(model, key(domain.KeyMenu))
	index := menuIndexOf(t, model, menuDelete)
	renderAndWait(t, model, menuZone(index))
	x, y := menuEntryPoint(t, model, index)

	clicked, cmd := updateCmd(model, click(x, y))

	if cmd == nil {
		t.Fatal("clicking Delete must start the removal")
	}
	if clicked.menuOpen {
		t.Error("activating an entry closes the menu")
	}
	if len(clicked.ops.running) != 1 || clicked.ops.running[0].kind != domain.OpKindClean {
		t.Fatalf("running = %+v, want the clean run recorded", clicked.ops.running)
	}
}

func TestEnterOnTheDeleteEntryStartsTheSameRemoval(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, key(domain.KeyMenu))
	for range menuIndexOf(t, model, menuDelete) {
		model = update(model, key("j"))
	}

	model, cmd := updateCmd(model, namedKey(tea.KeyEnter))

	if cmd == nil || len(model.ops.running) != 1 {
		t.Fatalf("running = %+v, want enter to start the same run the click does", model.ops.running)
	}
	if model.ops.running[0].kind != domain.OpKindClean {
		t.Errorf("kind = %q, want enter to activate the entry it is on", model.ops.running[0].kind)
	}
}

// The context menu acts on the worktree it was opened from, so it starts a
// reparent already holding that one — the modal only asks for the new parent.
func TestTheMenuStartsAReparentOnTheSelectedWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	model = update(model, key("j"))
	model = update(model, key(domain.KeyMenu))
	index := menuIndexOf(t, model, menuReparent)

	started, cmd := model.activateMenu(index)

	if cmd == nil {
		t.Fatal("activating the reparent entry must start the run")
	}
	if len(started.ops.running) != 1 || started.ops.running[0].kind != domain.OpKindReparent {
		t.Fatalf("running = %+v, want the reparent run recorded", started.ops.running)
	}
	if got := started.ops.running[0].target; got != "b" {
		t.Errorf("target = %q, want the worktree the menu was opened on", got)
	}
}

// A reparent is not destructive, so it must not read as one before it is used.
func TestTheReparentEntryIsNotMarkedDangerous(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	index := menuIndexOf(t, model, menuReparent)

	if model.menuItems()[index].danger {
		t.Error("changing a parent destroys nothing")
	}
}

// Every entry acts on the same worktree, so one run holding it disables them all.
func TestARunHoldingTheWorktreeDisablesEveryEntry(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model.ops, _ = model.ops.begin(operation{kind: domain.OpKindCreate, target: "a"})

	for _, item := range model.menuItems() {
		if item.disabled == "" {
			t.Errorf("entry %q stays usable while a run holds its worktree", item.label)
		}
	}
}

// The frame under an open menu carries no zone at all, so a click beside the menu
// dismisses it and nothing else — which is what a context menu does everywhere.
func TestClickingOffTheMenuOnlyClosesIt(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(2))
	model = update(model, key(domain.KeyMenu))
	renderAndWait(t, model, menuZone(0))

	// Right on a row of the frame, whose zone the last unobstructed frame left
	// behind: the menu still swallows it.
	model = update(model, click(rowTextX, rowY(2)))

	if model.menuOpen {
		t.Fatal("a click elsewhere closes the menu")
	}
	if model.cursor != 0 {
		t.Errorf("cursor = %d, want the dismissing click to have selected nothing", model.cursor)
	}
}

func TestTheFrameIsNotClickableUnderAnOverlay(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(2), zoneAdd)

	model = update(model, key(domain.KeyMenu))
	model.View()

	// Marking the frame under an overlay would mean cutting through its markers
	// when the box is pasted over it, and losing the zones they carried.
	if _, ok := model.marks().(noMarks); !ok {
		t.Errorf("marks() = %T while the menu is open, want the frame left unmarked", model.marks())
	}
}

// The entries sit on adjacent lines, so what keeps a click off the wrong one is
// that each spans the whole box: the pointer is always unambiguously inside one
// block, not in the space beside a label.
func TestEveryMenuEntryIsClickableAcrossTheWholeBox(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	renderAndWait(t, model, rowZone(0))
	model = update(model, key(domain.KeyMenu))
	// Every entry, not just the first two: the zones are scanned asynchronously,
	// so reading one that was never waited for hands back a nil zone.
	ids := make([]string, 0, len(model.menuItems()))
	for index := range model.menuItems() {
		ids = append(ids, menuZone(index))
	}
	renderAndWait(t, model, ids...)

	_, rect := model.menuBox()
	for index := range model.menuItems() {
		zone := model.zones.Get(menuZone(index))
		width := zone.EndX - zone.StartX + 1
		if want := rect.Width - 2*menuBorder - 2*menuPadding; width < want {
			t.Errorf("entry %d is clickable over %d columns, want the full %d", index, width, want)
		}
	}
}

// The two menus share their box and their keys; what they list is what differs.
// The global one acts on worktrees picked inside the run, so it must not be
// keyed off the selected row.
func TestTheActionsMenuListsGlobalActionsWithNoSelection(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	model = update(model, worktreesMsg{statuses: nil, parents: map[string]string{}})
	model = update(model, key(domain.KeyActions))

	if !model.menuOpen || model.menuKind != menuForGlobal {
		t.Fatal("a on an empty dashboard must still open the global menu")
	}
	items := model.menuItems()
	if len(items) == 0 {
		t.Fatal("the global menu must offer something with no worktree selected")
	}
	if items[0].action != menuFastForwardAll {
		t.Errorf("first entry = %v, want the batch fast-forward", items[0].action)
	}
	if title, ok := model.menuTitle(); !ok || title != domain.DashboardActionsTitle {
		t.Errorf("title = %q, want the menu to name itself rather than a worktree", title)
	}
}

func TestTheActionsMenuStartsTheBatchRun(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, key(domain.KeyActions))

	started, cmd := model.activateMenu(menuIndexOf(t, model, menuReparentBatch))

	if cmd == nil {
		t.Fatal("activating the entry must start the run")
	}
	if len(started.ops.running) != 1 || started.ops.running[0].kind != domain.OpKindReparent {
		t.Fatalf("running = %+v, want the reparent run recorded", started.ops.running)
	}
}

// A blocking run holds every action, and the entry has to say so rather than
// look available.
func TestTheActionsMenuGoesInertWhileARunHoldsTheSurface(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model.ops, _ = model.ops.begin(operation{kind: domain.OpKindClean, mode: flow.ModeBlocking})
	model = update(model, key(domain.KeyActions))

	for _, item := range model.menuItems() {
		if item.disabled == "" {
			t.Errorf("entry %q stays usable while a run holds the dashboard", item.label)
		}
	}
}

// prune is the second entry that acts on worktrees picked inside the run, and it
// destroys them — so it reads as dangerous before it is activated.
func TestTheActionsMenuOffersPrune(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, key(domain.KeyActions))

	index := menuIndexOf(t, model, menuPrune)
	if item := model.menuItems()[index]; !item.danger || item.label != domain.DashboardMenuPrune {
		t.Errorf("entry = %+v, want a danger-marked prune entry", item)
	}
}

func TestTheActionsMenuStartsThePruneRun(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, key(domain.KeyActions))

	started, cmd := model.activateMenu(menuIndexOf(t, model, menuPrune))

	if cmd == nil {
		t.Fatal("activating the entry must start the run")
	}
	if len(started.ops.running) != 1 || started.ops.running[0].kind != domain.OpKindPrune {
		t.Fatalf("running = %+v, want the prune run recorded", started.ops.running)
	}
	// It holds the whole surface and names no target: several worktrees go, so
	// there is no single one to lock.
	if got := started.ops.running[0]; got.mode != flow.ModeBlocking || got.target != "" {
		t.Errorf("operation = %+v, want a blocking run with no target", got)
	}
}

// The row menu leads with the fast-forward — the least destructive thing a row
// offers — then the rebase, which the Tree tab flags and which is acted on here.
func TestTheRowMenuLeadsWithFastForwardThenSync(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")

	items := model.worktreeMenuItems()

	if len(items) < 2 || items[0].action != menuFastForward || items[1].action != menuSync {
		t.Fatalf("items = %+v, want the fast-forward then the sync", items)
	}
	if items[0].label != domain.DashboardMenuFastForward || items[0].danger {
		t.Errorf("entry = %+v, want the fast-forward named and not marked dangerous", items[0])
	}
	if items[1].label != domain.DashboardMenuSync || items[1].danger {
		t.Errorf("entry = %+v, want the sync named and not marked dangerous", items[1])
	}
}

// The base row had no menu at all: it hangs off nothing, so there is nothing to
// rebase it onto — only its own refresh.
func TestTheBaseRowOffersTheBaseRefreshAlone(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	model = update(model, worktreesMsg{
		statuses: []domain.WorktreeStatus{{Branch: "main", IsParent: true}},
		parents:  map[string]string{},
	})

	items := model.worktreeMenuItems()

	if len(items) != 1 || items[0].action != menuFastForward {
		t.Fatalf("items = %+v, want exactly the fast-forward", items)
	}
	if items[0].label != domain.DashboardMenuFastForward {
		t.Errorf("label = %q, want the fast-forward named", items[0].label)
	}
}

func TestTheActionsMenuOffersSyncAll(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, key(domain.KeyActions))

	item := model.menuItems()[menuIndexOf(t, model, menuSyncAll)]
	if item.label != domain.DashboardMenuSyncAll {
		t.Errorf("label = %q, want the entry named %q", item.label, domain.DashboardMenuSyncAll)
	}
	if item.danger {
		t.Error("a rebase destroys nothing: it must not read as dangerous")
	}
}

func TestTheActionsMenuStartsTheSyncRun(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, key(domain.KeyActions))

	started, cmd := model.activateMenu(menuIndexOf(t, model, menuSyncAll))

	if cmd == nil {
		t.Fatal("activating the entry must start the run")
	}
	if len(started.ops.running) != 1 || started.ops.running[0].kind != domain.OpKindSync {
		t.Fatalf("running = %+v, want the sync run recorded", started.ops.running)
	}
}

func TestTheRowMenuStartsTheSyncOnItsWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, key("j"))
	model = update(model, key(domain.KeyMenu))

	started, cmd := model.activateMenu(menuIndexOf(t, model, menuSync))

	if cmd == nil {
		t.Fatal("activating the sync entry must start the run")
	}
	if len(started.ops.running) != 1 || started.ops.running[0].kind != domain.OpKindSync {
		t.Fatalf("running = %+v, want the sync run recorded", started.ops.running)
	}
}

func TestTheBaseRowStartsItsOwnFastForward(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	model = update(model, worktreesMsg{
		statuses: []domain.WorktreeStatus{{Branch: "main", IsParent: true}},
		parents:  map[string]string{},
	})
	model = update(model, key(domain.KeyMenu))

	started, cmd := model.activateMenu(menuIndexOf(t, model, menuFastForward))

	if cmd == nil {
		t.Fatal("activating the fast-forward entry must start the run")
	}
	if len(started.ops.running) != 1 || started.ops.running[0].kind != domain.OpKindFastForward {
		t.Fatalf("running = %+v, want the fast-forward run recorded", started.ops.running)
	}
}

func TestWorktreeMenuLeadsWithFastForward(t *testing.T) {
	items := worktreeActions(domain.WorktreeStatus{
		Branch:       "feat",
		OriginState:  domain.DivergenceBehind,
		OriginBehind: 2,
	})
	if len(items) == 0 || items[0].action != menuFastForward {
		t.Fatalf("first entry = %+v, want menuFastForward", items)
	}
	if items[0].label != domain.DashboardMenuFastForward {
		t.Fatalf("label = %q, want %q", items[0].label, domain.DashboardMenuFastForward)
	}
}

func TestBaseRowOffersTheSameFastForwardEntry(t *testing.T) {
	items := worktreeActions(domain.WorktreeStatus{
		Branch:      "main",
		IsParent:    true,
		OriginState: domain.DivergenceBehind,
	})
	if len(items) != 1 || items[0].action != menuFastForward {
		t.Fatalf("base row entries = %+v, want one menuFastForward", items)
	}
}

// The origin badges come from cached remote-tracking refs with no fetch, so
// every one of them says what is known rather than what is true: a branch shown
// up to date may be behind, and one shown without a counterpart may have been
// pushed from another machine. None of them may gate the entry — the run
// fetches and reports the truth.
func TestFastForwardIsNeverGatedOnTheCachedOriginBadges(t *testing.T) {
	states := []domain.DivergenceState{
		domain.DivergenceUpToDate,
		domain.DivergenceBehind,
		domain.DivergenceAhead,
		domain.DivergenceDiverged,
		domain.DivergenceUnknown,
	}
	for _, state := range states {
		items := worktreeActions(domain.WorktreeStatus{Branch: "feat", OriginState: state})
		if items[0].disabled != "" {
			t.Errorf("state %v: disabled = %q, want it enabled", state, items[0].disabled)
		}
	}
}

// The only thing that makes an entry inert is a run holding the worktree — the
// one meaning disabled has ever carried on this menu.
func TestOnlyARunningOperationDisablesTheFastForward(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model.ops, _ = model.ops.begin(operation{kind: domain.OpKindCreate, target: "a"})

	items := model.worktreeMenuItems()
	if items[0].disabled == "" {
		t.Fatal("a worktree another run is holding must disable its entries")
	}
}

func TestGlobalMenuOffersTheBatchFastForward(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	var found bool
	for _, item := range model.globalMenuItems() {
		if item.action != menuFastForwardAll {
			continue
		}
		found = true
		if item.label != domain.DashboardMenuFastForwardAll {
			t.Fatalf("label = %q, want %q", item.label, domain.DashboardMenuFastForwardAll)
		}
		if item.disabled != "" {
			t.Fatalf("disabled = %q, want it enabled", item.disabled)
		}
	}
	if !found {
		t.Fatal("global menu has no menuFastForwardAll entry")
	}
}
