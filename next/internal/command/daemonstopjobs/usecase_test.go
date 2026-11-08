package daemonstopjobs_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstopjobs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEveryJobOfEveryRepositoryStopsASharedOneLetGoByEachHold(t *testing.T) {
	d := running(web, db, dbHeld, stack, api, worker, docs)

	plan, _, err := usecase(d, operations()).Plan(t.Context(), daemonstopjobs.Request{})

	require.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"main (acme)":   {"web:stop", "db:stop:held", "mail:stop", "api:clear"},
		"feat/a (acme)": {"db:stop:held", "worker:stop"},
		"main (wtm)":    {"docs:stop"},
	}, worktrees(plan))
	assert.Equal(t, []string{"/src/acme", "/src/wtm"}, []string{plan.Repos[0].Root, plan.Repos[1].Root})
	assert.Equal(t, []string{"feat/a"}, plan.Repos[0].Worktrees[0].Jobs[1].HeldBy, "a shared service names the worktrees that lose it")
}

func TestASharedServiceItsMainCheckoutHoldsStopsAsItsHold(t *testing.T) {
	held := db
	held.MainHolds = true

	plan, _, err := usecase(running(held, dbHeld), operations()).Plan(t.Context(), daemonstopjobs.Request{})

	require.NoError(t, err)
	assert.Equal(t, map[string][]string{"main (acme)": {"db:stop:held"}, "feat/a (acme)": {"db:stop:held"}}, worktrees(plan))
}

func TestARepositoryNamedByAPathInItNarrowsTheStop(t *testing.T) {
	plan, _, err := usecase(running(web, docs), operations()).Plan(t.Context(), daemonstopjobs.Request{Repo: "/src/acme/apps"})

	require.NoError(t, err)
	assert.Equal(t, "/src/acme", plan.Repo)
	assert.Equal(t, map[string][]string{"main (acme)": {"web:stop"}}, worktrees(plan))
	assert.Equal(t, "repo", plan.Recap[0].Key, "the recap names the repository it narrows to")
}

func TestAPathInNoRepositoryIsAProblemOfItsField(t *testing.T) {
	_, _, err := usecase(running(web), operations()).Plan(t.Context(), daemonstopjobs.Request{Repo: "/tmp/x"})

	refused, ok := err.(*domain.Error)
	require.True(t, ok, err)
	require.Len(t, refused.Fields, 1)
	assert.Equal(t, domain.PathRepo, refused.Fields[0].Path)
}

func TestNothingRunningIsAnEmptyPlan(t *testing.T) {
	plan, _, err := usecase(running(), operations()).Plan(t.Context(), daemonstopjobs.Request{})

	require.NoError(t, err)
	assert.True(t, plan.Empty)
}

func TestADaemonWhoseJobsCannotBeReadIsRefused(t *testing.T) {
	d := running(web)
	d.status.Skew = run.SkewProtocol

	_, _, err := usecase(d, operations()).Plan(t.Context(), daemonstopjobs.Request{})

	refused, ok := err.(*domain.Error)
	require.True(t, ok, err)
	assert.Equal(t, daemonstopjobs.CodeDaemonSkew, refused.Code)
}

func TestAWorktreeThatDidNotStopIsFollowedByTheSameStopAgain(t *testing.T) {
	d := running(web, stack, docs)
	d.refuse = map[string]error{"mail /src/acme": errStack}

	outcome, err := usecase(d, operations()).Run(t.Context(), daemonstopjobs.Request{Repo: "/src/acme"}, domain.RunOptions{})

	require.NoError(t, err)
	require.Len(t, outcome.Result.Items, 1)
	assert.Equal(t, domain.StatusFailed, outcome.Result.Items[0].Status)
	assert.Equal(t, []domain.FollowUp{{Command: daemonstopjobs.Name, Answers: []domain.Answered{{Path: domain.PathRepo, Values: []string{"/src/acme"}}}, Code: daemonstopjobs.CodeAgain, Message: "stop the ones left"}}, outcome.Result.FollowUps)
	assert.ElementsMatch(t, []string{"web /src/acme", "mail /src/acme"}, d.stopped)
}

func TestARelativeRepositoryIsReadFromWhereTheCallerStands(t *testing.T) {
	plan, _, err := usecase(running(web, docs), operations()).Plan(t.Context(), daemonstopjobs.Request{Repo: "apps", Caller: domain.Caller{Cwd: "/src/acme"}})

	require.NoError(t, err)
	assert.Equal(t, "/src/acme", plan.Repo)
}

func TestAClaimRecordedForNoRepositoryStopsWithItsWorktreesRepository(t *testing.T) {
	claim := dbHeld
	claim.Repo, claim.Branch = "", ""

	plan, _, err := usecase(running(db, claim, worker), operations()).Plan(t.Context(), daemonstopjobs.Request{Repo: "/src/acme"})

	require.NoError(t, err)
	assert.Equal(t, map[string][]string{"main (acme)": {"db:stop:held"}, "feat/a (acme)": {"db:stop:held", "worker:stop"}}, worktrees(plan))
}
