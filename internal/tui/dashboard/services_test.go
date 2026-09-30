package dashboard

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func servicesModel(t *testing.T, running ...string) Model {
	t.Helper()
	model := New(RunParams{})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	known := append([]string{"idle"}, running...)
	model = update(model, worktreesMsg{statuses: statuses(known...), parents: map[string]string{}})
	model.tab = tabServices
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "worker"}}}

	now := time.Now()
	addresses := map[string]map[string]domain.JobAddress{}
	for _, branch := range running {
		model.jobs = append(model.jobs, domain.JobInfo{
			Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/" + branch, StartedAt: now.Add(-time.Hour),
		})
		addresses[branch] = map[string]domain.JobAddress{"web": {URL: "http://web." + branch + ".wtm"}}
	}
	model.addresses = addresses
	return model.withBoard()
}

func TestServicesTabListsOneBlockPerWorktreeThatRuns(t *testing.T) {
	model := servicesModel(t, "a")

	body := stripANSI(strings.Join(model.servicesBody(model.layout()), "\n"))

	if !strings.Contains(body, "a") {
		t.Errorf("body = %q, want the running worktree named", body)
	}
	if !strings.Contains(body, "http://web.a.wtm") {
		t.Errorf("body = %q, want the url visible — that is what this tab is for", body)
	}
	if strings.Contains(body, "idle") {
		t.Errorf("body = %q, want an idle worktree left out", body)
	}
	if strings.Contains(body, "worker") {
		t.Errorf("body = %q, want a stopped job left out: this tab answers what runs", body)
	}
}

func TestServicesTabTakesTheWholeWidth(t *testing.T) {
	model := servicesModel(t, "a")

	layout := model.layout()

	if layout.DetailVisible {
		t.Error("the detail shows beside the Running tab, want the tab alone")
	}
	if layout.List.Width != testWidth {
		t.Errorf("List.Width = %d, want the full %d", layout.List.Width, testWidth)
	}
}

func TestServicesTabCursorSelectsTheWorktreeOfItsJob(t *testing.T) {
	model := servicesModel(t, "a", "b")

	model = model.stepServices(1)
	selected, ok := model.selectedService()

	if !ok || selected.Branch != "b" {
		t.Errorf("selected = %+v, want b: the cursor walks jobs, and b's is the next one", selected)
	}
}

func TestServicesTabArrowsWalkTheJobs(t *testing.T) {
	model := servicesModel(t, "a", "b")

	model = model.moveCursor(1)
	second, ok := model.selectedService()
	if !ok || second.Branch != "b" {
		t.Errorf("selected = %+v, want the arrow to have reached b's job", second)
	}

	model = model.moveCursor(5)
	last, ok := model.selectedService()
	if !ok || last.Branch != "b" {
		t.Errorf("selected = %+v, want it clamped to the last job", last)
	}
}

func TestServicesTabMenuOffersRunOnly(t *testing.T) {
	model := servicesModel(t, "a")
	model.menuKind = menuForWorktree

	items := model.menuItems()

	if len(items) == 0 {
		t.Fatal("no entries at all")
	}
	for _, item := range items {
		if item.label == domain.DashboardMenuSectionGit {
			t.Error("the Running tab offers GIT, want run actions only: this tab does not speak of git")
		}
		if item.danger {
			t.Errorf("entry %q is destructive, want none here", item.label)
		}
	}
	if items[0].kind == menuEntrySeparator {
		t.Error("the block opens with a rule, want nothing above the first heading to separate")
	}
}

func TestServicesTabActionsTargetTheWorktreeOfTheJobUnderTheCursor(t *testing.T) {
	model := servicesModel(t, "a", "b").stepServices(1)

	target, ok := model.selected()

	if !ok || target.Branch != "b" {
		t.Errorf("selected = %+v, want b: the cursor's job is what actions act on", target)
	}
	if target.Path != "/tmp/b" {
		t.Errorf("Path = %q, want the worktree's own, not the row's copy", target.Path)
	}
}

func TestServicesTabCountsWhatIsUp(t *testing.T) {
	model := servicesModel(t, "a", "b")

	if got := model.countText(); !strings.Contains(got, "2") {
		t.Errorf("countText = %q, want the two running jobs counted", got)
	}
}

