package worktree

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// The probes a prune plan reads fail as "nothing to report": an interrupt
// during the scan must end it, not hand back a plan whose safety checks never ran.
func TestAPrunePlanCutShortByAnInterruptIsNoPlan(t *testing.T) {
	source := gittest.InitRepo(t)
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat-merged", filepath.Join(t.TempDir(), "feat-merged"), "HEAD")
	ctx, cancel := context.WithCancel(t.Context())
	params := domain.PruneParams{
		ProjectDir: source,
		StateDir:   t.TempDir(),
		Config:     domain.Config{Project: domain.ProjectConfig{Worktrees: domain.WorktreesConfig{BaseBranch: "main"}}},
		BaseBranch: "main",
		Merged:     true,
	}

	plan, err := PlanPrune(ctx, PlanPruneParams{Prune: params, PRs: func() []domain.PRInfo {
		cancel()
		return []domain.PRInfo{{Branch: "feat-merged", State: domain.PRStateMerged}}
	}})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, plan = %+v: want the interrupt, not a plan", err, plan)
	}
}
