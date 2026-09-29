package dashboard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// sampleForest is main ← feat ← feat-ui, plus a virtual root standing in for a
// parent branch nobody has a worktree for.
func sampleForest() domain.Forest {
	return domain.Forest{
		Roots: []domain.TreeNode{
			{
				Branch: "main", IsMain: true,
				Children: []domain.TreeNode{{
					Branch: "feat",
					Status: domain.TreeNodeStatus{CommitsAhead: 3},
					Children: []domain.TreeNode{{
						Branch: "feat-ui",
						Status: domain.TreeNodeStatus{NeedsSync: true},
					}},
				}},
			},
			{Branch: "dev", IsVirtual: true},
		},
	}
}

// treeRowY is the screen row of a tree node: each takes its own line plus the
// spacer that carries the gutter down to the next one.
func treeRowY(index int) int {
	return firstRowY + index*domain.DashboardTreeRowHeight
}

func treeModel(t *testing.T, branches ...string) Model {
	t.Helper()
	model := newTestModel(t, testWidth, testHeight, branches...)
	model, _ = model.selectTab(tabTree)
	return update(model, treeMsg{rows: rules.FlattenForest(sampleForest())})
}

func TestTheTreeIsOnlyBuiltOnceItsTabIsOpened(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat")

	if model.treeLoading || model.treeLoaded {
		t.Fatal("a user who never opens the Tree tab must not pay for the forest")
	}

	opened, cmd := model.selectTab(tabTree)

	if cmd == nil {
		t.Fatal("opening the tab must ask for the forest")
	}
	if !opened.treeLoading {
		t.Error("the tab must remember it already asked, so a second tab press does not ask again")
	}
	if _, second := opened.selectTab(tabTree); second != nil {
		t.Error("a tab already loading must not ask a second time")
	}
}

func TestTheTreeDrawsItsGuttersAndBadges(t *testing.T) {
	model := treeModel(t, "main", "feat", "feat-ui")

	body := strings.Join(model.treeBody(model.layout()), "\n")

	for _, want := range []string{
		"main", "feat", "feat-ui", "dev",
		domain.TreeConnectorLast, domain.TreeGutterBlank,
		domain.TreeBadgeNeedsSyncText,
		domain.DashboardTreeVirtual,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("tree body missing %q:\n%s", want, body)
		}
	}
}

func TestTheTreeSaysWhatItIsDoingBeforeItHasAForest(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model, _ = model.selectTab(tabTree)

	body := strings.Join(model.treeBody(model.layout()), "\n")
	if !strings.Contains(body, domain.DashboardLoadingTree) {
		t.Errorf("an empty panel says nothing; body was:\n%s", body)
	}
}

func TestTheTreeReportsAFailureInsteadOfDrawingNothing(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model, _ = model.selectTab(tabTree)
	model = update(model, treeMsg{err: domain.ErrWorktreeNotFound})

	body := strings.Join(model.treeBody(model.layout()), "\n")
	if !strings.Contains(body, domain.ErrWorktreeNotFound.Error()) {
		t.Errorf("a failed build must say so:\n%s", body)
	}
}

// Each tab keeps its own cursor: coming back to a tab must not have moved what
// was selected there.
func TestEachTabKeepsItsOwnCursor(t *testing.T) {
	model := withRunJobs(treeModel(t, "main", "feat", "feat-ui"))
	model = update(model, key("j"))
	model = update(model, key("j"))
	if model.treeCursor != 2 {
		t.Fatalf("tree cursor = %d, want it moved", model.treeCursor)
	}
	if model.cursor != 0 {
		t.Fatalf("list cursor = %d, want it untouched", model.cursor)
	}

	// Tab cycles Worktrees → Tree → Running, so two presses come back to the list.
	model = update(model, key(keyTab))
	model = update(model, key(keyTab))
	model = update(model, key("j"))

	if model.cursor != 1 {
		t.Errorf("list cursor = %d, want the list to move on its own tab", model.cursor)
	}
	if model.treeCursor != 2 {
		t.Errorf("tree cursor = %d, want it left where it was", model.treeCursor)
	}
}

// A tree row is a node; the detail panel and the context menu both key off the
// worktree it names, so both keep working from the Tree tab.
func TestATreeRowSelectsTheWorktreeItNames(t *testing.T) {
	model := treeModel(t, "main", "feat", "feat-ui")
	model = update(model, key("j"))

	status, ok := model.selected()
	if !ok {
		t.Fatal("a node with a worktree must select it")
	}
	if status.Branch != "feat" {
		t.Errorf("selected = %q, want the node the cursor is on", status.Branch)
	}
	if len(model.menuItems()) == 0 {
		t.Error("the context menu must be reachable from the tree")
	}
}

// A virtual node stands for a branch with no worktree: there is nothing to act
// on, and the menu must say so rather than offer something that would fail.
func TestAVirtualNodeSelectsNothingAndOffersNothing(t *testing.T) {
	model := treeModel(t, "main", "feat", "feat-ui")
	model.treeCursor = len(model.treeRows) - 1
	if node, _ := model.selectedTreeNode(); !node.IsVirtual {
		t.Fatalf("fixture drifted: last row is %+v, want the virtual root", node)
	}

	if _, ok := model.selected(); ok {
		t.Error("a branch with no worktree selects nothing")
	}
	if items := model.menuItems(); len(items) != 0 {
		t.Errorf("menu = %+v, want nothing offered on a virtual node", items)
	}
}

