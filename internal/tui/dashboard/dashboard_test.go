package dashboard

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

const (
	testWidth  = 120
	testHeight = 40
	narrowWide = 80
)

func statuses(branches ...string) []domain.WorktreeStatus {
	out := make([]domain.WorktreeStatus, 0, len(branches))
	for _, branch := range branches {
		out = append(out, domain.WorktreeStatus{Branch: branch, Path: "/tmp/" + branch})
	}
	return out
}

func newTestModel(t *testing.T, width, height int, branches ...string) Model {
	t.Helper()
	model := New(RunParams{})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: width, Height: height})
	return update(model, worktreesMsg{statuses: statuses(branches...), parents: map[string]string{}})
}

func update(model Model, msg tea.Msg) Model {
	next, _ := model.Update(msg)
	return next.(Model)
}

func updateCmd(model Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := model.Update(msg)
	return next.(Model), cmd
}

func key(name string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)} }

func namedKey(kind tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: kind} }

func click(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
}

func wheel(x, y int, button tea.MouseButton) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: button}
}

// renderAndWait renders one frame and blocks until the zone manager has stored
// the zones that frame declared. Scan hands them to a worker goroutine, so a
// mouse assertion right after View would otherwise race the registration.
func renderAndWait(t *testing.T, model Model, ids ...string) {
	t.Helper()
	model.View()
	for _, id := range ids {
		waitFor(t, id, func() bool { return !model.zones.Get(id).IsZero() })
	}
}

// renderAndWaitAt is renderAndWait for a second frame: a zone that already
// exists keeps its previous bounds until the new scan is applied, so presence
// alone would let an assertion read the old position.
func renderAndWaitAt(t *testing.T, model Model, id string, startY int) {
	t.Helper()
	model.View()
	waitFor(t, id, func() bool {
		info := model.zones.Get(id)
		return !info.IsZero() && info.StartY == startY
	})
}

