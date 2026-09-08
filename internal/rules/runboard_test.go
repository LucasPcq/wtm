package rules

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestRunBoardGroupsByWorktreeAndSkipsTheIdleOnes(t *testing.T) {
	now := time.Now()
	board := RunBoard(RunBoardParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "worker"}}},
		Jobs: []domain.JobInfo{
			{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/a", StartedAt: now.Add(-time.Minute)},
			{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/b", StartedAt: now.Add(-time.Hour)},
		},
		Addresses: map[string]map[string]domain.JobAddress{
			"feat/a": {"web": {URL: "http://a.wtm"}},
			"feat/b": {"web": {URL: "http://b.wtm"}},
		},
		Statuses: []domain.WorktreeStatus{
			{Branch: "feat/a", Path: "/wt/a"},
			{Branch: "feat/idle", Path: "/wt/idle"},
			{Branch: "feat/b", Path: "/wt/b"},
		},
		Now: now,
	})

	if len(board) != 2 {
		t.Fatalf("blocks = %d, want only the worktrees running something", len(board))
	}
	if board[0].Branch != "feat/a" || board[1].Branch != "feat/b" {
		t.Errorf("branches = %q, %q, want the statuses' own order", board[0].Branch, board[1].Branch)
	}
	if board[0].Up != 1 {
		t.Errorf("Up = %d, want 1", board[0].Up)
	}
	if len(board[0].Rows) != 1 {
		t.Fatalf("rows = %d, want only what is up: this board answers what runs", len(board[0].Rows))
	}
	if board[0].Rows[0].URL != "http://a.wtm" {
		t.Errorf("URL = %q, want feat/a's own — a job of the same name elsewhere is not this one", board[0].Rows[0].URL)
	}
	if board[0].Path != "/wt/a" {
		t.Errorf("Path = %q, want the worktree's own", board[0].Path)
	}
}

func TestRunBoardIsEmptyWhenNothingRuns(t *testing.T) {
	if got := RunBoard(RunBoardParams{
		Config:   domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
		Statuses: []domain.WorktreeStatus{{Branch: "feat/a", Path: "/wt/a"}},
		Now:      time.Now(),
	}); len(got) != 0 {
		t.Errorf("board = %v, want empty", got)
	}
}

// A job the daemon holds up but run.toml no longer declares has no row to
// draw: the board reads the declaration, like the detail panel's section.
func TestRunBoardSkipsAJobTheConfigNoLongerDeclares(t *testing.T) {
	now := time.Now()
	board := RunBoard(RunBoardParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
		Jobs: []domain.JobInfo{
			{Name: "gone", Status: domain.JobStatusRunning, WorkDir: "/wt/a", StartedAt: now},
		},
		Statuses: []domain.WorktreeStatus{{Branch: "feat/a", Path: "/wt/a"}},
		Now:      now,
	})

	if len(board) != 0 {
		t.Errorf("board = %+v, want no block: nothing declared is up here", board)
	}
}

func TestServicesRowsFlattensBlocksWithTheirHeaders(t *testing.T) {
	rows := ServicesRows([]RunWorktreeBlock{
		{Branch: "feat/a", Path: "/wt/a", Up: 2, Rows: []domain.DetailRow{{Key: "web"}, {Key: "api"}}},
		{Branch: "main", Path: "/wt/main", Up: 1, Rows: []domain.DetailRow{{Key: "pg"}}},
	})

	want := []domain.ServicesRowKind{
		domain.ServicesRowHeader, domain.ServicesRowJob, domain.ServicesRowJob,
		domain.ServicesRowGap,
		domain.ServicesRowHeader, domain.ServicesRowJob,
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d, want %d", len(rows), len(want))
	}
	for index, kind := range want {
		if rows[index].Kind != kind {
			t.Fatalf("row %d = %q, want %q", index, rows[index].Kind, kind)
		}
	}
	if rows[1].Branch != "feat/a" || rows[1].Path != "/wt/a" {
		t.Errorf("job row = %+v, want it to carry its worktree: the menu acts on it", rows[1])
	}
	if rows[0].Up != 2 {
		t.Errorf("header Up = %d, want 2", rows[0].Up)
	}
	if rows[1].Job.Key != "web" {
		t.Errorf("job row key = %q, want web", rows[1].Job.Key)
	}
}

func TestServicesRowsPutsNoGapBeforeTheFirstBlock(t *testing.T) {
	rows := ServicesRows([]RunWorktreeBlock{{Branch: "a", Rows: []domain.DetailRow{{Key: "web"}}}})

	if len(rows) != 2 || rows[0].Kind != domain.ServicesRowHeader {
		t.Errorf("rows = %+v, want a header then its job, with no leading gap", rows)
	}
}

