package dashboard

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

func logsModel(t *testing.T, params RunParams, branches ...string) Model {
	t.Helper()
	model := New(params)
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	return update(model, worktreesMsg{statuses: statuses(branches...), parents: map[string]string{}})
}

func TestOpeningTheLogsPanelTailsTheJob(t *testing.T) {
	model := logsModel(t, RunParams{
		LogsLoader: func(req logsRequest) ([]string, error) {
			if req.Job != "web" || req.WorkDir != "/tmp/a" {
				t.Errorf("request = %+v, want web in /tmp/a", req)
			}
			if req.Lines != domain.DashboardLogsLines {
				t.Errorf("lines = %d, want %d", req.Lines, domain.DashboardLogsLines)
			}
			return []string{"ready in 380ms"}, nil
		},
	}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})

	model, cmd := model.openLogsTabOn("web")
	if model.logsJob != "web" || model.logsBranch != "a" {
		t.Fatalf("panel = %q/%q, want web on a", model.logsBranch, model.logsJob)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want the tail read off the UI goroutine")
	}

	msg, ok := cmd().(logsTailMsg)
	if !ok {
		t.Fatalf("msg = %T, want logsTailMsg", cmd())
	}
	model = model.applyLogsTail(msg)
	if len(model.logsLines) != 1 || model.logsLines[0] != "ready in 380ms" {
		t.Errorf("lines = %v, want the tail", model.logsLines)
	}
}

func TestASupersededTailNeverLandsOnTheOpenPanel(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.logsBranch, model.logsJob = "a", "web"
	model.logsLines = []string{"kept"}

	model = model.applyLogsTail(logsTailMsg{branch: "a", job: "api", lines: []string{"other job"}})

	if len(model.logsLines) != 1 || model.logsLines[0] != "kept" {
		t.Errorf("lines = %v, want the open panel's own tail untouched", model.logsLines)
	}
}

func TestSelectingAnotherWorktreeClosesTheLogsPanel(t *testing.T) {
	model := logsModel(t, RunParams{}, "a", "b")
	model.logsBranch, model.logsJob = "a", "web"

	next, _ := updateCmd(model, namedKey(tea.KeyDown))

	if next.logsOpen() {
		t.Error("the panel survived the move, want it closed: it speaks of one worktree's job")
	}
}

func TestLogsPanelHeadsWithTheJobAndKeepsTheLastLines(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})
	model.jobs = []domain.JobInfo{{
		Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a",
		StartedAt: time.Now().Add(-72 * time.Minute),
	}}
	model.addresses = map[string]map[string]domain.JobAddress{"a": {"web": {URL: "http://web.wtm"}}}
	model.logsBranch, model.logsJob = "a", "web"
	model.logsLines = []string{"line 1", "line 2", "line 3"}

	body := stripANSI(strings.Join(model.logsBody(model.layout()), "\n"))

	if !strings.Contains(body, "web") {
		t.Errorf("body = %q, want the job named", body)
	}
	if !strings.Contains(body, "http://web.wtm") {
		t.Errorf("body = %q, want the address on the header", body)
	}
	if !strings.Contains(body, "line 3") {
		t.Errorf("body = %q, want the newest line kept", body)
	}
	if !strings.Contains(body, domain.DashboardLogsHint) {
		t.Errorf("body = %q, want the way out named", body)
	}
}

func TestLogsPanelDropsTheOldestLinesWhenTheBudgetIsShort(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"
	for index := range 200 {
		model.logsLines = append(model.logsLines, "line "+string(rune('a'+index%26))+string(rune('0'+index/26)))
	}
	model.logsLines[len(model.logsLines)-1] = "newest"
	model.logsLines[0] = "oldest"

	body := stripANSI(strings.Join(model.logsBody(model.layout()), "\n"))

	if !strings.Contains(body, "newest") {
		t.Error("the newest line was dropped, want the oldest ones to go first")
	}
	if strings.Contains(body, "oldest") {
		t.Error("the oldest line survived a short budget, want it dropped")
	}
}