func waitFor(t *testing.T, id string, settled func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !settled() {
		if time.Now().After(deadline) {
			t.Fatalf("zone %q never settled", id)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCursorMovesAndClampsAtBothEnds(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")

	if model = update(model, namedKey(tea.KeyDown)); model.cursor != 1 {
		t.Fatalf("cursor = %d after down, want 1", model.cursor)
	}
	if model = update(model, key("j")); model.cursor != 2 {
		t.Fatalf("cursor = %d after j, want 2", model.cursor)
	}
	if model = update(model, key("j")); model.cursor != 2 {
		t.Fatalf("cursor = %d past the last row, want it clamped to 2", model.cursor)
	}
	if model = update(model, key("k")); model.cursor != 1 {
		t.Fatalf("cursor = %d after k, want 1", model.cursor)
	}
	if model = update(model, namedKey(tea.KeyUp)); model.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", model.cursor)
	}
	if model = update(model, namedKey(tea.KeyUp)); model.cursor != 0 {
		t.Fatalf("cursor = %d before the first row, want it clamped to 0", model.cursor)
	}
}

func TestHomeAndEndJumpToTheEdges(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c", "d")

	if model = update(model, key("G")); model.cursor != 3 {
		t.Fatalf("cursor = %d after G, want the last row", model.cursor)
	}
	if model = update(model, key("g")); model.cursor != 0 {
		t.Fatalf("cursor = %d after g, want the first row", model.cursor)
	}
}

func TestCursorScrollsTheWindowWhenTheListOverflows(t *testing.T) {
	branches := make([]string, 40)
	for i := range branches {
		branches[i] = string(rune('a' + i%26))
	}
	// 13 is the shortest height that still fits one full row under the 3-line
	// header, the panel chrome and the folded output panel.
	model := newTestModel(t, testWidth, 13, branches...)
	rows := model.layout().ListRows
	if rows == 0 {
		t.Fatal("test setup: no row fits, the scroll assertion below would be vacuous")
	}

	for range branches {
		model = update(model, namedKey(tea.KeyDown))
	}

	if model.cursor != len(branches)-1 {
		t.Fatalf("cursor = %d, want the last row", model.cursor)
	}
	if want := len(branches) - rows; model.offset != want {
		t.Errorf("offset = %d, want %d so the last row stays visible", model.offset, want)
	}
}

func TestPollResultKeepsTheCursorOnAValidRow(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b", "c")
	model = update(model, key("G"))

	model = update(model, worktreesMsg{statuses: statuses("a"), parents: map[string]string{}})

	if model.cursor != 0 {
		t.Errorf("cursor = %d after the list shrank to one row, want 0", model.cursor)
	}
}

func TestFailedLoadKeepsTheLastGoodListAndReportsIt(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	failure := errors.New("git exploded")

	model = update(model, worktreesMsg{err: failure})

	if len(model.statuses) != 2 {
		t.Errorf("statuses dropped to %d, want the last good list kept", len(model.statuses))
	}
	if !errors.Is(model.loadErr, failure) {
		t.Errorf("loadErr = %v, want the load failure", model.loadErr)
	}
	if !strings.Contains(model.renderHelpBar(model.layout()), "git exploded") {
		t.Error("the help bar should surface the load failure")
	}
}

func TestTheGitPollDoesNotStackLoadsButKeepsTicking(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	model, cmd := updateCmd(model, gitPollMsg{})
	if !model.loading || cmd == nil {
		t.Fatalf("first poll: loading=%v cmd=%v, want a load in flight and a next tick", model.loading, cmd != nil)
	}

	model, cmd = updateCmd(model, gitPollMsg{})
	if !model.loading || cmd == nil {
		t.Fatal("a poll landing while a load is in flight must still schedule the next tick")
	}

	model = update(model, worktreesMsg{statuses: statuses("a"), parents: map[string]string{}})
	if model.loading {
		t.Error("loading should clear when the result lands")
	}
}

// Asking the daemon must never drag the repository along behind it.
func TestTheDaemonPollDoesNotReadTheRepository(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	model, cmd := updateCmd(model, pollMsg{})

	if model.loading {
		t.Error("the daemon poll started a git load; that is the git clock's job")
	}
	if cmd == nil {
		t.Error("the daemon poll must schedule its next tick")
	}
}

func TestRefreshKeyReloadsBothWorktreesAndPRs(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model = update(model, prsMsg{prs: []domain.PRInfo{{Number: 1, Branch: "a"}}})

	model, cmd := updateCmd(model, key(domain.KeyRefresh))

	if !model.loading || model.prsLoaded {
		t.Errorf("refresh state = loading:%v prsLoaded:%v, want both reloading", model.loading, model.prsLoaded)
	}
	if cmd == nil {
		t.Error("refresh must issue a command")
	}
}

func TestOutputPanelFoldsAndUnfolds(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	if model.layout().OutputLines != 0 {
		t.Fatal("the output panel starts folded")
	}

	model = update(model, key(domain.KeyToggleOutput))
	if model.layout().OutputLines != domain.DashboardOutputBodyHeight {
		t.Errorf("output lines = %d after o, want it unfolded", model.layout().OutputLines)
	}

	model = update(model, key(domain.KeyToggleOutput))
	if model.layout().OutputLines != 0 {
		t.Error("a second o must fold it back")
	}
}

func TestOutputLinesAppendAndFollowTheTail(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model = update(model, key(domain.KeyToggleOutput))
	visible := model.layout().OutputLines

	for i := 0; i < visible+5; i++ {
		model = update(model, OutputLineMsg{Text: "line"})
	}

	if len(model.outputLines) != visible+5 {
		t.Fatalf("stored %d lines, want %d", len(model.outputLines), visible+5)
	}
	if want := len(model.outputLines) - visible; model.outputOffset != want {
		t.Errorf("offset = %d, want %d so the newest line stays visible", model.outputOffset, want)
	}
}

func TestShiftArrowsScrollTheOutputPanel(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model = update(model, key(domain.KeyToggleOutput))
	for i := 0; i < 30; i++ {
		model = update(model, OutputLineMsg{Text: "line"})
	}

	before := model.outputOffset
	model = update(model, tea.KeyMsg{Type: tea.KeyShiftUp})
	if model.outputOffset != before-1 {
		t.Errorf("offset = %d after shift+up, want %d", model.outputOffset, before-1)
	}

	model = update(model, tea.KeyMsg{Type: tea.KeyShiftDown})
	if model.outputOffset != before {
		t.Errorf("offset = %d after shift+down, want %d", model.outputOffset, before)
	}
}

func TestNarrowTerminalOpensAndClosesTheDetail(t *testing.T) {
	model := newTestModel(t, narrowWide, testHeight, "a", "b")

	if !model.layout().Narrow {
		t.Fatal("80 columns must be narrow")
	}
	if model.layout().DetailVisible {
		t.Fatal("the detail starts closed in a narrow terminal")
	}

	model = update(model, namedKey(tea.KeyEnter))
	if !model.detailOpen || !model.layout().DetailVisible || model.layout().ListVisible {
		t.Fatal("enter must swap the list for the detail")
	}

	model = update(model, namedKey(tea.KeyEsc))
	if model.detailOpen || !model.layout().ListVisible {
		t.Fatal("esc must bring the list back")
	}
}

func TestWideTerminalIgnoresTheDetailToggleAndEscNeverQuits(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	model, cmd := updateCmd(model, namedKey(tea.KeyEnter))
	if model.detailOpen {
		t.Error("the detail is always on screen when wide; enter must not open an overlay")
	}
	if cmd != nil {
		t.Error("enter is inert in a wide terminal")
	}

	_, cmd = updateCmd(model, namedKey(tea.KeyEsc))
	if cmd != nil {
		t.Error("esc must not quit a persistent dashboard")
	}
}

func TestQuitKeys(t *testing.T) {
	for _, msg := range []tea.KeyMsg{key(domain.KeyQuit), {Type: tea.KeyCtrlC}} {
		model := newTestModel(t, testWidth, testHeight, "a")
		_, cmd := updateCmd(model, msg)
		if cmd == nil {
			t.Fatalf("%s should quit", msg.String())
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%s produced %T, want tea.QuitMsg", msg.String(), cmd())
		}
	}
}

func TestHelpOverlaySwallowsNavigationUntilClosed(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")

	model = update(model, key(domain.KeyHelp))
	if !model.showHelp {
		t.Fatal("? must open the help overlay")
	}

	model = update(model, namedKey(tea.KeyDown))
	if model.cursor != 0 {
		t.Error("navigation must not leak through the help overlay")
	}

	model = update(model, key(domain.KeyHelp))
	if model.showHelp {
		t.Error("? must close the help overlay")
	}
}

func TestQuitStillWorksFromTheHelpOverlay(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model = update(model, key(domain.KeyHelp))

	_, cmd := updateCmd(model, key(domain.KeyQuit))

	if cmd == nil {
		t.Fatal("the overlay advertises q as quit; it must not swallow it")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q produced %T, want tea.QuitMsg", cmd())
	}
}

func TestTabCyclingStaysInRange(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	model = update(model, namedKey(tea.KeyTab))
	if model.tab < 0 || model.tab >= len(tabs) {
		t.Fatalf("tab = %d, out of range for %d tabs", model.tab, len(tabs))
	}
	model = update(model, namedKey(tea.KeyShiftTab))
	if model.tab < 0 || model.tab >= len(tabs) {
		t.Fatalf("tab = %d, out of range for %d tabs", model.tab, len(tabs))
	}
}

func TestPRsArriveAsynchronously(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	if model.prsLoaded {
		t.Fatal("PRs are not loaded at startup")
	}
	if pr := model.prFor("a"); pr != nil {
		t.Errorf("PR for %q = %+v before the fetch lands, want nil", "a", pr)
	}

	model = update(model, prsMsg{prs: []domain.PRInfo{{Number: 42, Title: "Ship it", State: "OPEN", Branch: "a"}}})

	if !model.prsLoaded {
		t.Fatal("prsMsg must mark the PRs loaded")
	}
	pr := model.prFor("a")
	if pr == nil || pr.Number != 42 || pr.Title != "Ship it" {
		t.Errorf("PR for %q = %+v, want number 42 and title %q", "a", pr, "Ship it")
	}
	if model.prFor("other") != nil {
		t.Error("a branch without a PR resolves to nil")
	}
}

func TestDetailShowsBothDivergenceReferentials(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model.statuses[0] = domain.WorktreeStatus{
		Branch:       "a",
		Path:         "/tmp/a",
		CommitsAhead: 3,
		OriginState:  domain.DivergenceDiverged,
		OriginAhead:  2,
		OriginBehind: 5,
		CreatedAt:    time.Date(2026, 8, 18, 14, 3, 0, 0, time.Local),
	}
	model = update(model, prsMsg{})

	body := strings.Join(model.detailBody(model.layout()), "\n")

	for _, want := range []string{
		domain.BadgeGlyphAhead + "3",
		domain.BadgeGlyphAhead + "2 " + domain.BadgeGlyphBehind + "5",
		domain.DashboardLabelCreated, "/tmp/a",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("detail body is missing %q:\n%s", want, body)
		}
	}
}

func TestDetailIsEmptyWithoutASelection(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)

	body := strings.Join(model.detailBody(model.layout()), "\n")

	if !strings.Contains(body, domain.DashboardEmptySelection) {
		t.Errorf("empty detail = %q, want the placeholder", body)
	}
	if strings.Contains(strings.Join(model.listBody(model.layout()), "\n"), domain.LoadingWorktreesText) {
		t.Error("a loaded but empty list shows the empty placeholder, not the loader")
	}
}

