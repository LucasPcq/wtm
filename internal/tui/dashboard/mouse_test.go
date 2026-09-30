package dashboard

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// The coordinates below are derived from the layout rules, not read back from
// the zone manager: a zone marked over the wrong region is exactly the mistake a
// self-referential assertion would miss.
//
// At 120x40 (above the tall-header threshold) with the output panel folded:
//
//	y=0..2         signature block (drawn wordmark, one context row each)
//	y=3            the blank line under it
//	y=4            header bar (tabs and buttons)
//	y=5            the rule under the active tab
//	y=6            list/detail top border   (list x=0..47, detail x=48..119)
//	y=7            panel title
//	y=8            the blank line under it
//	y=9+3i         worktree row i           (two lines, then a gap; text at x=2)
//	y=36..38       output panel             (title row y=37)
//	y=39           help bar
const (
	headerBarY   = domain.DashboardHeaderTallHeight - 2
	titleRowY    = domain.DashboardHeaderTallHeight + 1
	firstRowY    = titleRowY + 1 + domain.DashboardTitleGap
	rowStride    = domain.DashboardRowHeight + domain.DashboardRowGap
	rowTextX     = 2
	detailX      = 60
	outputTitleY = 37
)

// rowY is the first line of worktree row i.
func rowY(index int) int { return firstRowY + index*rowStride }

func TestRowZonesAreMarkedWhereTheLayoutPutsThem(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(0), rowZone(2))

	first := model.zones.Get(rowZone(0))
	if first.StartX != rowTextX || first.StartY != firstRowY {
		t.Errorf("row 0 starts at (%d,%d), want (%d,%d)", first.StartX, first.StartY, rowTextX, firstRowY)
	}
	if want := model.layout().List.Width - borderWidth - paddingWidth + rowTextX - 1; first.EndX != want {
		t.Errorf("row 0 ends at x=%d, want %d — it must not reach into the detail panel", first.EndX, want)
	}

	third := model.zones.Get(rowZone(2))
	if third.StartY != rowY(2) {
		t.Errorf("row 2 sits at y=%d, want %d", third.StartY, rowY(2))
	}
	if height := third.EndY - third.StartY + 1; height != domain.DashboardRowHeight {
		t.Errorf("row 2 is %d lines tall, want %d", height, domain.DashboardRowHeight)
	}
}

func TestClickingARowSelectsThatWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(2))

	model = update(model, click(rowTextX+3, rowY(2)))

	if model.cursor != 2 {
		t.Fatalf("cursor = %d, want the clicked row 2", model.cursor)
	}
	selected, _ := model.selected()
	if selected.Branch != "c" {
		t.Errorf("selected %q, want the worktree the click landed on", selected.Branch)
	}
}

func TestClickingAScrolledRowSelectsTheRightWorktree(t *testing.T) {
	branches := make([]string, 30)
	for i := range branches {
		branches[i] = string(rune('a' + i))
	}
	// 13 is the shortest height that still fits one full row under the 3-line
	// header, the panel chrome and the folded output panel.
	model := newTestModel(t, testWidth, 13, branches...)
	model = update(model, key("G"))
	renderAndWait(t, model, rowZone(model.offset))

	model = update(model, click(rowTextX, firstRowY))

	if model.cursor != model.offset {
		t.Fatalf("cursor = %d, want the first visible row (%d) — zones must key on the absolute index",
			model.cursor, model.offset)
	}
}

func TestClickingOutsideTheListLeavesTheSelectionAlone(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(0), zoneDetail)

	for _, where := range []tea.MouseMsg{
		click(detailX, firstRowY), // the detail panel, beside row 0
		click(rowTextX, rowY(3)),  // below the last row
		click(rowTextX, 0),        // the header bar
	} {
		if got := update(model, where).cursor; got != 0 {
			t.Errorf("click at (%d,%d) moved the cursor to %d", where.X, where.Y, got)
		}
	}
}

func TestClickingTheOutputHeaderFoldsAndUnfolds(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	renderAndWait(t, model, zoneOutputToggle)

	model = update(model, click(10, outputTitleY))
	if !model.outputExpanded {
		t.Fatal("clicking the output header must unfold the panel")
	}

	// Unfolded, the panel grew upwards: its title row moved with it.
	expandedTitleY := model.layout().Output.Y + 1
	renderAndWaitAt(t, model, zoneOutputToggle, expandedTitleY)
	model = update(model, click(10, expandedTitleY))
	if model.outputExpanded {
		t.Fatal("clicking the header again must fold it back")
	}
}