// The addresses are what this tab is read down: a name column restarting at
// each worktree would break that read.
func TestServicesTabAlignsAddressesAcrossBlocks(t *testing.T) {
	model := servicesModel(t, "a", "b")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "database"}}}
	model.jobs = append(model.jobs, domain.JobInfo{
		Name: "database", Status: domain.JobStatusRunning, WorkDir: "/tmp/b", StartedAt: time.Now(),
	})
	model.addresses["b"]["database"] = domain.JobAddress{Ports: []int{5432}}
	model = model.withBoard()

	lines := model.servicesBody(model.layout())
	columns := make([]int, 0, 3)
	for _, line := range lines {
		plain := stripANSI(line)
		for _, address := range []string{"http://", ":5432"} {
			index := strings.Index(plain, address)
			if index < 0 {
				continue
			}
			// Measured in columns, not in bytes: the cursor's accent bar and the
			// state glyphs are three bytes each, so a byte offset reads as a
			// misalignment that is not on screen.
			columns = append(columns, lipgloss.Width(plain[:index]))
			break
		}
	}

	if len(columns) < 3 {
		t.Fatalf("found %d address cells, want one per running job", len(columns))
	}
	for _, column := range columns[1:] {
		if column != columns[0] {
			t.Errorf("addresses start at %v, want one column across every block", columns)
			break
		}
	}
}

// The board was rebuilt by countText, servicesBody, selected, rowCount, clickRow
// and selectedServiceBlock — six times a frame, each with its own time.Now().
func TestTheBoardIsBuiltByThePollNotByTheRenderer(t *testing.T) {
	model := servicesModel(t, "a")

	before := model.board
	_ = model.servicesBody(model.layout())
	_, _ = model.selected()
	_ = model.countText()

	if len(before) == 0 {
		t.Fatal("board is empty, want it built when the jobs landed")
	}
	if &model.board[0] != &before[0] {
		t.Error("the renderer rebuilt the board, want it read off the model")
	}
}

func TestServicesCursorWalksJobsAndSkipsHeaders(t *testing.T) {
	model := servicesModel(t, "a", "b")

	first, ok := model.selectedService()
	if !ok || first.Kind != domain.ServicesRowJob {
		t.Fatalf("selected = %+v, want a job row: a header is not a subject", first)
	}

	model = model.stepServices(1)
	second, ok := model.selectedService()
	if !ok || second.Kind != domain.ServicesRowJob {
		t.Fatalf("selected = %+v, want the header skipped", second)
	}
	if second.Branch == first.Branch {
		t.Errorf("branch = %q then %q, want the cursor to have crossed into the next block", first.Branch, second.Branch)
	}
}

func TestServicesScrollsSoTheCursorStaysVisible(t *testing.T) {
	branches := make([]string, 0, 12)
	for index := range 12 {
		branches = append(branches, fmt.Sprintf("wt%02d", index))
	}
	model := servicesModel(t, branches...)

	for range 11 {
		model = model.stepServices(1)
	}
	model = model.reflow()

	found := false
	for _, row := range model.servicesVisible(model.layout()) {
		if row.Kind == domain.ServicesRowJob && row.Branch == "wt11" {
			found = true
		}
	}
	if !found {
		t.Error("the cursor scrolled out of the drawn window: the tab has no offset")
	}
}

func TestServicesCursorIsReboundWhenAJobStops(t *testing.T) {
	model := servicesModel(t, "a", "b", "c")
	model = model.stepServices(2)

	model.jobs = model.jobs[:1]
	model, _ = model.applyJobs(jobsMsg{
		jobs: model.jobs, running: rules.RunningJobsByWorktree(model.jobs),
		config: model.runConfig, known: true,
	})

	if _, ok := model.selectedService(); !ok {
		t.Error("nothing is selected after the board shrank: m opens no menu until an arrow is pressed")
	}
}