func TestViewSurvivesDegenerateSizes(t *testing.T) {
	for _, size := range [][2]int{{0, 0}, {1, 1}, {20, 4}, {200, 3}, {120, 40}} {
		model := newTestModel(t, size[0], size[1], "a", "b")
		model.View()
	}
}

func TestLayoutMatchesTheDocumentedGeometry(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	layout := model.layout()
	want := rules.ComputeDashboardLayout(rules.DashboardLayoutParams{Width: testWidth, Height: testHeight})

	if layout != want {
		t.Errorf("layout = %+v, want %+v", layout, want)
	}
}

func TestViewNeverOverflowsTheTerminal(t *testing.T) {
	// 1, 2 and 3 pin the degenerate case: a terminal too short for the header
	// plus the help bar to both fit at full size. ComputeDashboardLayout must
	// shrink them rather than let the header overflow past what it was given.
	sizes := [][2]int{
		{120, 40}, {80, 30}, {100, 10}, {60, 6}, {40, 4},
		{200, 4}, {200, 3}, {200, 2}, {200, 1},
		{20, 20},
	}

	// A multi-line output entry (a hook or git error carrying embedded
	// newlines) must not push the frame past the terminal height either: a
	// sweep that only ever appends single-line entries cannot catch that
	// class of bug. 60 embedded lines outgrows every output panel budget in
	// the sizes above.
	multiline := "hook failed:\n" + strings.Repeat("hook output line\n", 60)

	for _, size := range sizes {
		model := newTestModel(t, size[0], size[1], "a", "b", "c")
		model = update(model, key(domain.KeyToggleOutput))
		model = update(model, OutputLineMsg{Text: multiline})

		view := model.View()
		lines := strings.Split(view, "\n")
		if len(lines) > size[1] {
			t.Errorf("%dx%d: view is %d lines, want at most %d", size[0], size[1], len(lines), size[1])
		}
		for index, line := range lines {
			if width := lipgloss.Width(line); width > size[0] {
				t.Errorf("%dx%d: line %d is %d columns wide", size[0], size[1], index, width)
			}
		}
	}
}

