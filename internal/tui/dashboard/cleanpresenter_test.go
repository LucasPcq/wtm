package dashboard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	cleanflow "github.com/LucasPcq/wtm/internal/flow/clean"
)

func TestABatchCleanTagsEachStageWithItsWorktree(t *testing.T) {
	msgs := posted(func(send func(tea.Msg)) {
		p := newCleanPresenter(presenter{send: send, id: 1})
		p.WorktreeStarted(flow.Progress{Branch: "feat/a", Position: 1, Total: 2})
		_ = p.Stage(flow.StageParams{Message: "removing a", Work: func() error { return nil }})
		p.WorktreeStarted(flow.Progress{Branch: "feat/b", Position: 2, Total: 2})
		_ = p.Stage(flow.StageParams{Message: "removing b", Work: func() error { return nil }})
	})

	targets := map[string]string{}
	for _, msg := range msgs {
		if stage, ok := msg.(opStageMsg); ok {
			targets[stage.stage] = stage.target
		}
	}
	if targets["removing a"] != "feat/a" || targets["removing b"] != "feat/b" {
		t.Errorf("stage targets = %v, want each stage on the worktree being removed", targets)
	}
}

func TestABatchCleanReloadsAsEachWorktreeGoesAndEndsOnATally(t *testing.T) {
	msgs := posted(func(send func(tea.Msg)) {
		p := newCleanPresenter(presenter{send: send, id: 1})
		p.WorktreeCleaned(domain.CleanResult{Branch: "feat/a"})
		p.WorktreeFailed(domain.BatchFailure{Branch: "feat/b", Error: "locked"})
		_ = p.Cleaned(cleanflow.Outcome{
			Results: []domain.CleanResult{{Branch: "feat/a"}},
			Failed:  []domain.BatchFailure{{Branch: "feat/b", Error: "locked"}},
		})
	})

	reloads := 0
	for _, msg := range msgs {
		if _, ok := msg.(cleanedMsg); ok {
			reloads++
		}
	}
	if reloads < 2 {
		t.Errorf("reloads = %d, want the list refreshed as soon as feat/a is gone, then at the end", reloads)
	}
	text := outputText(msgs)
	for _, want := range []string{"feat/a", "feat/b: locked", "1 removed", "1 failed"} {
		if !strings.Contains(text, want) {
			t.Errorf("output %q should contain %q", text, want)
		}
	}
}

func TestASingleCleanReadsAsItAlwaysDid(t *testing.T) {
	msgs := posted(func(send func(tea.Msg)) {
		_ = newCleanPresenter(presenter{send: send, id: 1}).Cleaned(cleanflow.Outcome{Results: []domain.CleanResult{{Branch: "feat/a"}}})
	})
	text := outputText(msgs)
	if strings.Contains(text, "removed") || !strings.Contains(text, "feat/a") {
		t.Errorf("output %q, want the single finished line, not a tally", text)
	}
}

func TestTheGlobalMenuOffersToDeleteSeveralWorktrees(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	for _, item := range model.globalMenuItems() {
		if item.action == menuDeleteBatch {
			if !item.danger {
				t.Error("deleting worktrees must read as destructive")
			}
			return
		}
	}
	t.Error("the global menu should offer to delete several worktrees")
}

// The dashboard cannot hand its terminal to sudo, so a batch member git could
// not remove names the way out, as a single clean does.
func TestABatchFailureThatNeedsSudoNamesTheWayOut(t *testing.T) {
	msgs := posted(func(send func(tea.Msg)) {
		newCleanPresenter(presenter{send: send, id: 1}).WorktreeFailed(domain.BatchFailure{Branch: "feat/b", Error: "permission denied", Privileged: true})
	})
	if text := outputText(msgs); !strings.Contains(text, "wtm clean feat/b --force") {
		t.Errorf("output %q should name the privileged removal", text)
	}
}