// Enter goes straight to the full run view. The tab used to turn its own body
// into a smaller logs view first, which meant two keystrokes and a second stop
// on the way to the same place.
func TestEnterOnAServiceOpensTheFullLogsView(t *testing.T) {
	model := servicesModel(t, "a")
	model.params.LogsLoader = func(logsRequest) ([]string, error) { return []string{"ready"}, nil }

	next, _ := updateCmd(model, namedKey(tea.KeyEnter))

	// The run view is opened through watchLogs, which needs a run.toml on disk
	// this fixture does not have; what enter must do here is name the job and
	// leave the panel alone, never turn the tab into a second logs view.
	if next.logsJob != "web" || next.logsBranch != "a" {
		t.Errorf("logs = %q/%q, want the job under the cursor", next.logsBranch, next.logsJob)
	}
	if next.panelTab == panelLogs {
		t.Error("the right-hand panel was armed from Services, where it is not drawn")
	}

	body := stripANSI(strings.Join(next.servicesBody(next.layout()), "\n"))
	if !strings.Contains(body, "web") {
		t.Errorf("body = %q, want the tab still showing its overview", body)
	}
}

func TestTheEmptyServicesTabNamesEveryWayToStartSomething(t *testing.T) {
	model := servicesModel(t)

	body := stripANSI(strings.Join(model.servicesBody(model.layout()), "\n"))

	if !strings.Contains(body, domain.DashboardServicesEmpty) {
		t.Errorf("body = %q, want it to say nothing runs", body)
	}
	for _, row := range domain.DashboardServicesEmptyRows {
		if !strings.Contains(body, row[0]) || !strings.Contains(body, row[1]) {
			t.Errorf("body = %q, misses the route %q — %q", body, row[0], row[1])
		}
	}
	if !strings.Contains(body, "--job") {
		t.Error("the empty state never mentions starting a single job")
	}
}

func TestTheAddressKeyOpensTheAddressOfTheJobUnderTheCursor(t *testing.T) {
	opened := make(chan string, 1)
	model := servicesModel(t, "a")
	model.params.URLOpener = func(url string) error { opened <- url; return nil }

	_, cmd := updateCmd(model, key(domain.KeyOpenAddress))
	if cmd == nil {
		t.Fatal("u did nothing")
	}
	cmd()

	select {
	case got := <-opened:
		if got != "http://web.a.wtm" {
			t.Errorf("opened %q, want the address of the job under the cursor", got)
		}
	default:
		t.Fatal("nothing opened")
	}
}

func TestClickingAServiceAddressOpensIt(t *testing.T) {
	opened := make(chan string, 1)
	model := servicesModel(t, "a")
	model.params.URLOpener = func(url string) error { opened <- url; return nil }
	renderAndWait(t, model, servicesURLZone(1))

	zone := model.zones.Get(servicesURLZone(1))
	_, cmd := updateCmd(model, click(zone.StartX, zone.StartY))
	if cmd == nil {
		t.Fatal("clicking a Services address did nothing: every url there used to be dead")
	}
	cmd()

	select {
	case got := <-opened:
		if got != "http://web.a.wtm" {
			t.Errorf("opened %q, want the row's own address", got)
		}
	default:
		t.Fatal("nothing opened")
	}
}

// Arming the right-hand panel from a tab that does not draw it left an
// invisible view holding esc, enter and the arrows.
func TestTheLogsKeyOnServicesOpensTheFullView(t *testing.T) {
	model := servicesModel(t, "a")
	model.params.LogsLoader = func(logsRequest) ([]string, error) { return []string{"ready"}, nil }

	next, _ := updateCmd(model, key(domain.KeyRunLogs))

	if next.panelTab == panelLogs {
		t.Error("the right-hand panel was armed from Services, where it is not drawn")
	}
	if next.logsJob != "web" {
		t.Errorf("logsJob = %q, want the job under the cursor named for the full view", next.logsJob)
	}
}

// panelBodyHeight is what renderPanel leaves for Body under a titled panel: its
// border, the title row and the blank row under it.
func panelBodyHeight(rect domain.Rect) int {
	return max(rect.Height-borderWidth-2, 0)
}

func TestServicesRowsFillTheWholePanel(t *testing.T) {
	branches := make([]string, 0, 12)
	for index := range 12 {
		branches = append(branches, fmt.Sprintf("wt%02d", index))
	}
	model := servicesModel(t, branches...)

	layout := model.layout()
	drawable := panelBodyHeight(layout.List)

	if layout.ServicesRows != drawable {
		t.Errorf("ServicesRows = %d, want %d: a Services row is one line, unlike a tree node", layout.ServicesRows, drawable)
	}
}