// The Services tab is where a reader picks a URL to open, so what has to be
// said about those URLs closes the block that lists them.
func TestServicesRowsClosesABlockOnWhatItHasToSay(t *testing.T) {
	rows := ServicesRows([]RunWorktreeBlock{{
		Branch: "main", Path: "/wt/main", Up: 1,
		Rows: []domain.DetailRow{{Key: "web"}},
		Note: "main answers on its ports",
	}})

	want := []domain.ServicesRowKind{
		domain.ServicesRowHeader, domain.ServicesRowJob,
		domain.ServicesRowGap, domain.ServicesRowNote,
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d, want %d — one row per drawn line", len(rows), len(want))
	}
	for index, kind := range want {
		if rows[index].Kind != kind {
			t.Fatalf("row %d = %q, want %q", index, rows[index].Kind, kind)
		}
	}
	if rows[3].Note != "main answers on its ports" || rows[3].Branch != "main" {
		t.Errorf("note row = %+v, want the line and the worktree it belongs to", rows[3])
	}
}

func TestRunBoardCarriesEachWorktreesNote(t *testing.T) {
	blocks := RunBoard(RunBoardParams{
		Config:   domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}}},
		Jobs:     []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/main"}},
		Statuses: []domain.WorktreeStatus{{Branch: "main", Path: "/wt/main"}},
		Notes:    map[string]string{"main": "main answers on its ports"},
	})

	if len(blocks) != 1 || blocks[0].Note != "main answers on its ports" {
		t.Errorf("blocks = %+v, want the note of the worktree they describe", blocks)
	}
}

// A stop or a view over several worktrees is about what is standing: the board
// already knows which those are.
func TestRunningWorktreeDirsNamesTheBlocksThatRun(t *testing.T) {
	blocks := []RunWorktreeBlock{
		{Branch: "a", Path: "/wt/a", Up: 2},
		{Branch: "b", Path: "/wt/b", Up: 1},
	}

	if got := RunningWorktreeDirs(blocks); len(got) != 2 || got[0] != "/wt/a" || got[1] != "/wt/b" {
		t.Errorf("RunningWorktreeDirs = %v, want one entry per block, as git spells it", got)
	}
}

// The Services tab answers "what is running", and nothing else. A worktree with
// a stopped job, a crashed one, or a log directory full of what it ran last week
// has no block at all: it briefly gained one, and the tab turned into a list of
// every worktree the repository has ever run something in.
func TestRunBoardSkipsAWorktreeWithNothingUp(t *testing.T) {
	blocks := RunBoard(RunBoardParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "api"}}},
		Jobs: []domain.JobInfo{
			{Name: "web", Status: domain.JobStatusStopped, WorkDir: "/wt/idle"},
			{Name: "api", Status: domain.JobStatusCrashed, WorkDir: "/wt/idle"},
			{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/live"},
		},
		Statuses: []domain.WorktreeStatus{
			{Branch: "idle", Path: "/wt/idle"},
			{Branch: "live", Path: "/wt/live"},
		},
		Now: time.Now(),
	})

	if len(blocks) != 1 {
		t.Fatalf("blocks = %+v, want only the worktree with something up", blocks)
	}
	if blocks[0].Branch != "live" || blocks[0].Up != 1 {
		t.Errorf("block = %+v, want live with one job up", blocks[0])
	}
	if len(blocks[0].Rows) != 1 {
		t.Errorf("rows = %+v, want the running job alone", blocks[0].Rows)
	}
}

// A runner's addresses fold here as they do in the detail panel, and its
// children never count as jobs: they are one process, and the header counts
// processes.
func TestRunBoardFoldsARunnersAddressesAndCountsItOnce(t *testing.T) {
	params := RunBoardParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "dev", Runs: []string{"web", "api"}}}},
		Jobs:   []domain.JobInfo{{Name: "dev", Status: domain.JobStatusRunning, WorkDir: "/wt/live"}},
		Addresses: map[string]map[string]domain.JobAddress{"live": {"dev": {Held: []domain.JobURLEntry{
			{Job: "web", URL: "http://web.wtm"},
			{Job: "api", URL: "http://api.wtm"},
		}}}},
		Statuses: []domain.WorktreeStatus{{Branch: "live", Path: "/wt/live"}},
		Now:      time.Now(),
	}

	folded := RunBoard(params)
	if len(folded[0].Rows) != 1 {
		t.Fatalf("rows = %+v, want the runner alone while it is folded", folded[0].Rows)
	}

	params.Expanded = map[string]bool{"dev": true}
	open := RunBoard(params)
	if len(open[0].Rows) != 3 {
		t.Fatalf("rows = %+v, want the runner and one row per address", open[0].Rows)
	}
	if open[0].Up != 1 {
		t.Errorf("up = %d, want the runner counted once and not its addresses", open[0].Up)
	}
	if rows := ServicesRows(open); rows[2].Kind != domain.ServicesRowHeld {
		t.Errorf("child row kind = %q, want it drawn but not selectable", rows[2].Kind)
	}
}