// TestMultilineOutputEntryDoesNotOverflowTheTerminal pins the exact bug
// report: a hook or git failure message appended as one OutputLineMsg with
// embedded newlines must not grow the frame past the terminal height. This
// is the assertion that would have caught the original bug — renderPanel and
// outputBody clipped by slice-entry count, not by rendered row count, so one
// multi-line entry escaped the panel's own box and scrolled the alt screen.
func TestMultilineOutputEntryDoesNotOverflowTheTerminal(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model = update(model, key(domain.KeyToggleOutput))
	model = update(model, OutputLineMsg{Text: strings.Repeat("hook output line\n", 30)})

	view := model.View()
	lines := strings.Split(view, "\n")
	if len(lines) > testHeight {
		t.Fatalf("view is %d lines after a multi-line output entry, want at most %d", len(lines), testHeight)
	}
}

// TestOutputPanelScrollsInternallyRatherThanGrowing pins the fix's intent:
// once an entry is split into one row each, more entries than the panel can
// show scroll its own window (offset + OutputLines) instead of growing the
// frame.
func TestOutputPanelScrollsInternallyRatherThanGrowing(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model = update(model, key(domain.KeyToggleOutput))

	before := len(strings.Split(model.View(), "\n"))

	for i := 0; i < 50; i++ {
		model = update(model, OutputLineMsg{Text: fmt.Sprintf("line %d", i)})
	}

	after := len(strings.Split(model.View(), "\n"))
	if after != before {
		t.Errorf("frame height = %d after appending output, want it unchanged at %d", after, before)
	}

	layout := model.layout()
	if layout.OutputLines <= 0 {
		t.Fatal("output panel has no visible rows to assert a window over")
	}
	body := model.outputBody(layout, layout.Output.Width-borderWidth-paddingWidth)
	if len(body) != layout.OutputLines {
		t.Errorf("output body = %d rows, want exactly the panel's %d visible rows", len(body), layout.OutputLines)
	}
	if !strings.Contains(strings.Join(body, "\n"), "line 49") {
		t.Error("the output panel should show a window ending at the tail of appended lines")
	}
}

