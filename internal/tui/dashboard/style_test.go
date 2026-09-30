package dashboard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/styles"
)

func TestAWorktreeRowIsTwoLinesAndSaysWhatItsStateIs(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model.statuses[1] = domain.WorktreeStatus{Branch: "b", Path: "/tmp/b", IsDirty: true, CommitsAhead: 3}
	model.parents = map[string]string{"b": "main"}
	model = update(model, prsMsg{prs: []domain.PRInfo{{Number: 61, Branch: "b"}}})

	lines := model.renderRow(1, model.layout().List.Width-borderWidth-paddingWidth)

	if len(lines) != domain.DashboardRowHeight {
		t.Fatalf("a row is %d lines, want %d", len(lines), domain.DashboardRowHeight)
	}
	if !strings.Contains(lines[0], "b") {
		t.Error("the first line names the worktree")
	}
	for _, want := range []string{"main", "#61", "base"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("the second line is missing %q — that is what it is there for:\n%s", want, lines[1])
		}
	}
	if lipgloss.Width(lines[0]) != lipgloss.Width(lines[1]) {
		t.Errorf("the two lines are %d and %d wide; a row is one block and its zone covers both",
			lipgloss.Width(lines[0]), lipgloss.Width(lines[1]))
	}
}

func TestTheSelectedRowIsTintedAcrossItsWholeWidth(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	width := model.layout().List.Width - borderWidth - paddingWidth

	selected := model.renderRow(0, width)
	plain := model.renderRow(1, width)

	if !strings.Contains(selected[0], rowBar) {
		t.Error("the selected row carries the accent bar")
	}
	if strings.Contains(plain[0], rowBar) {
		t.Error("only the selected row carries the bar")
	}
	if selected[0] == plain[0] {
		t.Error("the selection must be visible on the row itself, not only through the detail panel")
	}
	for _, line := range append(selected, plain...) {
		if lipgloss.Width(line) != width {
			t.Errorf("row line is %d wide, want the full %d so the block reads as one", lipgloss.Width(line), width)
		}
	}
}

// TestTheTallHeaderIsASignatureBlock pins the six-row structure at a
// comfortably tall terminal (testHeight is above
// domain.DashboardHeaderTallThreshold): three rows of drawn wordmark, each
// carrying one piece of context, a blank line (not optional — "too close to
// the tabs" is one of the three complaints it fixes), the tab bar with its
// count-free right cluster, and the rule.
func TestTheTallHeaderIsASignatureBlock(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model.repoName, model.activeBranch = "worktree-manager-cli", "a"
	model.params.Config.Project.Worktrees.BaseBranch = "main"

	header := model.renderHeader(model.layout())
	lines := strings.Split(header, "\n")

	if len(lines) != domain.DashboardHeaderTallHeight {
		t.Fatalf("the header is %d lines, want %d", len(lines), domain.DashboardHeaderTallHeight)
	}
	for i := 0; i < 3; i++ {
		if !strings.Contains(lines[i], domain.DashboardWordmarkLines[i]) {
			t.Errorf("line %d = %q, want the wordmark row %q", i, lines[i], domain.DashboardWordmarkLines[i])
		}
	}
	if !strings.Contains(lines[0], "worktree-manager-cli") {
		t.Errorf("line 0 = %q, want the repository name", lines[0])
	}
	if !strings.Contains(lines[1], "main") || !strings.Contains(lines[1], "a") {
		t.Errorf("line 1 = %q, want the base branch and the active worktree", lines[1])
	}
	if !strings.Contains(lines[2], "2 worktrees") {
		t.Errorf("line 2 = %q, want the worktree count", lines[2])
	}
	if strings.TrimSpace(lines[3]) != "" {
		t.Errorf("line 3 = %q, want a blank line separating the block from the tabs", lines[3])
	}
	if !strings.Contains(lines[4], domain.DashboardTabWorktrees) {
		t.Errorf("line 4 = %q, want the tabs", lines[4])
	}
	if strings.Contains(lines[4], "2 worktrees") {
		t.Error("the tab bar must not repeat the count — it already sits in the signature block")
	}
	if !strings.Contains(lines[5], domain.DashboardActiveRuleGlyph) {
		t.Errorf("line 5 = %q, want the active tab underlined rather than filled", lines[5])
	}
}