func TestClickingATabActivatesIt(t *testing.T) {
	defer func(saved []string) { tabs = saved }(tabs)
	tabs = []string{domain.DashboardTabWorktrees, "Configuration"}

	model := newTestModel(t, testWidth, testHeight, "a")
	renderAndWait(t, model, tabZone(0), tabZone(1))

	second := model.zones.Get(tabZone(1))
	if second.StartY != headerBarY {
		t.Fatalf("tab 1 sits at y=%d, want the tab bar row", second.StartY)
	}
	if first := model.zones.Get(tabZone(0)); second.StartX <= first.EndX {
		t.Fatalf("tab 1 starts at x=%d but tab 0 ends at x=%d — the tabs overlap", second.StartX, first.EndX)
	}

	model = update(model, click(second.StartX+1, headerBarY))
	if model.tab != 1 {
		t.Errorf("tab = %d after clicking the second tab, want 1", model.tab)
	}
}

func TestWheelOverTheListMovesTheSelection(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, zoneList)

	model = update(model, wheel(rowTextX, firstRowY, tea.MouseButtonWheelDown))
	if model.cursor != 1 {
		t.Fatalf("cursor = %d after a wheel-down over the list, want 1", model.cursor)
	}

	model = update(model, wheel(rowTextX, firstRowY, tea.MouseButtonWheelUp))
	if model.cursor != 0 {
		t.Errorf("cursor = %d after a wheel-up, want 0", model.cursor)
	}
}

func TestWheelOverTheOutputPanelScrollsItInsteadOfTheList(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	model = update(model, key(domain.KeyToggleOutput))
	for i := 0; i < 30; i++ {
		model = update(model, OutputLineMsg{Text: "line"})
	}
	renderAndWait(t, model, zoneOutput)

	insideOutput := model.layout().Output.Y + 2
	before := model.outputOffset

	model = update(model, wheel(10, insideOutput, tea.MouseButtonWheelUp))

	if model.outputOffset != before-1 {
		t.Errorf("output offset = %d, want %d", model.outputOffset, before-1)
	}
	if model.cursor != 0 {
		t.Error("a wheel over the output panel must not move the worktree selection")
	}
}

func TestWheelOverAFoldedOutputPanelIsInert(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	renderAndWait(t, model, zoneOutput)

	model = update(model, wheel(10, outputTitleY, tea.MouseButtonWheelDown))

	if model.cursor != 0 || model.outputOffset != 0 {
		t.Errorf("folded panel: cursor=%d offset=%d, want both untouched", model.cursor, model.outputOffset)
	}
}

func TestNarrowClickOnARowOpensTheDetail(t *testing.T) {
	model := newTestModel(t, narrowWide, testHeight, "a", "b")
	renderAndWait(t, model, rowZone(1))

	model = update(model, click(rowTextX, rowY(1)))

	if model.cursor != 1 {
		t.Fatalf("cursor = %d, want the clicked row", model.cursor)
	}
	if !model.detailOpen {
		t.Error("a narrow terminal must open the detail on a row click, mirroring enter")
	}
}

// The right button is no longer among them: it opens the context menu, which
// TestRightClickSelectsTheRowAndOpensItsMenu covers.
func TestNonLeftMouseEventsAreIgnored(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(2))

	ignored := []tea.MouseMsg{
		{X: rowTextX, Y: rowY(2), Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft},
		{X: rowTextX, Y: rowY(2), Action: tea.MouseActionRelease, Button: tea.MouseButtonRight},
		{X: rowTextX, Y: rowY(2), Action: tea.MouseActionMotion, Button: tea.MouseButtonNone},
	}
	for _, msg := range ignored {
		if got := update(model, msg).cursor; got != 0 {
			t.Errorf("%v moved the cursor to %d", msg, got)
		}
	}
}

func TestHelpOverlaySwallowsMouseEvents(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	renderAndWait(t, model, rowZone(2))
	model = update(model, key(domain.KeyHelp))

	model = update(model, click(rowTextX, rowY(2)))

	if model.cursor != 0 {
		t.Error("clicks must not reach the list through the help overlay")
	}
}