func TestLogsPanelSaysWhyItCouldNotRead(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"
	model.logsErr = errors.New("no log for web")

	body := stripANSI(strings.Join(model.logsBody(model.layout()), "\n"))

	if !strings.Contains(body, "no log for web") {
		t.Errorf("body = %q, want the failure named rather than an empty panel", body)
	}
}

func TestTheDetailPanelYieldsToTheLogsPanelWhileItIsOpen(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.detailOpen = true
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"
	model.logsLines = []string{"tailed"}

	body := stripANSI(strings.Join(model.detailBody(model.layout()), "\n"))

	if strings.Contains(body, domain.DetailSectionLinks) {
		t.Errorf("body = %q, want the sections replaced by the tail", body)
	}
	if !strings.Contains(body, "tailed") {
		t.Errorf("body = %q, want the tail shown", body)
	}
}

func TestEscapeLeavesTheLogsPanelForTheDetail(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"

	next, _ := updateCmd(model, namedKey(tea.KeyEscape))

	if next.logsOpen() {
		t.Error("esc left the panel open")
	}
}

func TestClickingAJobWithNoURLOpensItsLogs(t *testing.T) {
	model := logsModel(t, RunParams{
		LogsLoader: func(logsRequest) ([]string, error) { return []string{"ready"}, nil },
	}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "pg"}})
	model.jobs = []domain.JobInfo{{Name: "pg", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}}
	model.addresses = map[string]map[string]domain.JobAddress{"a": {"pg": {Ports: []int{5432}}}}
	renderAndWait(t, model, runRowZone("pg"))

	zone := model.zones.Get(runRowZone("pg"))
	next, _ := updateCmd(model, click(zone.StartX+3, zone.StartY))

	if next.logsJob != "pg" {
		t.Errorf("logsJob = %q, want the click to lead to what the job does have", next.logsJob)
	}
}

func TestTheJobColumnNamesEveryDeclaredJobAndMarksWhatIsUp(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}, {Name: "api"}, {Name: "pg"}})
	model.jobs = []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}}
	model.addresses = map[string]map[string]domain.JobAddress{"a": {"web": {URL: "http://web.wtm"}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"

	column := stripANSI(strings.Join(model.logsJobColumn(logsJobColumnParams{Width: 20, Rows: 10}), "\n"))

	for _, name := range []string{"web", "api", "pg"} {
		if !strings.Contains(column, name) {
			t.Errorf("column = %q, misses %q: a stopped job's tail is still readable", column, name)
		}
	}
	if !strings.Contains(column, domain.DetailJobUpGlyph) || !strings.Contains(column, domain.DetailJobRanGlyph) {
		t.Errorf("column = %q, want what is up told apart from what merely ran", column)
	}
	if address := stripANSI(strings.Join(model.logsAddressLines(60), "\n")); !strings.Contains(address, "http://web.wtm") {
		t.Errorf("address line = %q, want the current job's address", address)
	}
}

// Ten jobs used to be windowed into a row of chips behind ‹ › marks; the column
// lists them down the side and every one of them stays reachable.
func TestTheJobColumnHoldsEveryJobOfALongList(t *testing.T) {
	jobs := make([]domain.JobConfig, 0, 10)
	for i := range 10 {
		jobs = append(jobs, domain.JobConfig{Name: fmt.Sprintf("service-%02d", i)})
	}
	model := logsModel(t, RunParams{}, "a")
	model = declaringRunJobs(model, jobs)
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "service-09"

	column := stripANSI(strings.Join(model.logsJobColumn(logsJobColumnParams{Width: 20, Rows: 10}), "\n"))

	if !strings.Contains(column, "service-09") {
		t.Errorf("column = %q, want the selected job visible", column)
	}
	for _, mark := range []string{"‹", "›"} {
		if strings.Contains(column, mark) {
			t.Errorf("column = %q, want no windowing marks left", column)
		}
	}
}

func TestSteppingTheJobStaysInsideItsWorktree(t *testing.T) {
	model := logsModel(t, RunParams{}, "a", "b")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}, {Name: "api"}})
	model.logsBranch, model.logsJob = "a", "web"

	model = model.stepLogsJob(1)
	if model.logsJob != "api" || model.logsBranch != "a" {
		t.Errorf("job = %q on %q, want api on a", model.logsJob, model.logsBranch)
	}

	model = model.stepLogsJob(1)
	if model.logsJob != "api" {
		t.Errorf("job = %q, want it clamped at the last one", model.logsJob)
	}

	model = model.stepLogsJob(-5)
	if model.logsJob != "web" {
		t.Errorf("job = %q, want it clamped at the first one", model.logsJob)
	}
}