// TestTheSignatureBlockRowsAlignWithTheRepoNameAbove pins that "base main ·
// ● branch" and "N worktrees · fetched" start at the same column as the
// repository name on the row above them — no extra indent from a leading
// space meant for the compact header's inline wordmark text (row 1) or the
// count's own padding for its place at the end of the compact tab bar
// (row 2).
func TestTheSignatureBlockRowsAlignWithTheRepoNameAbove(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model.repoName, model.activeBranch = "worktree-manager-cli", "a"
	model.params.Config.Project.Worktrees.BaseBranch = "main"
	model.fetchedAt = time.Now().Add(-72 * time.Hour)

	lines := strings.Split(model.renderHeader(model.layout()), "\n")
	repoCol := stripANSI(lines[0])[:strings.Index(stripANSI(lines[0]), "worktree-manager-cli")]
	baseCol := stripANSI(lines[1])[:strings.Index(stripANSI(lines[1]), "base")]
	countCol := stripANSI(lines[2])[:strings.Index(stripANSI(lines[2]), "2 worktrees")]

	if lipgloss.Width(repoCol) != lipgloss.Width(baseCol) {
		t.Errorf("repo name starts at column %d, base branch at column %d — they must align",
			lipgloss.Width(repoCol), lipgloss.Width(baseCol))
	}
	if lipgloss.Width(repoCol) != lipgloss.Width(countCol) {
		t.Errorf("repo name starts at column %d, worktree count at column %d — they must align",
			lipgloss.Width(repoCol), lipgloss.Width(countCol))
	}
}

// TestTheHeaderFallsBackBelowTheTallThreshold pins the degrade rule itself:
// a terminal too short for the signature block gets the compact header
// instead — six rows of chrome would be a quarter of a 24-row terminal.
func TestTheHeaderFallsBackBelowTheTallThreshold(t *testing.T) {
	model := newTestModel(t, testWidth, domain.DashboardHeaderTallThreshold-1, "a", "b")

	header := model.renderHeader(model.layout())
	lines := strings.Split(header, "\n")

	if len(lines) != domain.DashboardHeaderCompactHeight {
		t.Fatalf("the header is %d lines, want the compact %d", len(lines), domain.DashboardHeaderCompactHeight)
	}
	if !strings.Contains(lines[0], domain.DashboardWordmark) {
		t.Errorf("context line = %q, want the plain wordmark", lines[0])
	}
	if strings.Contains(header, domain.DashboardWordmarkLines[0]) {
		t.Error("the drawn wordmark must not appear below the tall threshold")
	}
	if !strings.Contains(lines[1], "2 worktrees") {
		t.Errorf("header bar = %q, want the count of what is listed — the compact header keeps it there", lines[1])
	}
	if !strings.Contains(lines[2], domain.DashboardActiveRuleGlyph) {
		t.Errorf("rule = %q, want the active tab underlined rather than filled", lines[2])
	}
}

func TestTheHeaderCountAgreesWithTheList(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "only")

	if got := model.renderHeader(model.layout()); !strings.Contains(got, "1 worktree ") {
		t.Errorf("header = %q, want a single worktree named in the singular", got)
	}
}

// Points 3 of the style pass: the two confirmations are the same widget, so they
// cannot drift apart again.
func TestBothConfirmationsAreDrawnWithTheSameButtons(t *testing.T) {
	create, _ := openStepper(t, stepperSession())
	create = typeText(t, create, "feature/new")
	create = press(t, create, namedKey(13))
	create = press(t, create, namedKey(13))

	remove, _ := openDeleteForm(t, deleteSession())

	createButtons := buttonLabels(create)
	removeButtons := buttonLabels(remove)
	if len(createButtons) != 2 || len(removeButtons) != 2 {
		t.Fatalf("buttons: create %v, delete %v — both confirm and cancel", createButtons, removeButtons)
	}
	if createButtons[1] != removeButtons[1] {
		t.Errorf("cancel reads %q on a create and %q on a delete", createButtons[1], removeButtons[1])
	}
	if create.modal.renderButton(0, formRow{label: "X", confirm: true}) != remove.modal.renderButton(0, formRow{label: "X", confirm: true}) {
		t.Error("the same button must render the same way in both modals")
	}
}

