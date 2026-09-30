package runview

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

func inWorktree(view runlogs.JobView, dir, branch string) runlogs.JobView {
	view.WorkDir, view.Worktree = dir, branch
	return view
}

func multiModel(t *testing.T) Model {
	t.Helper()
	board := runlogstest.NewBoard(runlogstest.BoardParams{Views: []runlogs.JobView{
		inWorktree(running("web"), "/work/main", "main"),
		inWorktree(running("api"), "/work/main", "main"),
		inWorktree(running("web"), "/work/feature", "feature"),
	}})
	model := New(Params{Board: board})
	t.Cleanup(func() { model.panes.closeAll() })
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	return update(model, exec(t, model.refreshCmd()))
}

func TestTheListGroupsJobsUnderTheirWorktree(t *testing.T) {
	rows := multiModel(t).rows()

	if len(rows) != 6 {
		t.Fatalf("rows = %d, want three jobs under two headings, the groups set apart", len(rows))
	}
	if rows[0].Header != "main" || rows[4].Header != "feature" {
		t.Fatalf("headings landed at %q and %q", rows[0].Header, rows[4].Header)
	}
	// A blank line sets the second group off from the first, and only the second.
	if !rows[3].Spacer {
		t.Errorf("row 3 = %+v, want the groups set apart", rows[3])
	}
	for _, index := range []int{1, 2, 5} {
		if rows[index].Header != "" || rows[index].Spacer {
			t.Errorf("row %d is not a job row: %+v", index, rows[index])
		}
	}
}

// A board naming no worktree at all gets no heading: an empty row says less
// than the job it would push off the panel.
func TestAListWithNoWorktreeToNameGetsNoHeading(t *testing.T) {
	h := newHarness(t, harnessParams{Views: []runlogs.JobView{running("api"), running("web")}})

	for _, row := range h.model.rows() {
		if row.Header != "" || row.Spacer {
			t.Fatalf("a board naming no worktree got %+v", row)
		}
	}
}

// Headings are not selectable: they are a level of the list, not a target.
func TestMovingSkipsTheWorktreeHeadings(t *testing.T) {
	model := multiModel(t)

	seen := map[jobKey]bool{}
	for range 3 {
		seen[model.selected] = true
		next, _ := model.move(1)
		model = next.(Model)
	}
	if len(seen) != 3 {
		t.Fatalf("moving reached %d jobs, want each of them once", len(seen))
	}
}

// Two worktrees running the same profile hold two jobs called `web`. Selecting
// one by name would select both — the same pane and the same subscription.
func TestTheTwoJobsOfTheSameNameAreDistinctTargets(t *testing.T) {
	model := multiModel(t)

	first := model.selected
	for range 2 {
		next, _ := model.move(1)
		model = next.(Model)
	}
	if model.selected == first {
		t.Fatal("the cursor came back to the first job")
	}
	if model.selected.job() != first.job() {
		t.Fatalf("selected %q then %q, want the two jobs of the same name", first.job(), model.selected.job())
	}
	if model.selected.workDir() == first.workDir() {
		t.Fatal("the two selections share a worktree, so they are the same job")
	}
}

func TestTheHeaderCountsTheWorktrees(t *testing.T) {
	view := ansi.Strip(multiModel(t).View())

	if !strings.Contains(view, "2 worktrees") {
		t.Errorf("header does not count the worktrees:\n%s", view)
	}
}

// A run over several worktrees ends several times, and one of them ending says
// nothing about the others.
func TestTheSequenceEndsOnlyWhenEveryWorktreeHasReported(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{Views: []runlogs.JobView{
		inWorktree(running("web"), "/work/main", "main"),
		inWorktree(running("web"), "/work/feature", "feature"),
	}})
	model := New(Params{
		Board:     board,
		Worktrees: []string{"main", "feature"},
		Start:     func(context.Context, runlogs.Sink) (runlogs.Outcomes, error) { return nil, nil },
		Profile:   "dev",
	})
	t.Cleanup(func() { model.panes.closeAll() })
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})

	model = update(model, eventMsg{event: runlogs.Event{
		Phase: runlogs.PhaseStarting, Job: "web", WorkDir: "/work/main", Worktree: "main", Step: 1, Steps: 1,
	}})
	model = update(model, eventMsg{event: runlogs.Event{
		Phase: runlogs.PhaseReady, WorkDir: "/work/main", Worktree: "main",
		Outcome: runlogs.Outcome{WorkDir: "/work/main", Worktree: "main", Steps: 1, Started: []string{"web"}},
	}})
	if !model.sequence.active {
		t.Fatal("the sequence ended although one worktree had not reported")
	}

	model = update(model, eventMsg{event: runlogs.Event{
		Phase: runlogs.PhaseReady, WorkDir: "/work/feature", Worktree: "feature",
		Outcome: runlogs.Outcome{WorkDir: "/work/feature", Worktree: "feature", Steps: 1, Started: []string{"web"}},
	}})
	if model.sequence.active {
		t.Fatal("the sequence is still running with every worktree reported")
	}
	if len(model.sequence.outcomes) != 2 {
		t.Fatalf("outcomes = %d, want one per worktree", len(model.sequence.outcomes))
	}
}