func TestSteppingTheJobRetailsIt(t *testing.T) {
	asked := make(chan string, 2)
	model := logsModel(t, RunParams{
		LogsLoader: func(req logsRequest) ([]string, error) { asked <- req.Job; return nil, nil },
	}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}, {Name: "api"}})
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"

	model = model.stepLogsJob(1)
	if cmd := model.tailLogsCmd(); cmd != nil {
		cmd()
	}

	select {
	case job := <-asked:
		if job != "api" {
			t.Errorf("tailed %q, want api", job)
		}
	default:
		t.Fatal("nothing was tailed after the job changed")
	}
}

// The logs view is hosted by the Services tab too, where the selected worktree
// and the tailed one part company.
func TestTheLogsAddressIsReadOffTheTailedBranch(t *testing.T) {
	model := logsModel(t, RunParams{}, "a", "b")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})
	model.addresses = map[string]map[string]domain.JobAddress{
		"a": {"web": {URL: "http://web.a.wtm"}},
		"b": {"web": {URL: "http://web.b.wtm"}},
	}
	model.logsBranch, model.logsJob = "b", "web"

	if got := model.logsAddress().URL; got != "http://web.b.wtm" {
		t.Errorf("address = %q, want the tailed worktree's own, not the selected one's", got)
	}
}

func TestTheLogsKeyOpensTheTabWithoutAPicker(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "worker"}, {Name: "web"}})
	model.jobs = []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}}

	next, _ := updateCmd(model, key(domain.KeyRunLogs))

	if next.modal.open {
		t.Error("a modal opened, want the selection line to replace the picker")
	}
	if next.panelTab != panelLogs {
		t.Error("the panel stayed on DETAIL")
	}
	if next.logsJob != "web" {
		t.Errorf("logsJob = %q, want the first job that is up", next.logsJob)
	}
}

// The tab always opens: a greyed-out tab left the reader guessing, and a
// project with no run module has something to be told rather than a door that
// does not answer.
func TestTheLogsTabOpensWithoutARunModuleAndSaysWhatIsMissing(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")

	next, _ := updateCmd(model, key(domain.KeyRunLogs))

	if next.panelTab != panelLogs {
		t.Fatal("the LOGS tab did not open")
	}
	body := stripANSI(strings.Join(next.detailBody(next.layout()), "\n"))
	if !strings.Contains(body, domain.DashboardLogsNoModule) {
		t.Errorf("body = %q, want it to say the project runs nothing", body)
	}
	if !strings.Contains(body, domain.DashboardLogsNoModuleHint) {
		t.Errorf("body = %q, want `wtm run init` named", body)
	}
}