// The main worktree has no recorded parent to change and cannot be deleted, so
// on either tab its row offers nothing but its own refresh.
func TestTheMainWorktreeOffersOnlyTheBaseRefreshFromTheTree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	model = update(model, worktreesMsg{
		statuses: []domain.WorktreeStatus{{Branch: "main", IsParent: true}},
		parents:  map[string]string{},
	})
	model, _ = model.selectTab(tabTree)
	model = update(model, treeMsg{rows: rules.FlattenForest(sampleForest())})

	items := menuActions(model.menuItems())
	if items[0].action != menuFastForward {
		t.Errorf("menu = %+v, want the base fast-forward first on the main worktree", items)
	}
}

func TestTheHeaderCountsWhatTheActiveTabLists(t *testing.T) {
	model := withRunJobs(treeModel(t, "main", "feat", "feat-ui"))

	if got := model.countLabel(); !strings.Contains(got, "4") {
		t.Errorf("count = %q, want the four nodes of the forest", got)
	}

	model = update(model, key(keyTab))
	if got := model.countLabel(); !strings.Contains(got, "0") {
		t.Errorf("count = %q, want the Running tab to count what is up", got)
	}

	model = update(model, key(keyTab))
	if got := model.countLabel(); !strings.Contains(got, "3") {
		t.Errorf("count = %q, want the three worktrees", got)
	}
}

// A finished run changes the shape of the forest, not only the rows of the list.
func TestAFinishedRunRebuildsTheTreeOnceItExists(t *testing.T) {
	model := treeModel(t, "main", "feat")

	if cmd := model.treeCmd(); cmd == nil {
		t.Error("a built tree must be rebuilt after a run")
	}

	fresh := newTestModel(t, testWidth, testHeight, "main")
	if cmd := fresh.treeCmd(); cmd != nil {
		t.Error("a tree nobody has opened must not be built by a run")
	}
}

func TestClickingATreeRowSelectsIt(t *testing.T) {
	model := treeModel(t, "main", "feat", "feat-ui")
	renderAndWait(t, model, treeRowZone(1))

	model = update(model, click(rowTextX+2, treeRowY(1)))

	if model.treeCursor != 1 {
		t.Errorf("tree cursor = %d, want the row the click landed on", model.treeCursor)
	}
}

func TestRightClickingATreeRowOpensItsMenu(t *testing.T) {
	model := treeModel(t, "main", "feat", "feat-ui")
	renderAndWait(t, model, treeRowZone(1))

	model = update(model, rightClick(rowTextX+2, treeRowY(1)))

	if model.treeCursor != 1 {
		t.Fatalf("tree cursor = %d, want the menu to act on the row under the pointer", model.treeCursor)
	}
	if !model.menuOpen {
		t.Fatal("a right click on a tree row opens its context menu")
	}
}

func TestTheWheelScrollsTheTreePanel(t *testing.T) {
	model := treeModel(t, "main", "feat", "feat-ui")
	renderAndWait(t, model, zoneTree)

	model = update(model, wheel(rowTextX, firstRowY, tea.MouseButtonWheelDown))

	if model.treeCursor != 1 {
		t.Errorf("tree cursor = %d, want the wheel to move it", model.treeCursor)
	}
}

// The spacer under a row is what keeps the tree connected while it breathes: a
// blank line there would cut the gutter in two.
func TestTheTreeSpacerCarriesTheGutterOnDown(t *testing.T) {
	model := treeModel(t, "main", "feat", "feat-ui")

	body := model.treeBody(model.layout())

	if len(body) < 4 {
		t.Fatalf("body = %v, want a spacer under each row", body)
	}
	if !strings.Contains(body[1], domain.TreeGutterPipe) {
		t.Errorf("line under the root = %q, want it to carry the gutter down", body[1])
	}
}

// The last root of the sample forest is virtual and starts a new tree, so the
// line above it carries nothing — that is what sets the two trees apart.
func TestTheTreeBreaksTheGutterBetweenRoots(t *testing.T) {
	model := treeModel(t, "main", "feat", "feat-ui")

	rows := model.treeRows
	last := len(rows) - 1
	if rows[last].Depth != 0 {
		t.Fatalf("fixture drifted: last row is at depth %d, want a root", rows[last].Depth)
	}
	if got := model.treeSpacer(last-1, 40); strings.TrimSpace(got) != "" {
		t.Errorf("spacer before a new root = %q, want a blank line", got)
	}
}

// TestActiveTreeNodeIsMarked pins that the Tree tab says the same thing about
// the active worktree the list already does: the same "you are here" tag,
// not a marker unique to one view.
func TestActiveTreeNodeIsMarked(t *testing.T) {
	model := treeModel(t, "main", "feat", "feat-ui")
	model.activeBranch = "feat"

	body := strings.Join(model.treeBody(model.layout()), "\n")
	if !strings.Contains(body, domain.DetailYouAreHere) {
		t.Error("le nœud du worktree actif doit porter le tag \"you are here\", comme la liste")
	}
}
