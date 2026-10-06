package rules

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestJobStateOf(t *testing.T) {
	cases := []struct {
		name     string
		params   JobStateOfParams
		want     domain.JobState
		reported bool
	}{
		{"a service running", JobStateOfParams{Status: domain.JobStatusRunning}, domain.JobStateRunning, true},
		{"a launcher still running", JobStateOfParams{Status: domain.JobStatusRunning, Detached: true}, domain.JobStateStarting, true},
		{"a stack the launcher left up", JobStateOfParams{Status: domain.JobStatusDetached, Detached: true}, domain.JobStateRunning, true},
		{"a crash", JobStateOfParams{Status: domain.JobStatusCrashed}, domain.JobStateCrashed, true},
		{"a stop", JobStateOfParams{Status: domain.JobStatusStopped}, domain.JobStateStopped, true},
		{"a job wtm took down after its daemon died", JobStateOfParams{Status: domain.JobStatusReaped}, domain.JobStateStopped, true},
		{"a claim", JobStateOfParams{Status: domain.JobStatusJoined}, "", false},
		{"a claim from an older daemon", JobStateOfParams{Status: domain.JobStatusLegacyAttached}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reported := JobStateOf(tc.params)
			if got != tc.want || reported != tc.reported {
				t.Fatalf("JobStateOf(%+v) = %q, %v; want %q, %v", tc.params, got, reported, tc.want, tc.reported)
			}
		})
	}
}

func TestLastLines(t *testing.T) {
	cases := []struct {
		name   string
		params LastLinesParams
		want   []string
	}{
		{"keeps the last ones", LastLinesParams{Text: "a\nb\nc\nd", Count: 2}, []string{"c", "d"}},
		{"keeps everything when short", LastLinesParams{Text: "a\nb", Count: 5}, []string{"a", "b"}},
		{"nothing for no output", LastLinesParams{Text: "", Count: 5}, nil},
		{"nothing for no count", LastLinesParams{Text: "a", Count: 0}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LastLines(tc.params); !slices.Equal(got, tc.want) {
				t.Fatalf("LastLines(%+v) = %q, want %q", tc.params, got, tc.want)
			}
		})
	}
}

func TestWorktreeJobsKeepsOnlyThatWorktreesReportedJobs(t *testing.T) {
	code, killed := 1, -1
	jobs := []domain.JobInfo{
		{Name: "web", Kind: domain.JobKindService, Status: domain.JobStatusRunning, WorkDir: "/a", URL: "http://web.a", State: domain.JobStateRunning},
		{Name: "api", Kind: domain.JobKindService, Status: domain.JobStatusCrashed, WorkDir: "/a", ExitCode: &code},
		{Name: "db", Kind: domain.JobKindService, Status: domain.JobStatusJoined, WorkDir: "/a"},
		{Name: "web", Kind: domain.JobKindService, Status: domain.JobStatusRunning, WorkDir: "/b"},
		{Name: "worker", Kind: domain.JobKindService, Status: domain.JobStatusStopped, WorkDir: "/a", ExitCode: &killed},
	}
	got := WorktreeJobs(WorktreeJobsParams{Path: "/a", Jobs: jobs})
	want := []domain.JobSnapshot{
		{Name: "api", Kind: domain.JobKindService, State: domain.JobStateCrashed, ExitCode: &code},
		{Name: "web", Kind: domain.JobKindService, State: domain.JobStateRunning, URL: "http://web.a"},
		{Name: "worker", Kind: domain.JobKindService, State: domain.JobStateStopped},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].State != want[i].State || got[i].URL != want[i].URL || got[i].ExitCode != want[i].ExitCode {
			t.Fatalf("job %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if none := WorktreeJobs(WorktreeJobsParams{Path: "/c", Jobs: jobs}); none == nil || len(none) != 0 {
		t.Fatalf("a worktree with no job reports %#v, want an empty list", none)
	}
}