func TestFoldingTheOutputPanelClampsItsScroll(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model = update(model, key(domain.KeyToggleOutput))
	for i := 0; i < 30; i++ {
		model = update(model, OutputLineMsg{Text: "line"})
	}
	if model.outputOffset == 0 {
		t.Fatal("the panel should be scrolled to the tail")
	}

	model = update(model, key(domain.KeyToggleOutput))

	if model.outputOffset != 0 {
		t.Errorf("offset = %d once folded, want it clamped back to 0", model.outputOffset)
	}
}

// Every leaf runs on its own goroutine: a batch from Init holds listenCmd,
// which blocks on the flow channel until the program ends.
func fire(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		if batch, ok := cmd().(tea.BatchMsg); ok {
			for _, sub := range batch {
				fire(sub)
			}
		}
	}()
}

func TestJobsAreReadWithoutAnExplicitRefresh(t *testing.T) {
	wakes := make(chan bool, 8)
	model := New(RunParams{JobsLoader: func(wake bool) ([]domain.JobInfo, bool) {
		wakes <- wake
		return nil, true
	}})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{statuses: statuses("a"), parents: map[string]string{}})

	fire(model.Init())
	if wake := nextWake(t, wakes); !wake {
		t.Error("the first read wakes a sleeping daemon: the opening frame must be true")
	}

	_, cmd := updateCmd(model, pollMsg{})
	fire(cmd)
	if wake := nextWake(t, wakes); wake {
		t.Error("the poll must not wake a daemon: it reads what is there")
	}
}

func nextWake(t *testing.T, wakes chan bool) bool {
	t.Helper()
	select {
	case wake := <-wakes:
		return wake
	case <-time.After(5 * time.Second):
		t.Fatal("the jobs were never read")
		return false
	}
}

func TestJobCountsAreDerivedFromTheDaemonIndex(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	model = update(model, jobsMsg{
		jobs:    []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}},
		running: map[string]int{"/tmp/a": 1},
		known:   true,
	})

	if model.running["/tmp/a"] != 1 {
		t.Fatalf("running = %v, want the count the daemon reported", model.running)
	}
	if len(model.jobs) != 1 {
		t.Fatalf("jobs = %v, want the index kept for the detail panel", model.jobs)
	}
}

// A daemon that has withdrawn while detached stacks are still indexed cannot
// say what runs. Taking that silence for "nothing runs" blinked a running stack
// out of the panel on every poll, and back in on every refresh.
func TestAReadThatCouldNotTellKeepsTheCountsOnScreen(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model = update(model, jobsMsg{
		jobs:    []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}},
		running: map[string]int{"/tmp/a": 1},
		known:   true,
	})

	model = update(model, jobsMsg{known: false})

	if model.running["/tmp/a"] != 1 || len(model.jobs) != 1 {
		t.Fatalf("running = %v, jobs = %v, want the last reading kept", model.running, model.jobs)
	}
}