// The cursor was re-seated when the board shrank; the offset was not, and
// servicesVisible past the end drew nothing at all.
func TestServicesOffsetIsReboundWhenTheBoardShrinks(t *testing.T) {
	branches := make([]string, 0, 12)
	for index := range 12 {
		branches = append(branches, fmt.Sprintf("wt%02d", index))
	}
	model := servicesModel(t, branches...)
	for range 11 {
		model = model.stepServices(1)
	}
	model = model.reflow()
	if model.servicesOffset == 0 {
		t.Fatal("the list never scrolled, so this test proves nothing")
	}

	model.jobs = model.jobs[:1]
	model, _ = model.applyJobs(jobsMsg{
		jobs: model.jobs, running: rules.RunningJobsByWorktree(model.jobs),
		config: model.runConfig, known: true,
	})

	if len(model.servicesVisible(model.layout())) == 0 {
		t.Error("the tab draws nothing after the board shrank, want the window re-seated")
	}
}

// Opening a log is a full change of view and the rows sit close together, so a
// click only moves the cursor: enter opens. Only the address cell still acts on
// a click, since pointing at it says exactly what it does.
func TestClickingAServiceRowAwayFromItsAddressOnlyMovesTheCursor(t *testing.T) {
	model := servicesModel(t, "a", "b")
	model.params.LogsLoader = func(logsRequest) ([]string, error) { return []string{"ready"}, nil }
	renderAndWait(t, model, servicesRowZone(4))

	zone := model.zones.Get(servicesRowZone(4))
	next, _ := updateCmd(model, click(zone.EndX, zone.StartY))

	if next.panelTab == panelLogs {
		t.Error("a click opened the logs view; only enter may")
	}
	if next.servicesCursor != 4 {
		t.Errorf("servicesCursor = %d, want the clicked row 4", next.servicesCursor)
	}
}

// The right button never reached these rows: clickRow leaves them to
// clickServiceRow, and the menu had no path from the mouse at all.
func TestRightClickOnAServiceRowOpensTheMenuOnIt(t *testing.T) {
	model := servicesModel(t, "a", "b")
	renderAndWait(t, model, zoneServices)

	x, y, index := serviceJobRow(t, model, "b")
	next, _ := updateCmd(model, rightClick(x, y))

	if !next.menuOpen {
		t.Fatal("the right button opened no menu on the Services tab")
	}
	if next.servicesCursor != index {
		t.Errorf("cursor = %d, want the row under the pointer (%d): a menu acting elsewhere is a trap", next.servicesCursor, index)
	}
	selected, ok := next.selected()
	if !ok || selected.Branch != "b" {
		t.Errorf("subject = %+v, want the worktree of the job clicked", selected)
	}
}

// The cursor is on a job, so the stop names that one. Reopening a picker over a
// row that already designates a job is the question a designated subject exists
// to not ask.
func TestTheServicesMenuStopsTheJobUnderTheCursor(t *testing.T) {
	model := servicesModel(t, "a")

	items := model.menuItems()
	index := menuIndexOf(t, model, menuRunStop)
	if items[index].label != domain.DashboardMenuRunStopThis {
		t.Errorf("label = %q, want %q", items[index].label, domain.DashboardMenuRunStopThis)
	}
	if got := model.designatedJob(); got != "web" {
		t.Errorf("designated job = %q, want the one under the cursor", got)
	}
}

// Elsewhere no job is designated, so the picker is what names it.
func TestNoJobIsDesignatedOffTheServicesTab(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	if got := model.designatedJob(); got != "" {
		t.Errorf("designated job = %q, want none: the list designates a worktree, not a job", got)
	}
}

func serviceJobRow(t *testing.T, model Model, branch string) (x, y, index int) {
	t.Helper()
	for index, row := range model.services {
		if row.Kind != domain.ServicesRowJob || row.Branch != branch {
			continue
		}
		zone := model.zones.Get(servicesRowZone(index))
		if zone.IsZero() {
			t.Fatalf("the row of %q was not drawn", branch)
		}
		return zone.StartX + 1, zone.StartY, index
	}
	t.Fatalf("no job row for %q", branch)
	return 0, 0, 0
}
