package dashboard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	pruneflow "github.com/LucasPcq/wtm/internal/flow/prune"
	downflow "github.com/LucasPcq/wtm/internal/flow/run/down"
	stopflow "github.com/LucasPcq/wtm/internal/flow/run/stop"
	syncflow "github.com/LucasPcq/wtm/internal/flow/sync"
)

// collect drains what a presenter posted, without a running model behind it.
func collect(t *testing.T, run func(send func(tea.Msg))) []string {
	t.Helper()
	msgs := make(chan tea.Msg, 32)
	run(func(msg tea.Msg) { msgs <- msg })
	close(msgs)

	var lines []string
	for msg := range msgs {
		if line, ok := msg.(OutputLineMsg); ok {
			lines = append(lines, line.Text)
		}
	}
	return lines
}

// A worktree the user checked and the safety re-gate then dropped must say so.
// Without it the run silently does less than what was confirmed, and the panel
// gives the user no way to notice.
func TestPrunePresenterReportsWhatWasSkipped(t *testing.T) {
	outcome := pruneflow.Outcome{Result: domain.PruneResult{
		Pruned:  []domain.PruneCandidate{{Branch: "merged-wt", Reason: domain.PruneReasonPRMerged}},
		Skipped: []domain.PruneSkip{{Branch: "dirty-wt", Reason: domain.PruneSkipDirty}},
	}}

	lines := collect(t, func(send func(tea.Msg)) {
		if err := (prunePresenter{presenter{send: send}}).Pruned(outcome); err != nil {
			t.Fatalf("Pruned: %v", err)
		}
	})

	body := strings.Join(lines, "\n")
	if !strings.Contains(body, "merged-wt") {
		t.Errorf("the removed worktree must be reported:\n%s", body)
	}
	if !strings.Contains(body, "dirty-wt") || !strings.Contains(body, domain.PruneLabelDirty) {
		t.Errorf("the skipped worktree must be reported with its reason:\n%s", body)
	}
}

func TestPrunePresenterSaysWhenNothingMatched(t *testing.T) {
	lines := collect(t, func(send func(tea.Msg)) {
		if err := (prunePresenter{presenter{send: send}}).Pruned(pruneflow.Outcome{Empty: true}); err != nil {
			t.Fatalf("Pruned: %v", err)
		}
	})

	if len(lines) != 1 || lines[0] != domain.PruneNothingToPrune {
		t.Errorf("lines = %v, want the empty run to say so once", lines)
	}
}

// Keeping a conflict is offered here as it is on the CLI, so the panel owes the
// user the way out: the worktree it was left in, and the two commands that end it.
func TestSyncPresenterNamesTheWayOutOfAKeptConflict(t *testing.T) {
	outcome := syncflow.Outcome{Result: domain.SyncResult{Steps: []domain.SyncStepResult{{
		Branch:         "feat-a",
		Path:           "/tmp/trees/feat-a",
		Status:         domain.SyncStatusConflict,
		KeptInProgress: true,
	}}}}

	lines := collect(t, func(send func(tea.Msg)) {
		if err := (syncPresenter{presenter{send: send}}).Synced(outcome); err != nil {
			t.Fatalf("Synced: %v", err)
		}
	})

	body := strings.Join(lines, "\n")
	if !strings.Contains(body, "/tmp/trees/feat-a") || !strings.Contains(body, "rebase --continue") {
		t.Errorf("a kept conflict must name its worktree and the way out:\n%s", body)
	}
}

func TestSyncPresenterSaysWhenNothingMatched(t *testing.T) {
	lines := collect(t, func(send func(tea.Msg)) {
		if err := (syncPresenter{presenter{send: send}}).Synced(syncflow.Outcome{Empty: true}); err != nil {
			t.Fatalf("Synced: %v", err)
		}
	})

	if len(lines) != 1 || lines[0] != domain.SyncNothingToSync {
		t.Errorf("lines = %v, want the empty run to say so once", lines)
	}
}