// They abort independently, so naming one would hide the rest.
func TestTheReportNamesEveryWorktreeThatAborted(t *testing.T) {
	model := multiModel(t)
	model.started = true
	model.sequence.record(runlogs.Outcome{
		WorkDir: "/work/main", Worktree: "main", Steps: 2, Failed: "web", FailedStep: 1,
	})
	model.sequence.record(runlogs.Outcome{
		WorkDir: "/work/feature", Worktree: "feature", Steps: 2, Failed: "api", FailedStep: 2,
	})

	report := strings.Join(model.report(), "\n")
	for _, want := range []string{domain.RunViewAbortTitle, "main", "feature"} {
		if !strings.Contains(report, want) {
			t.Errorf("report is missing %q:\n%s", want, report)
		}
	}
}

// N sequences write into N panes at once. Following the run from one worktree's
// job to another's must not release the pane being left: nothing replays what a
// run is writing into it yet, so it is the only copy of that output.
func TestFollowingTheRunKeepsThePaneOfAWorktreeStillStarting(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{Views: []runlogs.JobView{
		inWorktree(running("web"), "/work/main", "main"),
		inWorktree(running("web"), "/work/feature", "feature"),
	}})
	model := New(Params{
		Board:     board,
		Worktrees: []string{"main", "feature"},
		Start:     func(context.Context, runlogs.Sink) (runlogs.Outcomes, error) { return nil, nil },
	})
	t.Cleanup(func() { model.panes.closeAll() })
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, exec(t, model.refreshCmd()))

	starting := func(dir, branch string) eventMsg {
		return eventMsg{event: runlogs.Event{
			Phase: runlogs.PhaseStarting, Job: "web", WorkDir: dir, Worktree: branch, Step: 1, Steps: 1,
		}}
	}

	model = update(model, starting("/work/main", "main"))
	model.panes.write(writeChunkParams{
		Key: jobKeyOf("/work/main", "web"), Source: sourceSequence, Chunk: []byte("listening on 8119\r\n"),
	})

	// The second worktree starts its own job, and the cursor follows it there.
	model = update(model, starting("/work/feature", "feature"))
	if model.selected != jobKeyOf("/work/feature", "web") {
		t.Fatalf("selected = %q, want the cursor to follow the run", model.selected)
	}

	entry, held := model.panes.entry(jobKeyOf("/work/main", "web"))
	if !held {
		t.Fatal("the first worktree's pane was released while its run was still writing into it")
	}
	if got := ansi.Strip(entry.pane.Render()); !strings.Contains(got, "listening on 8119") {
		t.Errorf("pane = %q, want what the run had written kept", got)
	}
}

// Two accounts run together read as one list of jobs, which is exactly what the
// heading is there to deny.
func TestTheRecapSetsEachWorktreesBlockApart(t *testing.T) {
	model := multiModel(t)
	model.started = true
	model.profile = "dev"
	model.sequence.record(runlogs.Outcome{
		WorkDir: "/work/main", Worktree: "main", Steps: 1, Started: []string{"web"},
	})
	model.sequence.record(runlogs.Outcome{
		WorkDir: "/work/feature", Worktree: "feature", Steps: 1, Started: []string{"web"},
	})

	if !blankLineBetween(recapLines(model), "main", "feature") {
		t.Errorf("the two blocks run together:\n%s", ansi.Strip(model.recap()))
	}
}