// Both global actions live in the header bar: they act on the repository, not on
// the panel below them, and the Tree tab has no list header to hang them from.
func TestTheHeaderCarriesBothGlobalActions(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	renderAndWait(t, model, zoneAdd, zoneActions)

	add, actions := model.zones.Get(zoneAdd), model.zones.Get(zoneActions)

	if add.StartY != headerBarY || actions.StartY != headerBarY {
		t.Errorf("buttons on rows %d and %d, want both on the header bar", add.StartY, actions.StartY)
	}
	if add.EndX >= actions.StartX {
		t.Errorf("the call to action ends at %d and Actions starts at %d, want them in that order", add.EndX, actions.StartX)
	}
}

// zonePoint is the first cell of a zone, the one a click lands on.
func zonePoint(model Model, id string) (x, y int) {
	zone := model.zones.Get(id)
	return zone.StartX, zone.StartY
}

func TestClickingTheAddButtonStartsTheSameRunAsTheKey(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	renderAndWait(t, model, zoneAdd)
	x, y := zonePoint(model, zoneAdd)

	clicked, cmd := updateCmd(model, click(x, y))

	if cmd == nil || len(clicked.ops.running) != 1 {
		t.Fatalf("clicking the add button started %+v, want one create run", clicked.ops.running)
	}
	if clicked.cursor != 0 {
		t.Error("the add button must not double as a row click")
	}
}

func TestClickingTheActionsButtonOpensTheGlobalMenu(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	renderAndWait(t, model, zoneActions)
	x, y := zonePoint(model, zoneActions)

	opened := update(model, click(x, y))

	if !opened.menuOpen {
		t.Fatal("the Actions button must open its menu")
	}
	if opened.menuKind != menuForGlobal {
		t.Error("the header button opens the global menu, not the row's")
	}
}

// prModel builds a model with a PR loaded for the (sole) worktree's branch
// and an injected PROpener, so a click on the REVIEW line can be exercised
// without shelling out to a real gh.
func prModel(t *testing.T, opener func(number int) error) Model {
	t.Helper()
	model := New(RunParams{PROpener: opener})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{statuses: statuses("feat/x"), parents: map[string]string{}})
	return update(model, prsMsg{conn: domain.GHConnectionOK, prs: []domain.PRInfo{
		{Branch: "feat/x", Number: 67, Title: "feat: x", State: "OPEN"},
	}})
}

func TestClickingThePRLineOpensItInABrowser(t *testing.T) {
	var calls int
	var gotNumber int
	model := prModel(t, func(number int) error {
		calls++
		gotNumber = number
		return nil
	})
	renderAndWait(t, model, zoneDetailPR)
	x, y := zonePoint(model, zoneDetailPR)

	next, cmd := updateCmd(model, click(x, y))
	if cmd == nil {
		t.Fatal("clicking the PR line must return a command — the launch runs off the UI goroutine")
	}
	update(next, cmd())

	if calls != 1 {
		t.Fatalf("PROpener called %d times, want 1", calls)
	}
	if gotNumber != 67 {
		t.Errorf("PROpener got PR #%d, want #67", gotNumber)
	}
}

func TestOpenPRFailureGoesToOutputPanel(t *testing.T) {
	model := prModel(t, func(int) error { return errors.New("gh: not authenticated") })
	renderAndWait(t, model, zoneDetailPR)
	x, y := zonePoint(model, zoneDetailPR)

	next, cmd := updateCmd(model, click(x, y))
	next = update(next, cmd())

	if !strings.Contains(strings.Join(next.outputLines, "\n"), "not authenticated") {
		t.Error("a failed PR open must say why in the output panel, like every other operation")
	}
}

// TestNoPRZoneWhenThereIsNoPR pins that no zone is registered for a line
// that was never drawn: gh fine, no PR for the branch, no REVIEW section at
// all — so there is nothing to click.
func TestNoPRZoneWhenThereIsNoPR(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "feat/x")
	model = update(model, prsMsg{conn: domain.GHConnectionOK})
	model.View()

	if !model.zones.Get(zoneDetailPR).IsZero() {
		t.Error("no PR line was drawn — no zone should exist for it")
	}
}

