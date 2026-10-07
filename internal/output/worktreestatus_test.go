package output

import (
	"bytes"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func statusSet() []domain.StatusDocument {
	ten, twenty, zero := 10, 20, 0
	exit := 1
	service := func(name string, state domain.JobState) domain.JobSnapshot {
		return domain.JobSnapshot{Name: name, Kind: domain.JobKindService, State: state}
	}
	env := domain.StatusEnv{Declared: 2, Missing: []string{}}
	return []domain.StatusDocument{
		{Branch: "main", Main: true, Isolation: domain.IsolationIsolated, Offset: &zero, RunConfig: true, Env: env,
			Jobs: []domain.JobSnapshot{service("api", domain.JobStateStopped), service("web", domain.JobStateStopped)}, Problems: []domain.StatusProblem{}},
		{Branch: "feat/login", Isolation: domain.IsolationIsolated, Offset: &ten, RunConfig: true, Env: env,
			Jobs: []domain.JobSnapshot{service("api", domain.JobStateRunning), service("web", domain.JobStateRunning)}, Problems: []domain.StatusProblem{}},
		{Branch: "feat/payments", Isolation: domain.IsolationIsolated, Offset: &twenty, RunConfig: true,
			Env:  domain.StatusEnv{Declared: 2, Missing: []string{"apps/web/.env"}},
			Jobs: []domain.JobSnapshot{{Name: "api", Kind: domain.JobKindService, State: domain.JobStateCrashed, ExitCode: &exit}, service("web", domain.JobStateRunning)},
			Problems: []domain.StatusProblem{
				{Code: domain.StatusProblemEnvMissing, Message: "apps/web/.env is missing", Fix: "wtm env feat/payments --yes"},
				{Code: domain.StatusProblemJobCrashed, Message: "api crashed (exit 1)", Fix: "wtm run start feat/payments --job api -d --yes"},
			}},
		{Branch: "fix/legacy", Isolation: domain.IsolationIsolated, RunConfig: true, Env: env,
			Jobs:     []domain.JobSnapshot{service("api", domain.JobStateStopped), service("web", domain.JobStateStopped)},
			Problems: []domain.StatusProblem{{Code: domain.StatusProblemIsolationPending, Message: "predates the isolation choice: run up and run start refuse it", Fix: "wtm env fix/legacy --isolation isolated --yes"}}},
		{Branch: "docs/readme", Isolation: domain.IsolationVerbatim, Offset: &zero, RunConfig: true, Env: env,
			Jobs: []domain.JobSnapshot{service("api", domain.JobStateStopped), service("web", domain.JobStateStopped)}, Problems: []domain.StatusProblem{}},
	}
}

const statusAllWant = `  ! 5 worktrees · 2 need attention

     WORKTREE       ISOLATION   PORTS     JOBS                   ENV
     main           isolated    base      2 stopped              2 files
     feat/login     isolated    +10       2 running              2 files
  !  feat/payments  isolated    +20       1 running · 1 crashed  1 missing
  !  fix/legacy     not chosen  —         2 stopped              2 files
     docs/readme    verbatim    source's  2 stopped              2 files

  feat/payments
  ! apps/web/.env is missing
  → wtm env feat/payments --yes
  ! api crashed (exit 1)
  → wtm run start feat/payments --job api -d --yes

  fix/legacy
  ! predates the isolation choice: run up and run start refuse it
  → wtm env fix/legacy --isolation isolated --yes
`

func TestStatusAllIsATableThenEachProblemUnderItsWorktree(t *testing.T) {
	var out bytes.Buffer
	FormatStatusAll(&out, statusSet())

	if out.String() != statusAllWant {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), statusAllWant)
	}
}

func TestStatusAllWithoutRunTomlKeepsOnlyTheEnvColumn(t *testing.T) {
	docs := statusSet()[:3]
	for i := range docs {
		docs[i].RunConfig, docs[i].Jobs, docs[i].Offset = false, []domain.JobSnapshot{}, nil
		docs[i].Problems = docs[i].Problems[:min(len(docs[i].Problems), 1)]
	}
	var out bytes.Buffer
	FormatStatusAll(&out, docs)

	want := `  ! 3 worktrees · 1 needs attention

     WORKTREE       ENV
     main           2 files
     feat/login     2 files
  !  feat/payments  1 missing

  feat/payments
  ! apps/web/.env is missing
  → wtm env feat/payments --yes
`
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestStatusAllOfHealthyWorktreesConcludesOnNothingToFix(t *testing.T) {
	docs := []domain.StatusDocument{statusSet()[0], statusSet()[1]}
	var out bytes.Buffer
	FormatStatusAll(&out, docs)

	if got := out.String(); got[:len("  = 2 worktrees — nothing to fix\n")] != "  = 2 worktrees — nothing to fix\n" {
		t.Errorf("got:\n%s", got)
	}
}

func TestOneWorktreeReadsItsPortsAndAChoiceItNeverMade(t *testing.T) {
	var out bytes.Buffer
	FormatStatus(&out, FormatStatusParams{Document: statusSet()[3], ProjectDir: "/repo"})

	for _, want := range []string{"! fix/legacy — 1 problem\n", "isolation  not chosen\n", "ports      —\n", "env        2 files\n"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Errorf("lacks %q:\n%s", want, out.String())
		}
	}
}
