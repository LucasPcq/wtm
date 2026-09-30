package owed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

// fixture is a repository with a shared postgres whose remove leaves a
// witness, a live linked worktree, and two debts: one for that worktree —
// re-created since the clean — and one for a worktree that is gone.
func fixture(t *testing.T) (flow.Context, string) {
	t.Helper()
	globaldir.Isolate(t)
	repo := gittest.InitRepo(t)
	stateDir := filepath.Join(repo, ".git", "wtm")
	gittest.Git(t, repo, "worktree", "add", "-b", "feat-live", filepath.Join(t.TempDir(), "feat-live"))
	witness := filepath.Join(t.TempDir(), "witness")

	if err := config.WriteRun(config.WriteRunParams{StateDir: stateDir, Force: true, Config: domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "postgres", Kind: domain.JobKindService, Cmd: "true", Scope: domain.JobScopeShared,
		Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "true", Remove: "echo $WTM_NAMESPACE >> " + witness},
	}}}}); err != nil {
		t.Fatal(err)
	}
	if err := runjobs.QueueRemovals(runjobs.QueueRemovalsParams{StateDir: stateDir, Refs: []domain.NamespaceRef{
		{Job: "postgres", Worktree: "feat-live", Ordinal: 1},
		{Job: "postgres", Worktree: "gone", Ordinal: 2},
	}}); err != nil {
		t.Fatal(err)
	}
	return flow.Context{ProjectDir: repo, StateDir: stateDir}, witness
}

// A debt whose worktree exists again names the new worktree's namespace:
// paying it would drop that worktree's data. It is withdrawn, never paid.
func TestSettleWithdrawsADebtWhoseWorktreeExistsAgain(t *testing.T) {
	ctx, witness := fixture(t)
	presenter := &flowtest.Recorder{}

	result := Settle(Params{Context: ctx, Presenter: presenter})

	if body, _ := os.ReadFile(witness); len(body) != 0 {
		t.Errorf("remove ran with %q, want nothing dropped", body)
	}
	left := runjobs.LoadPendingRemovals(ctx.StateDir)
	if len(left) != 1 || left[0].Worktree != "gone" {
		t.Errorf("queue = %+v, want only the gone worktree's debt", left)
	}
	if len(presenter.Statuses) != 1 || !strings.Contains(presenter.Statuses[0].Text, "exists again") {
		t.Errorf("statuses = %+v, want the withdrawal said", presenter.Statuses)
	}
	if result.Owed["postgres"] != 1 {
		t.Errorf("owed = %+v, want the gone worktree's debt still owed", result.Owed)
	}
}

// A service that is down leaves its debt where it is, and counted.
func TestSettleLeavesADebtOwedWhileItsServiceIsDown(t *testing.T) {
	ctx, _ := fixture(t)

	Settle(Params{Context: ctx, Presenter: &flowtest.Recorder{}})
	result := Settle(Params{Context: ctx, Presenter: &flowtest.Recorder{}})

	if len(result.Settled) != 0 || result.Owed["postgres"] != 1 {
		t.Errorf("result = %+v, want nothing paid and one still owed", result)
	}
}