// runningDetailModel is a dashboard whose selected worktree has jobs up, with
// the detail panel showing: what a click on a RUN row is asserted against.
func runningDetailModel(t *testing.T, params RunParams, addresses map[string]domain.JobAddress, jobs ...domain.JobConfig) Model {
	t.Helper()
	model := New(params)
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{statuses: statuses("a"), parents: map[string]string{}})
	model.runConfig = domain.RunConfig{Jobs: jobs}
	infos := make([]domain.JobInfo, 0, len(jobs))
	for _, job := range jobs {
		infos = append(infos, domain.JobInfo{Name: job.Name, Status: domain.JobStatusRunning, WorkDir: "/tmp/a"})
	}
	model.jobs = infos
	model.details = map[string]domain.WorktreeDetail{"a": {Branch: "a"}}
	model.addresses = map[string]map[string]domain.JobAddress{"a": addresses}
	return model
}

func TestClickingTheAddressOpensItAndClickingTheRowOpensTheLogs(t *testing.T) {
	opened := make(chan string, 1)
	model := runningDetailModel(t,
		RunParams{
			URLOpener:  func(url string) error { opened <- url; return nil },
			LogsLoader: func(logsRequest) ([]string, error) { return []string{"ready"}, nil },
		},
		map[string]domain.JobAddress{"web": {URL: "http://web.wtm"}},
		domain.JobConfig{Name: "web"},
	)
	renderAndWait(t, model, runRowZone("web"), runURLZone("web"))

	address := model.zones.Get(runURLZone("web"))
	_, cmd := updateCmd(model, click(address.StartX, address.StartY))
	if cmd == nil {
		t.Fatal("clicking the address did nothing")
	}
	cmd()
	select {
	case got := <-opened:
		if got != "http://web.wtm" {
			t.Errorf("opened %q, want http://web.wtm", got)
		}
	default:
		t.Fatal("the address was not opened")
	}

	// Away from the address, the row leads to what else the job has.
	row := model.zones.Get(runRowZone("web"))
	next, _ := updateCmd(model, click(row.EndX, row.StartY))
	if next.logsJob != "web" {
		t.Error("clicking the row away from its address did not open the logs")
	}
}

func TestADownJobRowTakesNoZone(t *testing.T) {
	model := New(RunParams{})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{statuses: statuses("a"), parents: map[string]string{}})
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "worker"}}}
	model.details = map[string]domain.WorktreeDetail{"a": {Branch: "a"}}
	model.addresses = map[string]map[string]domain.JobAddress{"a": {"worker": {URL: "http://worker.wtm"}}}

	renderAndWait(t, model, zoneDetail)

	if !model.zones.Get(runRowZone("worker")).IsZero() {
		t.Error("a stopped job's row is clickable, want no zone: it answers nowhere")
	}
}

// The DETAIL panel has no cursor, so it designates no job: u is silent there,
// exactly as p is silent where no PR is designated.
func TestTheAddressKeyIsSilentOnTheDetailPanel(t *testing.T) {
	model := runningDetailModel(t, RunParams{
		URLOpener: func(string) error { t.Error("u opened an address with no job designated"); return nil },
	}, map[string]domain.JobAddress{"web": {URL: "http://web.wtm"}}, domain.JobConfig{Name: "web"})

	if _, cmd := updateCmd(model, key(domain.KeyOpenAddress)); cmd != nil {
		cmd()
	}
}

// A zone that is marked but whose lookup sits in a function nothing calls is a
// click that lands nowhere, and nothing else catches it. Checking only that a
// lookup exists somewhere is not enough — that is how the logs view's job chips
// shipped marked and inert, their lookup alive inside a helper handleMouse
// never reached.
func TestEveryZoneHelperIsMarkedAndReachableFromTheMouseHandler(t *testing.T) {
	sources := dashboardSources(t)

	helpers := regexp.MustCompile(`func ([a-z][A-Za-z]*Zone)\(`).FindAllStringSubmatch(sources["zones.go"], -1)
	if len(helpers) == 0 {
		t.Fatal("no zone helper found, this test is looking at the wrong file")
	}
	reachable := mouseHandlerCallees(t, sources)

	for _, helper := range helpers {
		name := helper[1]
		marked, handledBy := false, ""
		for path, body := range sources {
			if path == "zones.go" {
				continue
			}
			if strings.Contains(body, "Mark("+name+"(") {
				marked = true
			}
			for _, fn := range functionsMentioning(body, "inZone("+name+"(") {
				handledBy = fn
			}
		}
		if !marked {
			t.Errorf("%s is never marked: nothing draws it", name)
			continue
		}
		if handledBy == "" {
			t.Errorf("%s is marked but never looked up: a click on it lands nowhere", name)
			continue
		}
		if !reachable[handledBy] {
			t.Errorf("%s is looked up in %s, which handleMouse never calls: the zone is inert", name, handledBy)
		}
	}
}

