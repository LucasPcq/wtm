package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// The addresses a surface caches are read off the ports and the published url,
// so a port edited without a rename is a change like any other.
func TestSameRunJobsSeesAPortMoveWithNoRename(t *testing.T) {
	before := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Ports: map[string]int{"PORT": 3000}}}}
	after := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Ports: map[string]int{"PORT": 4000}}}}

	if SameRunJobs(before, after) {
		t.Fatal("a job binding another port is not the same job to a surface showing its address")
	}
	if !SameRunJobs(before, before) {
		t.Fatal("the same config read twice is the same config")
	}
}

func TestSameRunJobsSeesAPublishedURLAppearAndVanish(t *testing.T) {
	plain := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	published := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", URL: &domain.JobURLConfig{Port: "PORT"}}}}

	if SameRunJobs(plain, published) || SameRunJobs(published, plain) {
		t.Fatal("a job that starts publishing a name changes what the panel shows")
	}
	if !SameRunJobs(published, published) {
		t.Fatal("the same published job is the same job")
	}
}

func TestBranchesWithJobsUpNamesOnlyWhatRuns(t *testing.T) {
	branches := BranchesWithJobsUp(BranchesWithJobsUpParams{
		Jobs: []domain.JobInfo{
			{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/b"},
			{Name: "api", Status: domain.JobStatusRunning, WorkDir: "/wt/a"},
			{Name: "old", Status: domain.JobStatusStopped, WorkDir: "/wt/idle"},
		},
		Statuses: []domain.WorktreeStatus{
			{Branch: "feat/b", Path: "/wt/b"},
			{Branch: "feat/idle", Path: "/wt/idle"},
			{Branch: "feat/a", Path: "/wt/a"},
			{Branch: "", Path: "/wt/detached"},
		},
	})

	if len(branches) != 2 {
		t.Fatalf("branches = %v, want only the two running something", branches)
	}
	if branches[0] != "feat/a" || branches[1] != "feat/b" {
		t.Errorf("branches = %v, want them sorted so two polls ask for the same thing", branches)
	}
}

// The run flows answer with paths, and every surface keyed on branches has to be
// able to recognise what they answered.
func TestBranchesForPathsNamesWhatTheStatusesKnow(t *testing.T) {
	statuses := []domain.WorktreeStatus{
		{Branch: "main", Path: "/wt/main"},
		{Branch: "feat", Path: "/wt/feat"},
	}

	named := BranchesForPaths(BranchesForPathsParams{
		Paths:    []string{"/wt/feat", "/wt/unknown"},
		Statuses: statuses,
	})

	if len(named) != 2 || named[0] != "feat" {
		t.Fatalf("named = %v, want the branch of the path that is known", named)
	}
	// A worktree nothing names keeps the only identity it has: dropping it would
	// release its lock instead of moving it.
	if named[1] != "/wt/unknown" {
		t.Errorf("named = %v, want the unknown path left as it is", named)
	}
}

// `run ps` reads one worktree at a time: the daemon's order scattered a
// worktree's jobs between another's.
func TestJobsByWorktreeGroupsEachWorktreeAndOrdersItsJobs(t *testing.T) {
	jobs := []domain.JobInfo{
		{Name: "web", WorkDir: "/repo.trees/feat"},
		{Name: "web", WorkDir: "/repo"},
		{Name: "api", WorkDir: "/repo.trees/feat"},
		{Name: "api", WorkDir: "/repo"},
	}

	sorted := JobsByWorktree(jobs)

	var got []string
	for _, job := range sorted {
		got = append(got, job.WorkDir+" "+job.Name)
	}
	want := []string{"/repo api", "/repo web", "/repo.trees/feat api", "/repo.trees/feat web"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v", got, want)
	}
	if jobs[0].WorkDir != "/repo.trees/feat" {
		t.Error("the caller's slice was reordered")
	}
}
