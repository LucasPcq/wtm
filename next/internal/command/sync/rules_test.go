package sync_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachWorktreeStandsByTheFirstThatApplies(t *testing.T) {
	cases := map[string]struct {
		seed  seeded
		facts func(*sync.Facts)
		want  sync.Standing
	}{
		"missing":     {seed: seeded{branch: "feat/x", parent: "main", behind: 1, missing: true}, want: sync.StandingMissing},
		"busy":        {seed: seeded{branch: "feat/x", parent: "main", behind: 1, state: worktree.Rebasing{Branch: "feat/x"}}, want: sync.StandingBusy},
		"unreadable":  {seed: seeded{branch: "feat/x", parent: "main", behind: 1}, facts: func(f *sync.Facts) { f.RecordErrs["feat/x"] = errors.New("newer schema") }, want: sync.StandingUnreadable},
		"no_parent":   {seed: seeded{branch: "feat/x", behind: 1}, want: sync.StandingNoParent},
		"parent_gone": {seed: seeded{branch: "feat/x", parent: "feat/deleted"}, want: sync.StandingParentGone},
		"diverged":    {seed: seeded{branch: "feat/x", parent: "main", behind: 1, origin: worktree.AheadBehind{Ahead: 1, Behind: 1}, never: 1}, want: sync.StandingDiverged},
		"rewritten":   {seed: seeded{branch: "feat/x", parent: "main", behind: 1, origin: worktree.AheadBehind{Ahead: 1, Behind: 1}}, want: sync.StandingRebases},
		"rebases":     {seed: seeded{branch: "feat/x", parent: "main", behind: 2}, want: sync.StandingRebases},
		"on_parent":   {seed: seeded{branch: "feat/x", parent: "main"}, want: sync.StandingOnParent},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			facts := factsOf(seeded{branch: "main"}, c.seed)
			if c.facts != nil {
				c.facts(&facts)
			}

			planned, _, err := sync.Decide(facts, naming("feat/x"))

			require.NoError(t, err)
			assert.Equal(t, c.want, standings(planned)["feat/x"])
		})
	}
}

func TestAWorktreeUnderOneThatRebasesRebasesToo(t *testing.T) {
	facts := factsOf(seeded{branch: "main"}, seeded{branch: "feat/a", parent: "main", behind: 1}, seeded{branch: "feat/b", parent: "feat/a"})

	planned, _, err := sync.Decide(facts, naming("feat/a"))

	require.NoError(t, err)
	assert.Equal(t, sync.StandingRebases, standings(planned)["feat/b"])
	assert.True(t, rebaseOf(planned, "feat/b").Descendant)
	assert.Equal(t, "feat/a-fork", rebaseOf(planned, "feat/b").Fork)
}

func TestTheBaseBranchNamedIsRefusedWithTheFastForwardThatRefreshesIt(t *testing.T) {
	facts := factsOf(seeded{branch: "main"}, seeded{branch: "feat/x", parent: "main"})

	_, err := sync.Resolve(facts, naming("main"))

	refused := failureOf(t, err)
	assert.Equal(t, domain.KindInvalid, refused.Kind)
	assert.Equal(t, sync.CodeBaseBranch, refused.Fields[0].Code)
	require.NotNil(t, refused.FollowUp)
	assert.Equal(t, domain.FollowUp{Command: domain.CommandFastForward, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"main"}}}, Code: sync.CodeFastForward, Message: "bring it up to origin instead"}, *refused.FollowUp)
}

func TestEachEntryNamedIsCheckedAndAllRefusesNames(t *testing.T) {
	facts := factsOf(seeded{branch: "main"}, seeded{branch: "feat/x", parent: "main"}, seeded{branch: "", state: worktree.Detached{}})
	facts.Located["/repo"] = "/repo"

	_, named := sync.Resolve(facts, naming("feat/x", "feat/x", "feat/nope", " "))
	_, all := sync.Resolve(facts, sync.Request{Worktrees: []string{"feat/x"}, All: true})

	fields := map[string]string{}
	for _, field := range failureOf(t, named).Fields {
		fields[field.Path] = field.Code
	}
	assert.Equal(t, map[string]string{"worktree[1]": domain.CodeDistinct, "worktree[2]": domain.CodeNotFound, "worktree[3]": domain.CodeRequired}, fields)
	assert.Equal(t, sync.CodeAllAndNames, failureOf(t, all).Fields[0].Code)
}

func TestNothingNamedSyncsTheWorktreesThatNeedIt(t *testing.T) {
	facts := factsOf(seeded{branch: "main"}, seeded{branch: "feat/a", parent: "main", behind: 1}, seeded{branch: "feat/b", parent: "main"}, seeded{branch: "feat/c", parent: "main", behind: 1, state: worktree.Bisecting{Branch: "feat/c"}}, seeded{branch: "feat/d", parent: "main", behind: 1, changes: 1})

	planned, _, err := sync.Decide(facts, sync.Request{})

	require.NoError(t, err)
	assert.Equal(t, map[string]sync.Standing{"feat/a": sync.StandingRebases}, standings(planned))
}

