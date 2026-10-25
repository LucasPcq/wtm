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
		"no_worktree": {ConditionParams{NoWorktree: true, Dirty: true}, ConditionNoWorktree},
		"unreadable":  {ConditionParams{Unreadable: true, Missing: true}, ConditionUnreadable},
		"missing":     {ConditionParams{Missing: true, Rebasing: true}, ConditionMissing},
		"rebasing":    {ConditionParams{Rebasing: true, Merged: true}, ConditionRebasing},
		"merged":      {ConditionParams{Merged: true, Dirty: true}, ConditionMerged},
		"changed":     {ConditionParams{Dirty: true}, ConditionChanged},
		"clean":       {ConditionParams{}, ConditionClean},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.Want, ConditionOf(c.Params))
		})
	}
	assert.Len(t, Conditions(), len(cases), "each condition has its case")
}
