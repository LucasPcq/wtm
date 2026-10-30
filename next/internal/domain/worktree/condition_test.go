package worktree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAWorktreeIsInTheFirstConditionThatApplies(t *testing.T) {
	cases := map[string]struct {
		Params ConditionParams
		Want   Condition
	}{
		"missing":        {ConditionParams{Missing: true, Operation: OperationRebase}, ConditionMissing},
		"rebasing":       {ConditionParams{Operation: OperationRebase, Merged: true}, ConditionRebasing},
		"bisecting":      {ConditionParams{Operation: OperationBisect, Merged: true}, ConditionBisecting},
		"merging":        {ConditionParams{Operation: OperationMerge, Dirty: true}, ConditionMerging},
		"cherry_picking": {ConditionParams{Operation: OperationCherryPick}, ConditionCherryPicking},
		"reverting":      {ConditionParams{Operation: OperationRevert}, ConditionReverting},
		"applying":       {ConditionParams{Operation: OperationApply}, ConditionApplying},
		"merged":         {ConditionParams{Merged: true, Dirty: true}, ConditionMerged},
		"changed":        {ConditionParams{Dirty: true}, ConditionChanged},
		"clean":          {ConditionParams{}, ConditionClean},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.Want, ConditionOf(c.Params))
		})
	}
	assert.Len(t, Conditions(), len(cases)+3, "each condition a read can rank has its case; no_worktree, deleted and unreadable are set by the read")
}

func TestAConditionNamesTheOperationGitStoppedInTheMiddleOf(t *testing.T) {
	for _, operation := range Operations() {
		assert.Contains(t, Conditions(), Condition(operation), operation)
		assert.Equal(t, operation, Condition(operation).Operation())
	}
	assert.Empty(t, ConditionChanged.Operation())
	assert.Empty(t, ConditionMissing.Operation())
}