// Four different silences, four different answers. "Never ran" used to be
// inferred from the job not being up, which said it of a job that had run, been
// stopped and written nothing; the log on disk now answers instead.
func TestTheLogsViewTellsTheKindsOfSilenceApart(t *testing.T) {
	body := func(model Model) string {
		return stripANSI(strings.Join(model.logsViewBody(logsViewParams{Width: 60, Height: 20}), "\n"))
	}

	// A project that declares nothing at all.
	bare := logsModel(t, RunParams{}, "a")
	bare.panelTab, bare.logsBranch, bare.logsJob = panelLogs, "a", ""
	if got := body(bare); !strings.Contains(got, domain.DashboardLogsNoModule) {
		t.Errorf("body = %q, want a project with no run module pointed at `run init`", got)
	}

	// Declared, but nothing has ever been started in this worktree.
	idle := logsModel(t, RunParams{}, "a")
	idle.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	idle.panelTab, idle.logsBranch, idle.logsJob = panelLogs, "a", "web"
	if got := body(idle); !strings.Contains(got, domain.DashboardLogsNothingRan) {
		t.Errorf("body = %q, want a worktree that has started nothing told so", got)
	}

	// It ran here and wrote nothing: its log exists and is empty.
	silent := logsModel(t, RunParams{}, "a")
	silent = declaringRunJobs(silent, []domain.JobConfig{{Name: "web"}})
	silent.panelTab, silent.logsBranch, silent.logsJob = panelLogs, "a", "web"
	if got := body(silent); !strings.Contains(got, domain.DashboardLogsSilent) {
		t.Errorf("body = %q, want a run that produced no output told apart", got)
	}

	// It is up and has not written yet.
	quiet := silent
	quiet.jobs = []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}}
	if got := body(quiet); !strings.Contains(got, domain.DashboardLogsQuiet) {
		t.Errorf("body = %q, want a running job that wrote nothing told apart", got)
	}
}

func TestArrowsWalkTheJobsWhileTheLogsTabIsUp(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}, {Name: "api"}})
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"

	next, _ := updateCmd(model, namedKey(tea.KeyRight))

	if next.logsJob != "api" {
		t.Errorf("logsJob = %q, want the right arrow to walk the selection line", next.logsJob)
	}
	if next.cursor != 0 {
		t.Errorf("cursor = %d, want the list left alone while the logs tab has the keys", next.cursor)
	}
}

func TestEnterHandsRunviewTheJobOnScreen(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}, {Name: "api"}})
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "api"

	request := model.watchLogsRequest()

	if request.Job != "api" {
		t.Errorf("Job = %q, want runview opened on the job on screen, not on the whole worktree", request.Job)
	}
	if len(request.Worktrees) != 1 || request.Worktrees[0] != "a" {
		t.Errorf("Worktrees = %v, want a", request.Worktrees)
	}
}

func TestChangingTabClosesTheLogsView(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"

	next, _ := updateCmd(model, key(keyTab))

	if next.panelTab != panelDetail {
		t.Error("the logs view survived the tab change and kept esc and enter, while not being drawn")
	}
}

// The chips were marked but nothing ever looked them up: a zone with no
// handler is a click that lands nowhere.
func TestClickingAJobChipSwitchesToIt(t *testing.T) {
	tailed := make(chan string, 2)
	model := logsModel(t, RunParams{
		LogsLoader: func(req logsRequest) ([]string, error) { tailed <- req.Job; return nil, nil },
	}, "a")
	model.detailOpen = true
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}, {Name: "api"}})
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"
	renderAndWait(t, model, logsJobZone("api"))

	zone := model.zones.Get(logsJobZone("api"))
	next, cmd := updateCmd(model, click(zone.StartX, zone.StartY))

	if next.logsJob != "api" {
		t.Fatalf("logsJob = %q, want the chip that was clicked", next.logsJob)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want the new job tailed")
	}
	cmd()
	select {
	case job := <-tailed:
		if job != "api" {
			t.Errorf("tailed %q, want api", job)
		}
	default:
		t.Fatal("nothing was tailed")
	}
}

func TestClickingTheChipAlreadyShownChangesNothing(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.detailOpen = true
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}, {Name: "api"}})
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"
	model.logsLines = []string{"kept"}
	renderAndWait(t, model, logsJobZone("web"))

	zone := model.zones.Get(logsJobZone("web"))
	next, _ := updateCmd(model, click(zone.StartX, zone.StartY))

	if len(next.logsLines) != 1 {
		t.Errorf("lines = %v, want the tail on screen left alone", next.logsLines)
	}
}

