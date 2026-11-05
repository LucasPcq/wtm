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

	assert.Equal(t, []Option{{Value: "feat/a"}, {Value: "feat/ab", Up: []string{"web"}, Crashed: 1, Holds: []string{"db"}, Profile: "dev"}}, options)
}

func TestAWorktreeHoldsItsClaimsAndMainOnlyTheSharedServicesItHoldsItself(t *testing.T) {
	worktrees := []worktree.Worktree{
		{Path: "/repo/main", Main: true, State: worktree.OnBranch{Branch: "main"}},
		{Path: "/repo/feat-a", State: worktree.OnBranch{Branch: "feat/a"}},
	}
	served := func(mainHolds, unknown bool) []run.Job {
		return []run.Job{
			{Name: "db", WorkDir: "/repo/main", Up: true, State: run.Running, Hold: run.HoldOwn, SharedDir: "/repo/main", MainHolds: mainHolds, MainHoldsUnknown: unknown},
			{Name: "db", WorkDir: "/repo/feat-a", Up: true, Hold: run.HoldClaim, SharedDir: "/repo/main"},
		}
	}
	for _, tc := range []struct {
		name      string
		mainHolds bool
		unknown   bool
		main      []string
	}{
		{name: "for_the_others", main: nil},
		{name: "held", mainHolds: true, main: []string{"db"}},
		{name: "unknown", unknown: true, main: []string{"db"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := RunningIn(WorktreeOptions(worktrees, WorktreeQuery{}), worktrees, served(tc.mainHolds, tc.unknown))

			assert.Equal(t, tc.main, options[0].Holds, "main holds a shared service it runs only when it holds it itself")
			assert.Equal(t, []string{"db"}, options[1].Holds, "a claim-only worktree holds its claim")
		})
	}
}