// A base refresh rebases nothing, so the per-step lines are empty: without the
// base line the user would watch a run report nothing at all.
func TestSyncPresenterReportsTheBaseAndEveryStep(t *testing.T) {
	result := domain.SyncResult{
		BaseBranch:   "main",
		BaseTargeted: true,
		BaseUpdated:  true,
		Steps: []domain.SyncStepResult{
			{Branch: "feat-a", Status: domain.SyncStatusSynced},
			{Branch: "feat-b", Status: domain.SyncStatusSkippedDirty},
		},
		ParentUpdates: []domain.ParentUpdate{{Branch: "dev", Status: domain.ParentFastForwarded}},
	}

	lines := collect(t, func(send func(tea.Msg)) {
		(syncPresenter{presenter{send: send}}).Rebased(result)
	})

	body := strings.Join(lines, "\n")
	for _, want := range []string{"main", domain.SyncBaseLabelFastForwarded, "feat-a", domain.SyncLabelSynced,
		"feat-b", domain.SyncLabelSkippedDirty, "dev", domain.SyncParentLabelFastForwarded} {
		if !strings.Contains(body, want) {
			t.Errorf("output missing %q:\n%s", want, body)
		}
	}
}

func TestSyncPresenterReportsWhatItPushed(t *testing.T) {
	outcome := syncflow.Outcome{Result: domain.SyncResult{Steps: []domain.SyncStepResult{
		{Branch: "feat-a", Status: domain.SyncStatusSynced, Pushed: true},
		{Branch: "feat-b", Status: domain.SyncStatusSynced},
	}}}

	lines := collect(t, func(send func(tea.Msg)) {
		if err := (syncPresenter{presenter{send: send}}).Synced(outcome); err != nil {
			t.Fatalf("Synced: %v", err)
		}
	})

	body := strings.Join(lines, "\n")
	if !strings.Contains(body, "feat-a") {
		t.Errorf("a pushed branch must be reported:\n%s", body)
	}
	if strings.Contains(body, "feat-b") {
		t.Errorf("a branch left local must not read as pushed:\n%s", body)
	}
}

// A run whose steps only say "failed" leaves the panel with no trace of why —
// and the run itself reports nothing more, having already said its piece.
func TestSyncPresenterNamesWhyAStepFailed(t *testing.T) {
	result := domain.SyncResult{Steps: []domain.SyncStepResult{
		{Branch: "feat-a", Status: domain.SyncStatusError, Detail: "could not read HEAD"},
		{Branch: "feat-b", Status: domain.SyncStatusConflict},
	}}

	lines := collect(t, func(send func(tea.Msg)) {
		(syncPresenter{presenter{send: send}}).Rebased(result)
	})

	body := strings.Join(lines, "\n")
	if !strings.Contains(body, "could not read HEAD") {
		t.Errorf("a failed step must name its cause:\n%s", body)
	}
	// An aborted conflict left nothing behind; saying so is what keeps the user
	// from going to look for a half-rebased worktree.
	if !strings.Contains(body, domain.SyncLabelConflictAborted) {
		t.Errorf("an aborted conflict must say the worktree was left clean:\n%s", body)
	}
}

// Two worktrees each stopping a `web` read as the same line twice: above one
// worktree the name would only repeat the row, above several it is the subject.
func TestAStopOverSeveralWorktreesNamesEach(t *testing.T) {
	results := []domain.WorktreeJobResults{
		{Branch: "feat/a", Path: "/wt/a", Jobs: []domain.JobActionResult{{Name: "web", Status: domain.JobActionStopped}}},
		{Branch: "feat/b", Path: "/wt/b", Jobs: []domain.JobActionResult{{Name: "web", Status: domain.JobActionError, Message: "refused"}}},
	}
	down := collect(t, func(send func(tea.Msg)) {
		if err := (downPresenter{presenter{send: send}}).Downed(downflow.Outcome{Results: results}); err != nil {
			t.Fatalf("Downed: %v", err)
		}
	})
	stop := collect(t, func(send func(tea.Msg)) {
		if err := (stopPresenter{presenter{send: send}}).Stopped(stopflow.Outcome{Results: results}); err != nil {
			t.Fatalf("Stopped: %v", err)
		}
	})
	for name, lines := range map[string][]string{"down": down, "stop": stop} {
		body := strings.Join(lines, "\n")
		if !strings.Contains(body, "web stopped · feat/a") || !strings.Contains(body, "web: refused · feat/b") {
			t.Errorf("%s = %q, want each line attributed to its worktree", name, body)
		}
	}

	one := collect(t, func(send func(tea.Msg)) {
		_ = (downPresenter{presenter{send: send}}).Downed(downflow.Outcome{Results: results[:1]})
	})
	if body := strings.Join(one, "\n"); strings.Contains(body, "feat/a") {
		t.Errorf("down = %q, want a single worktree left unnamed", body)
	}
}
