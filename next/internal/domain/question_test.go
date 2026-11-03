package domain

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
)

func TestAValueOutsideItsChoicesIsAFieldError(t *testing.T) {
	accepted := []string{"a", "b"}

	assert.Nil(t, OneOf(OneOfParams{Path: "mode", Value: "a", Accepted: accepted}))
	assert.Nil(t, OneOf(OneOfParams{Path: "mode", Accepted: accepted}))
	assert.Equal(t, []FieldError{{Path: "mode", Code: CodeOneOf, Template: "{path} must be one of {accepted}, not {value}", Params: Params{"value": "c"}, Accepted: accepted}}, OneOf(OneOfParams{Path: "mode", Value: "c", Accepted: accepted}))
}

func TestNamesSpellsEachValueInOrder(t *testing.T) {
	assert.Equal(t, []string{"isolated", "verbatim"}, Names(project.Isolations()))
}

func TestEachWorktreeOptionSaysWhatItRunsByItsName(t *testing.T) {
	worktrees := []worktree.Worktree{
		{Path: "/repo/main", Main: true, State: worktree.OnBranch{Branch: "main"}},
		{Path: "/repo/feat-a", State: worktree.OnBranch{Branch: "feat/a"}},
		{Path: "/repo/feat-ab", State: worktree.OnBranch{Branch: "feat/ab"}},
	}
	jobs := []run.Job{
		{Name: "web", WorkDir: "/repo/feat-ab", Up: true, State: run.Running, Hold: run.HoldOwn, Profile: "dev"},
		{Name: "worker", WorkDir: "/repo/feat-ab", State: run.Crashed, Hold: run.HoldOwn},
		{Name: "db", WorkDir: "/repo/main", Up: true, State: run.Running, Hold: run.HoldOwn, SharedDir: "/repo/main"},
		{Name: "db", WorkDir: "/repo/feat-ab", Up: true, Hold: run.HoldClaim, SharedDir: "/repo/main"},
	}
	// feat is ambiguous: the options are those it matches, no longer one per worktree.
	options := RunningIn(WorktreeOptions(worktrees, WorktreeQuery{Name: "feat"}), worktrees, jobs)

	assert.Equal(t, []Option{{Value: "feat/a"}, {Value: "feat/ab", Up: []string{"web"}, Crashed: 1, Profile: "dev"}}, options)
	assert.Equal(t, []string{"db"}, RunningIn(WorktreeOptions(worktrees, WorktreeQuery{}), worktrees, jobs)[0].Holds)
}