func TestAReadThatTellsNothingIsUpClearsTheCounts(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model = update(model, jobsMsg{
		jobs:    []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}},
		running: map[string]int{"/tmp/a": 1},
		known:   true,
	})

	model = update(model, jobsMsg{known: true})

	if len(model.running) != 0 || len(model.jobs) != 0 {
		t.Fatalf("running = %v, jobs = %v, want them cleared by an answer", model.running, model.jobs)
	}
}

func TestJobsPollOnlyAsksAddressesForWorktreesThatHaveSomethingUp(t *testing.T) {
	asked := make(chan []string, 1)
	model := New(RunParams{
		JobsLoader: func(bool) ([]domain.JobInfo, bool) {
			return []domain.JobInfo{
				{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"},
			}, true
		},
		AddressLoader: func(request AddressRequest) domain.RunAddresses {
			asked <- request.Branches
			return domain.RunAddresses{
				ByBranch: map[string]map[string]domain.JobAddress{"a": {"web": {URL: "http://web.wtm"}}},
			}
		},
	})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{statuses: statuses("a", "idle"), parents: map[string]string{}})

	jobs, _ := model.loadJobsCmd(false)().(jobsMsg)
	jobs.config = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model, _ = model.applyJobs(jobs)

	msg := model.resolveAddressesCmd()()

	select {
	case branches := <-asked:
		if len(branches) != 1 || branches[0] != "a" {
			t.Errorf("branches = %v, want only the one with a job up: BranchEnv writes an ordinal", branches)
		}
	default:
		t.Fatal("addresses were never asked for")
	}

	got, ok := msg.(addressesMsg)
	if !ok {
		t.Fatalf("msg = %T, want addressesMsg", msg)
	}
	if got.addresses["a"]["web"].URL != "http://web.wtm" {
		t.Errorf("addresses = %v, want them carried by the poll", got.addresses)
	}
}

// The daemon is polled every few seconds; what it holds up changes far less
// often, and re-deriving from a reading identical to the last one is where an
// idle dashboard spent its git.
func TestAJobsPollThatFindsTheSameThingsUpDerivesNothingAgain(t *testing.T) {
	model := New(RunParams{
		AddressLoader: func(AddressRequest) domain.RunAddresses { return domain.RunAddresses{} },
		TraceLoader:   func([]string) map[string]map[string]bool { return nil },
	})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{statuses: statuses("a"), parents: map[string]string{}})

	reading := jobsMsg{
		jobs:    []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}},
		running: map[string]int{"/tmp/a": 1},
		config:  domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
		known:   true,
	}

	model, cmd := model.applyJobs(reading)
	if cmd == nil {
		t.Fatal("the first reading of a job coming up must resolve what it implies")
	}

	model, cmd = model.applyJobs(reading)
	if cmd != nil {
		t.Error("a poll that found exactly the same jobs up re-derived them anyway")
	}

	down := reading
	down.jobs, down.running = nil, nil
	if _, cmd = model.applyJobs(down); cmd == nil {
		t.Error("the job going down must re-derive: that is precisely when a trace appears")
	}
}

// An address is not a function of the jobs alone: the loader dials the proxy
// and reads the worktree's .env. A proxy that came up after the dashboard, or a
// port that moved under a job that never stopped, is caught only by resolving
// again — and the jobs poll cannot see either of them.
func TestAddressesAreResolvedAgainOnTheGitPoll(t *testing.T) {
	asked := make(chan []string, 8)
	model := New(RunParams{
		AddressLoader: func(request AddressRequest) domain.RunAddresses {
			asked <- request.Branches
			return domain.RunAddresses{
				ByBranch: map[string]map[string]domain.JobAddress{"a": {"web": {URL: "http://web.wtm"}}},
			}
		},
	})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{statuses: statuses("a"), parents: map[string]string{}})
	model, _ = model.applyJobs(jobsMsg{
		jobs:    []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}},
		running: map[string]int{"/tmp/a": 1},
		config:  domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
		known:   true,
	})
	model = update(model, addressesMsg{
		addresses: map[string]map[string]domain.JobAddress{"a": {"web": {URL: "http://stale"}}},
	})
	forget(asked)

	_, cmd := updateCmd(model, worktreesMsg{statuses: statuses("a"), parents: map[string]string{}})
	fire(cmd)

	select {
	case <-asked:
	case <-time.After(5 * time.Second):
		t.Fatal("the addresses on hand are never resolved again: a stale URL would outlive the session, KeyRefresh included")
	}
}