func buttonLabels(model Model) []string {
	var labels []string
	for _, row := range model.modal.rows {
		if row.kind == formButton {
			labels = append(labels, row.label)
		}
	}
	return labels
}

func TestARecapStepIsAConfirmationNotAnotherList(t *testing.T) {
	model, _ := openStepper(t, stepperSession())
	model = typeText(t, model, "feature/new")
	model = press(t, model, namedKey(13))
	model = press(t, model, namedKey(13))

	if !model.modal.usesRows() {
		t.Fatal("the recap must be drawn as a confirmation")
	}
	if got := model.modal.hint(); got != domain.DashboardStepperRowsHint {
		t.Errorf("hint = %q, want the keys a stepper confirmation takes", got)
	}
	if strings.Contains(strings.Join(model.modal.body(m0zones{}), "\n"), domain.WizardCancelValue) {
		t.Error("the cancel row must read as a button, not carry its wizard value")
	}
}

func TestAPlainConfirmSaysNothingAboutToggling(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{{
		Kind: flow.StepRecap, Key: keyConfirm, Title: "Sure?",
		Options: []flow.Option{{Label: "Yes", Value: confirmYes}},
	}}}
	model := newTestModel(t, testWidth, testHeight, "a")
	reply := make(chan promptReply, 1)
	model = prompt(t, model, promptMsg{shape: modalForm, session: session, reply: reply})

	if got := model.modal.hint(); got != domain.DashboardConfirmHint {
		t.Errorf("hint = %q, want no mention of toggling in a form with nothing to toggle", got)
	}
}

func TestTheRowMetaStartsWhereTheNameDoes(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model.parents = map[string]string{"b": "main"}
	width := model.layout().List.Width - borderWidth - paddingWidth

	for index, name := range []string{"a", "b"} {
		lines := model.renderRow(index, width)
		if indentOf(lines[0]) != indentOf(lines[1]) {
			t.Errorf("%s: the name starts at column %d and the line under it at %d — they must align",
				name, indentOf(lines[0]), indentOf(lines[1]))
		}
	}
}

// indentOf counts the leading blanks of a rendered line, styling aside.
func indentOf(line string) int {
	plain := stripANSI(line)
	return len(plain) - len(strings.TrimLeft(plain, " ▌"))
}

func stripANSI(text string) string {
	var out strings.Builder
	inEscape := false
	for _, char := range text {
		switch {
		case char == '\x1b':
			inEscape = true
		case inEscape && (char == 'm' || char == 'z'):
			inEscape = false
		case !inEscape:
			out.WriteRune(char)
		}
	}
	return out.String()
}

func TestTheContextMenuNamesItsWorktreeAndRulesItOff(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "feature/x")
	model = update(model, key("j"))
	model = update(model, key(domain.KeyMenu))

	box, rect := model.menuBox()
	lines := strings.Split(box, "\n")

	if !strings.Contains(box, "feature/x") {
		t.Error("the menu must name the worktree it acts on")
	}
	if !strings.Contains(box, domain.DashboardRuleGlyph) {
		t.Error("the actions must be ruled off from that name")
	}
	// Everything in the box hangs off the same left edge: the name, the rule and
	// the entries, with the focused one marked by the tint alone.
	for index, line := range lines[1:3] {
		if indentOf(line) != 0 {
			t.Errorf("menu line %d starts at column %d, want it flush left", index+1, indentOf(line))
		}
	}
	if rect.Height != len(lines) {
		t.Errorf("the placement rule reserves %d rows for a box of %d", rect.Height, len(lines))
	}
}

func TestOnlyTheHeaderButtonIsAFilledBlock(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	form, _ := openDeleteForm(t, deleteSession())

	button := form.modal.renderButton(99, formRow{label: "Yes, delete", confirm: true})

	if !strings.Contains(stripANSI(button), "[ Yes, delete ]") {
		t.Errorf("an unfocused action reads by its brackets, not by a slab: %q", stripANSI(button))
	}
	if strings.Contains(button, styles.DashboardAddButton.Render("")) && styles.DashboardAddButton.Render("") != "" {
		t.Error("the filled block belongs to the header button alone")
	}
	if !strings.Contains(model.headerRight(testWidth), domain.DashboardAddLabelLong) {
		t.Error("the header button says what it does when there is room for it")
	}
}