func TestClickingTheAddressInTheLogsViewOpensIt(t *testing.T) {
	opened := make(chan string, 1)
	model := logsModel(t, RunParams{
		URLOpener: func(url string) error { opened <- url; return nil },
	}, "a")
	model.detailOpen = true
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})
	model.addresses = map[string]map[string]domain.JobAddress{"a": {"web": {URL: "http://web.wtm"}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "web"
	renderAndWait(t, model, logsURLZone())

	zone := model.zones.Get(logsURLZone())
	_, cmd := updateCmd(model, click(zone.StartX, zone.StartY))
	if cmd == nil {
		t.Fatal("clicking the address in the logs view did nothing")
	}
	cmd()

	select {
	case got := <-opened:
		if got != "http://web.wtm" {
			t.Errorf("opened %q, want http://web.wtm", got)
		}
	default:
		t.Fatal("nothing opened")
	}
}

// A job that publishes no address has none to mark: a zone over empty text
// would swallow clicks meant for the chips beside it.
func TestAJobWithoutAnAddressMarksNoZoneInTheLogsView(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.detailOpen = true
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "pg"}})
	model.addresses = map[string]map[string]domain.JobAddress{"a": {"pg": {Ports: []int{5432}}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "pg"
	renderAndWait(t, model, zoneDetail)

	if !model.zones.Get(logsURLZone()).IsZero() {
		t.Error("an address zone was marked for a job that publishes none")
	}
}

// The panel shows the run view itself, at a panel's size: the same renderer and
// the same live stream `run logs` uses, rather than a second way of drawing
// logs. Enter opens that very view full-screen — the miniature is a preview,
// and it reads no key.
func TestTheLogsPanelHostsTheRunView(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{{Name: "web", Kind: domain.JobKindService, Status: domain.JobStatusStopped}},
		Lines: map[string][]string{"web": {"ready in 380ms"}},
	})
	model := logsModel(t, RunParams{
		BoardLoader: func(logsRequest) runlogs.Board { return board },
		LogsLoader:  func(logsRequest) ([]string, error) { return nil, nil },
	}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})

	model, _ = model.openLogsTabOn("web")
	if !model.previewOn {
		t.Fatal("the panel opened without a preview although a board was available")
	}

	body := stripANSI(strings.Join(model.logsBody(model.layout()), "\n"))
	if !strings.Contains(body, "web") {
		t.Errorf("panel does not name the job it is previewing:\n%s", body)
	}

	// Leaving releases the stream: a panel closed without it leaks one
	// subscription per job it ever showed.
	model = model.closePanelLogs()
	if model.previewOn {
		t.Error("the preview outlived the panel that held it")
	}
}

// Without a board — a test, a surface with no daemon — the panel keeps the
// persisted tail it always had.
func TestTheLogsPanelFallsBackToTheTailWithoutABoard(t *testing.T) {
	model := logsModel(t, RunParams{
		LogsLoader: func(logsRequest) ([]string, error) { return []string{"ready in 380ms"}, nil },
	}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})

	model, cmd := model.openLogsTabOn("web")
	if model.previewOn {
		t.Fatal("a preview was opened with no board to attach through")
	}
	tail, ok := cmd().(logsTailMsg)
	if !ok {
		t.Fatalf("msg = %T, want logsTailMsg", cmd())
	}
	model = model.applyLogsTail(tail)

	body := stripANSI(strings.Join(model.logsBody(model.layout()), "\n"))
	if !strings.Contains(body, "ready in 380ms") {
		t.Errorf("panel lost its tail:\n%s", body)
	}
}