// The running set is compared with its counts, not as a set of branches: a job
// stopping beside another still up moves no branch, and the tree carries a
// per-node count that would keep saying two.
func TestAJobStoppingBesideAnotherStillUpRederives(t *testing.T) {
	model := New(RunParams{
		AddressLoader: func(AddressRequest) domain.RunAddresses { return domain.RunAddresses{} },
		TraceLoader:   func([]string) map[string]map[string]bool { return nil },
	})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{statuses: statuses("a"), parents: map[string]string{}})

	both := jobsMsg{
		jobs: []domain.JobInfo{
			{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"},
			{Name: "api", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"},
		},
		running: map[string]int{"/tmp/a": 2},
		config:  domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "api"}}},
		known:   true,
	}
	model, _ = model.applyJobs(both)

	one := both
	one.jobs = both.jobs[:1]
	one.running = map[string]int{"/tmp/a": 1}
	if _, cmd := model.applyJobs(one); cmd == nil {
		t.Error("one of two jobs stopped and nothing was re-derived; the tree would keep its old count")
	}
}

func forget(asked chan []string) {
	for {
		select {
		case <-asked:
		default:
			return
		}
	}
}

func TestJobsPollAsksNoAddressWhenNothingIsUp(t *testing.T) {
	called := false
	model := New(RunParams{
		JobsLoader: func(bool) ([]domain.JobInfo, bool) { return nil, true },
		AddressLoader: func(AddressRequest) domain.RunAddresses {
			called = true
			return domain.RunAddresses{}
		},
	})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, worktreesMsg{statuses: statuses("a"), parents: map[string]string{}})

	if cmd := model.resolveAddressesCmd(); cmd != nil {
		cmd()
	}

	if called {
		t.Error("addresses were asked for with nothing up, want the ordinal left unallocated")
	}
}

// Init loads worktrees and jobs in parallel, so the jobs command is built while
// m.statuses is still empty. Capturing the statuses there asked for no address
// at all, and the RUN section showed no url until the next poll.
func TestAddressesLandEvenWhenTheJobsLoadRacesTheWorktrees(t *testing.T) {
	model := New(RunParams{
		JobsLoader: func(bool) ([]domain.JobInfo, bool) {
			return []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/tmp/a"}}, true
		},
		AddressLoader: func(request AddressRequest) domain.RunAddresses {
			if len(request.Branches) != 1 || request.Branches[0] != "a" {
				t.Errorf("branches = %v, want the worktree that has a job up", request.Branches)
			}
			return domain.RunAddresses{
				ByBranch: map[string]map[string]domain.JobAddress{"a": {"web": {URL: "http://web.wtm"}}},
			}
		},
	})
	t.Cleanup(model.Close)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})

	jobs, ok := model.loadJobsCmd(true)().(jobsMsg)
	if !ok {
		t.Fatal("want a jobsMsg")
	}
	jobs.config = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model, _ = model.applyJobs(jobs)
	model = update(model, worktreesMsg{statuses: statuses("a"), parents: map[string]string{}})

	cmd := model.resolveAddressesCmd()
	if cmd == nil {
		t.Fatal("cmd = nil, want the addresses asked for once the worktrees are known")
	}
	got, ok := cmd().(addressesMsg)
	if !ok {
		t.Fatalf("msg = %T, want addressesMsg", cmd())
	}
	if got.addresses["a"]["web"].URL != "http://web.wtm" {
		t.Errorf("addresses = %v, want them resolved on the first load", got.addresses)
	}
}
