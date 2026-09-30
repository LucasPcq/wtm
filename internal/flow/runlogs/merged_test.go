package runlogs_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

const otherWorkDir = "/work/feature"

func worktreeBoard(t *testing.T, service *runlogstest.Service, dir, branch string, jobs ...domain.JobConfig) runlogs.MergedEntry {
	t.Helper()
	board := runlogs.NewBoard(runlogs.BoardParams{
		Service:  service,
		Jobs:     jobs,
		WorkDir:  dir,
		Worktree: branch,
		LogDir:   "/state/logs/" + branch,
	})
	if err := board.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return runlogs.MergedEntry{WorkDir: dir, Board: board}
}

func TestMergedBoardKeepsEachWorktreesJobsApart(t *testing.T) {
	service := &runlogstest.Service{Infos: []domain.JobInfo{
		running("api", domain.JobKindService),
		{Name: "api", Kind: domain.JobKindService, Status: domain.JobStatusRunning, WorkDir: otherWorkDir},
	}}

	merged := runlogs.NewMergedBoard([]runlogs.MergedEntry{
		worktreeBoard(t, service, workDir, "main", api),
		worktreeBoard(t, service, otherWorkDir, "feature", api),
	})

	views := merged.Jobs()
	if len(views) != 2 {
		t.Fatalf("merged board shows %d jobs, want one per worktree", len(views))
	}
	if views[0].WorkDir != workDir || views[0].Worktree != "main" {
		t.Fatalf("first row reads as %+v", views[0])
	}
	if views[1].WorkDir != otherWorkDir || views[1].Worktree != "feature" {
		t.Fatalf("second row reads as %+v", views[1])
	}
	if views[0].Name != views[1].Name {
		t.Fatal("the two rows should share a name — that is the whole point of the worktree axis")
	}
}

func TestMergedBoardOfOneIsTheBoardItself(t *testing.T) {
	service := &runlogstest.Service{}
	entry := worktreeBoard(t, service, workDir, "main", api)

	if merged := runlogs.NewMergedBoard([]runlogs.MergedEntry{entry}); merged != entry.Board {
		t.Fatal("a merged board of one should be the board itself, not a routing layer")
	}
}

func TestMergedBoardRoutesHistoryToTheNamedWorktree(t *testing.T) {
	service := &runlogstest.Service{
		Infos: []domain.JobInfo{running("api", domain.JobKindService)},
		Lines: map[string][]string{"api": {"hello"}},
	}

	merged := runlogs.NewMergedBoard([]runlogs.MergedEntry{
		worktreeBoard(t, service, workDir, "main", api),
		worktreeBoard(t, service, otherWorkDir, "feature", api),
	})

	if _, err := merged.History(runlogs.HistoryParams{Job: "api", WorkDir: otherWorkDir}); err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(service.Tailed) != 1 {
		t.Fatalf("tailed %d times, want one", len(service.Tailed))
	}
	if service.Tailed[0].LogDir != "/state/logs/feature" {
		t.Fatalf("read the history of %q, want the named worktree's", service.Tailed[0].LogDir)
	}
}

func TestMergedBoardRefusesAnUnknownWorktree(t *testing.T) {
	service := &runlogstest.Service{Infos: []domain.JobInfo{running("api", domain.JobKindService)}}
	merged := runlogs.NewMergedBoard([]runlogs.MergedEntry{
		worktreeBoard(t, service, workDir, "main", api),
		worktreeBoard(t, service, otherWorkDir, "feature", api),
	})

	if _, err := merged.History(runlogs.HistoryParams{Job: "api", WorkDir: "/work/nowhere"}); err == nil {
		t.Fatal("an unnamed worktree must be refused, never defaulted to the first")
	}
}