// The panel's body must not move as the reader walks the jobs: the address
// keeps its row whether or not the job publishes one.
func TestTheLogsPanelKeepsItsHeightWithAndWithoutAnAddress(t *testing.T) {
	model := logsModel(t, RunParams{
		LogsLoader: func(logsRequest) ([]string, error) { return []string{"ready"}, nil },
	}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}, {Name: "worker"}})
	model.addresses = map[string]map[string]domain.JobAddress{"a": {"web": {URL: "http://web.wtm"}}}

	published, _ := model.openLogsTabOn("web")
	bare, _ := model.openLogsTabOn("worker")

	withURL := strings.Split(strings.Join(published.logsBody(published.layout()), "\n"), "\n")
	without := strings.Split(strings.Join(bare.logsBody(bare.layout()), "\n"), "\n")
	if len(withURL) != len(without) {
		t.Errorf("body is %d rows with an address and %d without: the panel moves under the reader",
			len(withURL), len(without))
	}
	if !strings.Contains(stripANSI(withURL[0]), "http://web.wtm") {
		t.Errorf("first row = %q, want the address", stripANSI(withURL[0]))
	}
	// The row is kept, and it says why it holds no address rather than leaving a
	// hole where one usually is.
	if !strings.Contains(stripANSI(without[0]), domain.DashboardLogsNoAddress) {
		t.Errorf("first row = %q, want it to say the job declares no port", stripANSI(without[0]))
	}
}

// A live preview draws a bordered pane whose first row is that border and whose
// second names the job. The column starts on that second row, so the job it
// highlights sits level with the job the pane names.
func TestTheJobColumnLinesUpWithThePaneTitle(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{{Name: "web", Kind: domain.JobKindService, Status: domain.JobStatusStopped}},
	})
	model := logsModel(t, RunParams{
		BoardLoader: func(logsRequest) runlogs.Board { return board },
		LogsLoader:  func(logsRequest) ([]string, error) { return nil, nil },
	}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}, {Name: "worker"}})

	model, _ = model.openLogsTabOn("web")
	rows := strings.Split(stripANSI(strings.Join(model.logsBody(model.layout()), "\n")), "\n")

	border, first := -1, -1
	for i, row := range rows {
		if border < 0 && strings.Contains(row, "╭") {
			border = i
		}
		if first < 0 && strings.Contains(row, "web") {
			first = i
		}
	}
	if border < 0 || first < 0 {
		t.Fatalf("body holds no pane or no job:\n%s", strings.Join(rows, "\n"))
	}
	if first != border+1 {
		t.Errorf("first job on row %d, pane border on row %d: want the job level with the pane's title", first, border)
	}
}

// The output panel takes its rows from the panel above it. A preview sized when
// it opened and never again was drawn into a box it no longer fitted.
func TestThePreviewFollowsTheOutputPanelOpening(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{{Name: "web", Kind: domain.JobKindService, Status: domain.JobStatusStopped}},
	})
	model := logsModel(t, RunParams{
		BoardLoader: func(logsRequest) runlogs.Board { return board },
		LogsLoader:  func(logsRequest) ([]string, error) { return nil, nil },
	}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})

	model, _ = model.openLogsTabOn("web")
	before := len(model.logsBody(model.layout()))

	next, _ := model.Update(key(keyToggleOutput))
	expanded, _ := next.(Model)
	if !expanded.outputExpanded {
		t.Fatal("the output panel did not open")
	}

	after := len(expanded.logsBody(expanded.layout()))
	if after >= before {
		t.Fatalf("logs body is %d rows with the output panel open and %d without: it did not give any up", after, before)
	}
	rendered := len(strings.Split(strings.Join(expanded.logsBody(expanded.layout()), "\n"), "\n"))
	if rendered != after {
		t.Errorf("body renders %d rows for a %d-row box", rendered, after)
	}
}