func TestTheDetailIsGroupedUnderHeadings(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model.statuses[0] = domain.WorktreeStatus{Branch: "a", Path: "/tmp/a", IsDirty: true, CommitsAhead: 2}
	model = update(model, prsMsg{})

	body := model.detailBody(model.layout())

	index := lineIndex(body, domain.DetailSectionLinks)
	if index < 0 {
		t.Fatalf("the detail is missing the %q group", domain.DetailSectionLinks)
	}
	if body[index-1] != "" || body[index+1] != "" {
		t.Errorf("%q must stand on its own, with a blank line either side", domain.DetailSectionLinks)
	}
	if !strings.Contains(body[0], "a") {
		t.Errorf("the heading = %q, want the worktree name", body[0])
	}
	if strings.Contains(body[0], "dirty") {
		t.Error("the working-tree state belongs to the vital strip, not the title row")
	}
	if !strings.Contains(body[1], domain.DashboardRuleGlyph) {
		t.Error("a rule must separate the heading from the vital strip")
	}
}

func lineIndex(lines []string, needle string) int {
	for index, line := range lines {
		if strings.Contains(line, needle) {
			return index
		}
	}
	return -1
}

// The base has no parent to be moved to and cannot be deleted, so the only graph
// action its row offers is catching up with its own remote — the run entries
// apply to it like to any other worktree.
func TestTheParentWorktreeIsOfferedOnlyItsOwnFastForward(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feature/x")
	model.statuses[0] = domain.WorktreeStatus{Branch: "main", Path: "/tmp/main", IsParent: true}

	items := menuActions(model.menuItems())
	if items[0].action != menuFastForward {
		t.Fatalf("items = %+v, want the base fast-forward first: no other graph action could apply", items)
	}

	model = update(model, key(domain.KeyMenu))
	box, _ := model.menuBox()

	if !strings.Contains(box, "main") {
		t.Error("the menu still names the worktree it was opened on")
	}
	if !strings.Contains(box, domain.DashboardMenuFastForward) {
		t.Errorf("menu = %q, want the one action it does offer", box)
	}

	model, cmd := updateCmd(model, namedKey(13))
	if cmd == nil || len(model.ops.running) != 1 {
		t.Fatalf("running = %+v, want enter to start the fast-forward", model.ops.running)
	}
	if model.ops.running[0].kind != domain.OpKindFastForward {
		t.Errorf("kind = %q, want the fast-forward flow", model.ops.running[0].kind)
	}
}

// A worktree a run is holding keeps its entry — the removal will be possible
// again — but the entry is inert, unfocusable and unclickable, with what is in
// its way written under it.
func TestAHeldWorktreeKeepsAnInertEntryThatSaysWhy(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "feat")
	model = creating(t, model, "feat")
	model = selectBranch(t, model, "feat")

	item := menuActions(model.menuItems())[0]
	if item.disabled == "" {
		t.Fatal("the entry must say it cannot be used")
	}
	if !strings.Contains(item.disabled, domain.OpKindCreate) {
		t.Errorf("caption = %q, want it to name what is in the way", item.disabled)
	}

	model = update(model, key(domain.KeyMenu))
	box, _ := model.menuBox()
	if !strings.Contains(stripANSI(box), item.disabled) {
		t.Errorf("menu = %q, want the reason under the entry", box)
	}

	model.View()
	if !model.zones.Get(menuZone(0)).IsZero() {
		t.Error("an inert entry must not be clickable at all")
	}
}

// styles/ owns every visual attribute. A .Width()/.MaxWidth() chained in tui/ is
// a dimension and stays allowed; a .Bold()/.Foreground()/.Underline() is a
// choice and belongs in a named style.
func TestNoStyleAttributeIsChosenOutsideStyles(t *testing.T) {
	forbidden := regexp.MustCompile(`styles\.[A-Za-z]+\.(Bold|Foreground|Background|Underline|Italic|Faint)\(`)

	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if loc := forbidden.FindIndex(body); loc != nil {
			t.Errorf("%s chooses a style attribute: %q", path, body[loc[0]:loc[1]])
		}
	}
}