func TestAPlanThatMovesNothingIsEmpty(t *testing.T) {
	facts := factsOf(seeded{branch: "main"}, seeded{branch: "feat/a", parent: "main"})

	planned, _, err := sync.Decide(facts, naming("feat/a"))

	require.NoError(t, err)
	assert.True(t, sync.PlanEmpty(planned))
	assert.Equal(t, "main", planned.Worktrees[0].Parent, "the plan names the parent it already stands on")
}

func TestABaseBehindOriginIsFastForwardedFirstUnlessAskedNotTo(t *testing.T) {
	facts := factsOf(seeded{branch: "main", origin: worktree.AheadBehind{Behind: 3}}, seeded{branch: "feat/a", parent: "main"})

	planned, _, err := sync.Decide(facts, naming("feat/a"))
	require.NoError(t, err)
	asStands := naming("feat/a")
	asStands.FFParents = new(false)
	left, warnings, err := sync.Decide(facts, asStands)

	require.NoError(t, err)
	assert.Equal(t, []domain.Phase{domain.PhaseFastForward}, planned.Onto[0].Phases)
	assert.Equal(t, 3, planned.Onto[0].Origin.Behind)
	assert.Equal(t, sync.StandingRebases, standings(planned)["feat/a"], "its parent moves: it rebases")
	assert.Empty(t, left.Onto[0].Phases)
	assert.Equal(t, sync.StandingOnParent, standings(left)["feat/a"])
	require.Equal(t, []string{sync.CodeOntoBehind}, codes(warnings))
	assert.Equal(t, sync.CodeParents, warnings[0].FollowUp.Code)
}

func TestAParentDivergedOnlyByItsOwnRewriteIsNoWarning(t *testing.T) {
	rewritten := factsOf(seeded{branch: "main"}, seeded{branch: "feat/a", parent: "main", origin: worktree.AheadBehind{Ahead: 1, Behind: 1}}, seeded{branch: "feat/b", parent: "feat/a", behind: 1})
	pushedTo := factsOf(seeded{branch: "main"}, seeded{branch: "feat/a", parent: "main", origin: worktree.AheadBehind{Ahead: 1, Behind: 1}, never: 1}, seeded{branch: "feat/b", parent: "feat/a", behind: 1})
	alone := naming("feat/b")

	_, quiet, err := sync.Decide(rewritten, alone)
	require.NoError(t, err)
	_, warned, err := sync.Decide(pushedTo, alone)

	require.NoError(t, err)
	assert.Empty(t, quiet)
	assert.Equal(t, []string{sync.CodeOntoDiverged}, codes(warned))
}

func TestTheCommitsReplayedAreTheWorktreesOwnSinceItsForkPoint(t *testing.T) {
	facts := factsOf(seeded{branch: "main"}, seeded{branch: "feat/a", parent: "main", behind: 1})
	check := facts.Checks["feat/a"]
	check.Fork = worktree.Fork{Commit: "fork", Own: 3}
	facts.Checks["feat/a"] = check

	planned, _, err := sync.Decide(facts, naming("feat/a"))

	require.NoError(t, err)
	assert.Equal(t, "fork", rebaseOf(planned, "feat/a").Fork)
	assert.Equal(t, 3, rebaseOf(planned, "feat/a").Commits)
}

func TestTheForcedFollowUpOfAKeptWorktreeKeepsTheRestOfTheRequest(t *testing.T) {
	facts := factsOf(seeded{branch: "main"}, seeded{branch: "feat/a", parent: "main", behind: 1}, seeded{branch: "feat/b", parent: "feat/a", changes: 1})
	req := naming("feat/a")
	req.Push, req.KeepConflict, req.FFParents = true, true, new(false)

	_, warnings, err := sync.Decide(facts, req)

	require.NoError(t, err)
	require.Equal(t, []string{sync.CodeKept}, codes(warnings))
	forced := sync.Request{}
	for _, answer := range warnings[0].FollowUp.Answers {
		forced, err = forced.Answer(answer.Path, answer.Values)
		require.NoError(t, err)
	}
	assert.Equal(t, sync.Request{Worktrees: []string{"feat/b"}, Force: true, Push: true, KeepConflict: true, FFParents: new(false)}, forced)
}

func TestAWorktreeSharingNoCommitWithTheBaseItIsOnHasNoReparentToFollow(t *testing.T) {
	facts := factsOf(seeded{branch: "main"}, seeded{branch: "feat/x", parent: "main"})
	check := facts.Checks["feat/x"]
	check.Unrelated = true
	facts.Checks["feat/x"] = check

	_, warnings, err := sync.Decide(facts, naming("feat/x"))

	require.NoError(t, err)
	require.Equal(t, []string{sync.CodeUnrelated}, codes(warnings))
	assert.Nil(t, warnings[0].FollowUp, "it is on main already")
}
