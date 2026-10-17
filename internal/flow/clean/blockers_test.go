package clean

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

// The delete step states its refusals twice on purpose: as prose for a surface
// that can only print, and as blockers for one that can have each of them lifted.
func TestDeleteStepStatesEachRefusalOnItsOwn(t *testing.T) {
	f := &cleanFlow{runCtx: t.Context(),
		request: Request{Branches: []string{"feat"}},
		checks: map[string]domain.CleanCheckEntry{
			"feat": {Check: domain.CleanCheckResult{
				Branch:          "feat",
				WorktreePath:    "/w/feat",
				IsDirty:         true,
				UnpushedCommits: 2,
				HasOpenPR:       true,
				PRUrl:           "http://pr/1",
			}},
		},
	}

	content, err := f.deleteStep().Build(answers(map[string]string{KeyWorktree: "feat"}))
	if err != nil {
		t.Fatalf("build the delete step: %v", err)
	}

	if len(content.Blockers) != 3 {
		t.Fatalf("got %d blockers, want one per refusal: %+v", len(content.Blockers), content.Blockers)
	}
	for _, blocker := range content.Blockers {
		if !strings.Contains(content.Description, blocker.Label) {
			t.Errorf("blocker %q is missing from the printed recap — the two must say the same thing", blocker.Key)
		}
	}
	if !hasDanger(content.Options) {
		t.Error("a blocked removal must still offer the option the blockers stand in the way of")
	}
}

func TestDeleteStepOfBlockedNothingHasNoBlockers(t *testing.T) {
	f := &cleanFlow{runCtx: t.Context(),
		request: Request{Branches: []string{"feat"}},
		checks: map[string]domain.CleanCheckEntry{
			"feat": {Check: domain.CleanCheckResult{Branch: "feat", WorktreePath: "/w/feat"}},
		},
	}

	content, err := f.deleteStep().Build(answers(map[string]string{KeyWorktree: "feat"}))
	if err != nil {
		t.Fatalf("build the delete step: %v", err)
	}

	if len(content.Blockers) != 0 {
		t.Errorf("blockers = %+v, want none for a clean worktree", content.Blockers)
	}
	if hasDanger(content.Options) {
		t.Error("nothing to lift, so nothing dangerous to offer")
	}
}

func hasDanger(options []flow.Option) bool {
	for _, option := range options {
		if option.Danger {
			return true
		}
	}
	return false
}

func TestOperationDeclaresHowTheRunHoldsItsSurface(t *testing.T) {
	op := Operation()
	if op.Mode != flow.ModeBlocking {
		t.Errorf("clean mode = %v, want blocking: it destroys the worktree it targets", op.Mode)
	}
	if op.TargetKey != KeyWorktree {
		t.Errorf("target key = %q, want the answer naming the removed worktree", op.TargetKey)
	}
	if op.Kind != domain.OpKindClean {
		t.Errorf("kind = %q, want %q", op.Kind, domain.OpKindClean)
	}
}
