package rules

import (
	"reflect"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestStatusJobsListsEveryDeclaredJobWithItsState(t *testing.T) {
	code := 1
	got := StatusJobs(StatusJobsParams{
		Declared: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService},
			{Name: "worker", Kind: domain.JobKindService},
			{Name: "migrate", Kind: domain.JobKindTask},
		},
		Up: []domain.JobSnapshot{
			{Name: "worker", Kind: domain.JobKindService, State: domain.JobStateCrashed, ExitCode: &code},
			{Name: "api", Kind: domain.JobKindService, State: domain.JobStateRunning, URL: "http://daemon"},
			{Name: "legacy", Kind: domain.JobKindService, State: domain.JobStateRunning},
		},
		URLs: map[string]string{"api": "http://localhost:3091"},
	})

	want := []domain.JobSnapshot{
		{Name: "api", Kind: domain.JobKindService, State: domain.JobStateRunning, URL: "http://localhost:3091"},
		{Name: "worker", Kind: domain.JobKindService, State: domain.JobStateCrashed, ExitCode: &code},
		{Name: "migrate", Kind: domain.JobKindTask, State: domain.JobStateStopped},
		{Name: "legacy", Kind: domain.JobKindService, State: domain.JobStateRunning},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StatusJobs =\n%+v\nwant\n%+v", got, want)
	}
}

func TestEveryStatusProblemCarriesTheCommandThatClearsIt(t *testing.T) {
	code := 2
	got := StatusProblems(StatusProblemsParams{
		Branch: "feat/x",
		MissingEnv: []domain.EnvMissingFile{
			{Target: "apps/e2e/.env", Scaffolded: true, HasTemplate: true},
			{Target: "apps/web/.env", HasTemplate: true},
			{Target: "apps/doc/.env"},
		},
		IsolationPending: true,
		Jobs: []domain.JobSnapshot{
			{Name: "api", State: domain.JobStateRunning},
			{Name: "worker", State: domain.JobStateCrashed, ExitCode: &code},
		},
	})

	want := []domain.StatusProblem{
		{Code: domain.StatusProblemEnvMissing, Message: "apps/e2e/.env is missing", Fix: "wtm env feat/x --yes"},
		{Code: domain.StatusProblemEnvMissing, Message: "apps/web/.env is missing", Fix: "wtm env feat/x --from example --yes"},
		{Code: domain.StatusProblemEnvMissing, Message: "apps/doc/.env is missing, with no copy to rebuild it from and no template: declare it where it exists, or write it by hand", Fix: "wtm config edit"},
		{Code: domain.StatusProblemIsolationPending, Message: "feat/x predates isolation: run up and run start refuse it until its isolation is chosen", Fix: "wtm env feat/x --isolation isolated --yes"},
		{Code: domain.StatusProblemJobCrashed, Message: "worker crashed (exit 2)", Fix: "wtm run start feat/x --job worker -d --yes"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StatusProblems =\n%+v\nwant\n%+v", got, want)
	}
}

func TestAHealthyWorktreeHasAnEmptyProblemList(t *testing.T) {
	got := StatusProblems(StatusProblemsParams{Branch: "main"})
	if got == nil || len(got) != 0 {
		t.Errorf("problems = %#v, want an empty list, never null", got)
	}
}

func TestAJobKilledByASignalIsNotGivenItsSentinelCode(t *testing.T) {
	code := -1
	got := StatusProblems(StatusProblemsParams{Branch: "b", Jobs: []domain.JobSnapshot{{Name: "api", State: domain.JobStateCrashed, ExitCode: &code}}})
	if len(got) != 1 || got[0].Message != "api crashed (killed by a signal)" {
		t.Errorf("problems = %+v", got)
	}
}

func TestAFixQuotesTheNamesItCarries(t *testing.T) {
	got := StatusProblems(StatusProblemsParams{
		Branch:     "feat;touch$IFS/tmp/p",
		MissingEnv: []domain.EnvMissingFile{{Target: ".env", Scaffolded: true}},
		Jobs:       []domain.JobSnapshot{{Name: "api", State: domain.JobStateCrashed}},
	})
	want := []string{
		"wtm env 'feat;touch$IFS/tmp/p' --yes",
		"wtm run start 'feat;touch$IFS/tmp/p' --job api -d --yes",
	}
	if len(got) != 2 || got[0].Fix != want[0] || got[1].Fix != want[1] {
		t.Errorf("fixes = %+v, want %q", got, want)
	}
}