func dashboardSources(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sources := make(map[string]string, len(paths))
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sources[path] = string(body)
	}
	return sources
}

// functionsMentioning names the top-level functions of a file whose body holds
// the needle.
func functionsMentioning(body, needle string) []string {
	var found []string
	decls := regexp.MustCompile(`(?m)^func (?:\([^)]*\) )?([A-Za-z][A-Za-z0-9]*)\(`).FindAllStringSubmatchIndex(body, -1)
	for index, decl := range decls {
		end := len(body)
		if index+1 < len(decls) {
			end = decls[index+1][0]
		}
		if strings.Contains(body[decl[0]:end], needle) {
			found = append(found, body[decl[2]:decl[3]])
		}
	}
	return found
}

// mouseHandlerCallees is handleMouse itself plus every method it calls, one
// level deep — enough to catch a helper that was written and never wired.
// mouseHandlerCallees closes over what handleMouse can reach, not only what it
// calls itself: a lookup one hop further down — the right button's own row
// selection, say — is just as reachable, and a direct-callees-only walk would
// call it inert.
func mouseHandlerCallees(t *testing.T, sources map[string]string) map[string]bool {
	t.Helper()
	reachable := map[string]bool{"handleMouse": true}
	for grew := true; grew; {
		grew = false
		for name := range reachable {
			for _, called := range callsMadeBy(sources, name) {
				if reachable[called] {
					continue
				}
				reachable[called] = true
				grew = true
			}
		}
	}
	if len(reachable) < 2 {
		t.Fatal("handleMouse was not found, this test is checking nothing")
	}
	return reachable
}

func callsMadeBy(sources map[string]string, name string) []string {
	var called []string
	for _, body := range sources {
		decls := regexp.MustCompile(`(?m)^func \([^)]*\) `+name+`\(`).FindAllStringSubmatchIndex(body, -1)
		for _, decl := range decls {
			end := len(body)
			if next := strings.Index(body[decl[1]:], "\n}\n"); next >= 0 {
				end = decl[1] + next
			}
			for _, call := range regexp.MustCompile(`m\.([a-z][A-Za-z0-9]*)\(`).FindAllStringSubmatch(body[decl[0]:end], -1) {
				called = append(called, call[1])
			}
		}
	}
	return called
}

// A runner's addresses are folded until asked for, and unfolding one gives each
// app it answers for a row that can be clicked — which the joined cell they used
// to share never was, having no url of its own to carry.
func TestUnfoldingARunnerGivesEachAddressItsOwnClickableRow(t *testing.T) {
	opened := make(chan string, 1)
	model := runningDetailModel(t,
		RunParams{URLOpener: func(url string) error { opened <- url; return nil }},
		map[string]domain.JobAddress{"dev": {Held: []domain.JobURLEntry{
			{Job: "web", URL: "http://web.wtm"},
			{Job: "api", URL: "http://api.wtm"},
		}}},
		domain.JobConfig{Name: "dev", Runs: []string{"web", "api"}},
	)
	renderAndWait(t, model, runFoldZone("dev"))

	child := rules.HeldRowKey("dev", "web")
	if !model.zones.Get(runURLZone(child)).IsZero() {
		t.Fatal("a runner's children were on screen before anything unfolded them")
	}

	fold := model.zones.Get(runFoldZone("dev"))
	model, _ = updateCmd(model, click(fold.EndX, fold.StartY))
	if !model.runExpanded["dev"] {
		t.Fatal("clicking the fold marker did not open the runner")
	}
	renderAndWait(t, model, runURLZone(child))

	address := model.zones.Get(runURLZone(child))
	if address.IsZero() {
		t.Fatal("an unfolded child has no address zone of its own")
	}
	_, cmd := updateCmd(model, click(address.StartX, address.StartY))
	if cmd == nil {
		t.Fatal("clicking a child's address did nothing")
	}
	cmd()
	select {
	case got := <-opened:
		if got != "http://web.wtm" {
			t.Errorf("opened %q, want the child's own url", got)
		}
	default:
		t.Fatal("the child's address was not opened")
	}
}