// A single worktree is named like any other: the recap of the run they do most
// is exactly the one that must not leave them guessing.
func TestTheRecapNamesASingleWorktreeToo(t *testing.T) {
	model := manyWorktreesModel(t, "main")
	model.started = true
	model.profile = "dev"
	model.sequence.record(runlogs.Outcome{
		WorkDir: "/w/main", Worktree: "main", Steps: 1, Started: []string{"web"},
	})

	lines := recapLines(model)
	named := false
	for _, line := range lines {
		if line == "main" {
			named = true
		}
	}
	if !named {
		t.Errorf("the recap does not name the worktree:\n%s", ansi.Strip(model.recap()))
	}
	if !blankLineBetween(lines, "Profile:", "main") {
		t.Errorf("the block is not set off from the profile line:\n%s", ansi.Strip(model.recap()))
	}
}

// recapLines is the recap without the box the styles draw around it: what the
// reader sees as content, one line each.
func recapLines(model Model) []string {
	var lines []string
	for _, line := range strings.Split(ansi.Strip(model.recap()), "\n") {
		lines = append(lines, strings.TrimSpace(strings.ReplaceAll(line, "\u2503", "")))
	}
	return lines
}

func blankLineBetween(lines []string, first, second string) bool {
	from, to := -1, -1
	for index, line := range lines {
		if from < 0 && strings.HasPrefix(line, first) {
			from = index
		}
		if from >= 0 && index > from && strings.HasPrefix(line, second) {
			to = index
			break
		}
	}
	if from < 0 || to < 0 {
		return false
	}
	for _, line := range lines[from+1 : to] {
		if line == "" {
			return true
		}
	}
	return false
}

func manyWorktreesModel(t *testing.T, worktrees ...string) Model {
	t.Helper()
	var views []runlogs.JobView
	for _, branch := range worktrees {
		for _, job := range []string{"docker", "web", "api"} {
			views = append(views, inWorktree(running(job), "/w/"+branch, branch))
		}
	}
	board := runlogstest.NewBoard(runlogstest.BoardParams{Views: views})
	model := New(Params{Board: board, Profile: "dev"})
	t.Cleanup(func() { model.panes.closeAll() })
	model = update(model, tea.WindowSizeMsg{Width: 100, Height: 24})
	return update(model, exec(t, model.refreshCmd()))
}

// A list long enough to scroll takes its headings away with it, and the rows
// left at the top then belong to a worktree nothing on screen names.
func TestALongListPinsTheHeadingItScrolledPast(t *testing.T) {
	model := manyWorktreesModel(t, "main", "feat-a", "feat-b", "feat-c", "feat-d", "feat-e")
	for range 14 {
		next, _ := model.move(1)
		model = next.(Model)
	}
	if model.offset == 0 {
		t.Fatal("the list never scrolled, so there is nothing to pin")
	}

	sidebar := sidebarLines(model)
	if len(sidebar) == 0 {
		t.Fatal("the sidebar rendered nothing")
	}
	if !isHeading(model, sidebar[0]) {
		t.Errorf("the list opens on %q, want the heading of the group it is inside:\n%s",
			sidebar[0], strings.Join(sidebar, "\n"))
	}
}

// Pinning costs a row. Counting it as available is what let the cursor slide out
// from under the panel it was measured to fit in.
func TestTheCursorStaysOnScreenUnderThePinnedHeading(t *testing.T) {
	model := manyWorktreesModel(t, "main", "feat-a", "feat-b", "feat-c", "feat-d", "feat-e")
	for step := range 17 {
		next, _ := model.move(1)
		model = next.(Model)
		if !strings.Contains(strings.Join(sidebarLines(model), "\n"), domain.RunViewCursorMark) {
			t.Fatalf("the cursor left the panel after %d moves", step+1)
		}
	}
}

// A single worktree is named like any other: a list whose shape changes between
// one and two is a shape the reader has to learn twice.
func TestASingleWorktreeIsNamedToo(t *testing.T) {
	model := manyWorktreesModel(t, "main")

	rows := model.rows()
	if len(rows) == 0 || rows[0].Header != "main" {
		t.Fatalf("the list does not name the only worktree: %+v", rows)
	}
}

// sidebarLines is the job list as drawn, without the box around it.
func sidebarLines(model Model) []string {
	var lines []string
	for _, line := range strings.Split(ansi.Strip(model.renderSidebar(model.layout())), "\n") {
		trimmed := strings.Trim(line, "│ ")
		if trimmed == "" || trimmed == domain.RunViewJobsTitle || strings.Contains(trimmed, "─") {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines
}

func isHeading(model Model, line string) bool {
	for _, row := range model.rows() {
		if row.Header != "" && row.Header == line {
			return true
		}
	}
	return false
}