// The preview builds its own addresses, so it needs the one worktree fact it
// cannot read for itself. Without it the pane announced ports under a header
// announcing the url — the same job's address spelled two ways, two lines apart.
func TestThePreviewsBoardIsToldWhetherTheWorktreeAnswersOnItsPorts(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{{Name: "web", Kind: domain.JobKindService, Status: domain.JobStatusRunning}},
	})

	var asked logsRequest
	model := logsModel(t, RunParams{
		BoardLoader: func(req logsRequest) runlogs.Board {
			asked = req
			return board
		},
		LogsLoader: func(logsRequest) ([]string, error) { return nil, nil },
	}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "web"}})
	model.portAddressed = map[string]bool{"a": true}

	model, _ = model.openLogsTabOn("web")

	if !model.previewOn {
		t.Fatal("the panel opened without a preview although a board was available")
	}
	if !asked.PortAddressed {
		t.Error("the preview's board was opened without the worktree's addressing verdict")
	}
}

// declaringRunJobs gives a model its run.toml and marks every job in it as
// having run in each worktree on screen. What a surface shows is decided by
// rules.VisibleJobs and tested there; these tests are about the column, the
// tail and the preview, and would otherwise all be asserting the same rule a
// second time.
func declaringRunJobs(model Model, jobs []domain.JobConfig) Model {
	model.runConfig = domain.RunConfig{Jobs: jobs}

	logged := make(map[string]bool, len(jobs))
	for _, job := range jobs {
		logged[job.Name] = true
	}
	model.logged = make(map[string]map[string]bool, len(model.statuses))
	for _, status := range model.statuses {
		model.logged[status.Branch] = logged
	}
	return model
}

// A runner publishes nothing of its own, so the addresses a reader came for are
// its children's. The view used to head with their number alone — naming
// something it gave no way to reach — and now heads with one line per address,
// each opening its own url. They sit in the row where an address has always
// been read, not in the job column, which is for picking a log.
func TestTheLogsViewHeadsWithEachAddressARunnerAnswersFor(t *testing.T) {
	opened := make(chan string, 1)
	model := logsModel(t, RunParams{
		URLOpener:  func(url string) error { opened <- url; return nil },
		LogsLoader: func(logsRequest) ([]string, error) { return nil, nil },
	}, "a")
	model = declaringRunJobs(model, []domain.JobConfig{{Name: "dev", Runs: []string{"web", "api"}}})
	model.addresses = map[string]map[string]domain.JobAddress{"a": {"dev": {Held: []domain.JobURLEntry{
		{Job: "web", URL: "http://web.wtm"},
		{Job: "api", URL: "http://api.wtm"},
	}}}}
	model.panelTab, model.logsBranch, model.logsJob = panelLogs, "a", "dev"

	head := stripANSI(strings.Join(model.logsAddressLines(60), "\n"))
	for _, want := range []string{"web", "http://web.wtm", "api", "http://api.wtm"} {
		if !strings.Contains(head, want) {
			t.Errorf("head = %q, misses %q", head, want)
		}
	}
	// The column stays a job selector: the addresses are not in it.
	column := stripANSI(strings.Join(model.logsJobColumn(logsJobColumnParams{Width: 20, Rows: 10}), "\n"))
	if strings.Contains(column, "http://") {
		t.Errorf("column = %q, want no address in the job selector", column)
	}

	renderAndWait(t, model, logsHeldZone("web"))
	zone := model.zones.Get(logsHeldZone("web"))
	if zone.IsZero() {
		t.Fatal("an address the view heads with has no zone of its own")
	}
	next, cmd := updateCmd(model, click(zone.StartX, zone.StartY))
	if cmd == nil {
		t.Fatal("clicking an address did nothing")
	}
	// The pane keeps showing the runner: it is the process writing that output.
	if next.logsJob != "dev" {
		t.Errorf("logsJob = %q, want the runner still on screen", next.logsJob)
	}
	cmd()
	select {
	case got := <-opened:
		if got != "http://web.wtm" {
			t.Errorf("opened %q, want the address on the line clicked", got)
		}
	default:
		t.Fatal("the address was not opened")
	}
}
