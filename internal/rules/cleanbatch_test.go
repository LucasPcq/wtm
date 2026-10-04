package rules

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestCleanBatchBlockersKeyEachRefusalByWorktree(t *testing.T) {
	blockers := CleanBatchBlockers([]domain.CleanCheckResult{
		{Branch: "feat/a", IsDirty: true},
		{Branch: "feat/b", IsDirty: true, UnpushedCommits: 1},
	})

	keys := map[string]bool{}
	for _, blocker := range blockers {
		keys[blocker.Key] = true
	}
	for _, want := range []string{"feat/a:dirty", "feat/b:dirty", "feat/b:unpushed"} {
		if !keys[want] {
			t.Errorf("blockers %+v, want a key %q", blockers, want)
		}
	}
	if !strings.HasPrefix(blockers[0].Label, "feat/a") {
		t.Errorf("label %q should name its worktree", blockers[0].Label)
	}
}

func TestCleanBatchBlockersOfOneWorktreeReadAsBefore(t *testing.T) {
	check := domain.CleanCheckResult{Branch: "feat", IsDirty: true}
	if got, want := CleanBatchBlockers([]domain.CleanCheckResult{check}), CleanBlockers(check); !reflect.DeepEqual(got, want) {
		t.Errorf("blockers = %+v, want the single-worktree ones %+v", got, want)
	}
}

func TestCleanUnsafeRefusal(t *testing.T) {
	if _, unsafe := CleanUnsafeRefusal([]domain.CleanCheckResult{{Branch: "a"}, {Branch: "b"}}); unsafe {
		t.Error("a safe batch must not be refused")
	}

	single, _ := CleanUnsafeRefusal([]domain.CleanCheckResult{{Branch: "feat", IsDirty: true}})
	if single != fmt.Sprintf(domain.CleanForceHintFmt, "feat", domain.CleanUnsafeDirty) {
		t.Errorf("single refusal = %q, want the sentence it always had", single)
	}

	locked, _ := CleanUnsafeRefusal([]domain.CleanCheckResult{{Branch: "feat", IsLocked: true, IsDirty: true}})
	if locked != fmt.Sprintf(domain.CleanForceHintFmt, "feat", domain.CleanUnsafeLocked) {
		t.Errorf("locked refusal = %q, want the lock named first: it is an intent someone set", locked)
	}

	many, unsafe := CleanUnsafeRefusal([]domain.CleanCheckResult{{Branch: "a"}, {Branch: "b", IsDirty: true}, {Branch: "c", HasOpenPR: true}})
	if !unsafe {
		t.Fatal("one unsafe worktree refuses the batch")
	}
	for _, want := range []string{"2 worktree(s)", "nothing was removed", "b has uncommitted changes", "c has an open pull request", "--force"} {
		if !strings.Contains(many, want) {
			t.Errorf("refusal %q should contain %q", many, want)
		}
	}
}

func TestCleanSkipReasonSpeaksPrunesVocabulary(t *testing.T) {
	cases := map[domain.CleanCheckResult]string{
		{IsLocked: true, IsDirty: true}:     domain.PruneSkipLocked,
		{IsDirty: true, UnpushedCommits: 1}: domain.PruneSkipDirty,
		{UnpushedCommits: 1}:                domain.PruneSkipUnpushed,
		{HasOpenPR: true}:                   domain.PruneSkipOpenPR,
		{}:                                  "",
	}
	for check, want := range cases {
		if got := CleanSkipReason(check); got != want {
			t.Errorf("CleanSkipReason(%+v) = %q, want %q", check, got, want)
		}
	}
}

func TestCleanedBranchesSplitsRemovalsFromNoOps(t *testing.T) {
	removed, absent := CleanedBranches([]domain.CleanResult{{Branch: "a"}, {Branch: "b", AlreadyAbsent: true}, {Branch: "c"}})
	if !reflect.DeepEqual(removed, []string{"a", "c"}) || !reflect.DeepEqual(absent, []string{"b"}) {
		t.Errorf("removed = %v, absent = %v", removed, absent)
	}
}
