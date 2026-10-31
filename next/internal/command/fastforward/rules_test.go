package fastforward_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWorktreeStandsWhereTheFirstFactThatAppliesPutsIt(t *testing.T) {
	behindOne := worktree.AheadBehind{Behind: 1}
	cases := map[string]struct {
		worktree seeded
		standing fastforward.Standing
	}{
		"missing_first":    {worktree: seeded{branch: "feat/x", offset: behindOne, missing: true, state: worktree.Rebasing{Branch: "feat/x"}}, standing: fastforward.StandingMissing},
		"rebasing":         {worktree: seeded{branch: "feat/x", offset: behindOne, state: worktree.Rebasing{Branch: "feat/x"}}, standing: fastforward.StandingBusy},
		"merging":          {worktree: seeded{branch: "feat/x", offset: behindOne, midway: worktree.MidwayMerge}, standing: fastforward.StandingBusy},
		"no_origin":        {worktree: seeded{branch: "feat/x", offOrigin: true}, standing: fastforward.StandingNoOrigin},
		"diverged":         {worktree: seeded{branch: "feat/x", offset: worktree.AheadBehind{Ahead: 1, Behind: 1}}, standing: fastforward.StandingDiverged},
		"ahead_not_latest": {worktree: seeded{branch: "feat/x", offset: worktree.AheadBehind{Ahead: 1}}, standing: fastforward.StandingAhead},
		"up_to_date":       {worktree: seeded{branch: "feat/x"}, standing: fastforward.StandingUpToDate},
		"behind":           {worktree: seeded{branch: "feat/x", offset: behindOne}, standing: fastforward.StandingBehind},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			planned, _, err := fastforward.Decide(factsOf(seeded{branch: "main"}, c.worktree), naming("feat/x"))

			require.NoError(t, err)
			assert.Equal(t, c.standing, planned.Worktrees[0].Standing)
			assert.Equal(t, c.standing == fastforward.StandingBehind, len(planned.Worktrees[0].Phases) > 0)
		})
	}
}

func TestDecideRefusesOnlyANamedWorktreeItCannotMoveSafely(t *testing.T) {
	behindDirty := func(branch string) seeded {
		return seeded{branch: branch, offset: worktree.AheadBehind{Behind: 1}, changes: 2}
	}
	facts := factsOf(seeded{branch: "main"}, behindDirty("feat/a"), behindDirty("feat/b"), seeded{branch: "feat/c", changes: 1}, seeded{branch: "feat/d", offset: worktree.AheadBehind{Behind: 1}})
	cases := map[string]struct {
		req    fastforward.Request
		fields map[string][]string
		moving []string
		kept   int
	}{
		"each_named_dirty_refused":   {req: naming("feat/d", "feat/a", "feat/b"), fields: map[string][]string{"worktree[1]": {fastforward.CodeDirty}, "worktree[2]": {fastforward.CodeDirty}}},
		"dirty_with_nothing_to_move": {req: naming("feat/c", "feat/d"), moving: []string{"feat/d"}},
		"forced":                     {req: fastforward.Request{Worktrees: []string{"feat/a"}, Force: true}, moving: []string{"feat/a"}},
		"chosen_by_all_kept":         {req: fastforward.Request{All: true}, moving: []string{"feat/d"}, kept: 2},
		"chosen_by_none_named_kept":  {req: fastforward.Request{}, moving: []string{"feat/d"}, kept: 2},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			planned, warnings, err := fastforward.Decide(facts, c.req)

			if c.fields != nil {
				refused := failureOf(t, err)
				assert.Equal(t, fastforward.CodeUnsafe, refused.Code)
				assert.Equal(t, domain.KindRefused, refused.Kind)
				assert.Equal(t, c.fields, fieldCodes(refused.Fields))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.moving, moving(planned))
			assert.Len(t, keptOf(warnings), c.kept)
		})
	}
}

func TestOnlyTheLastKeptWorktreeCarriesTheRequestForcingEveryOne(t *testing.T) {
	behindDirty := func(branch string) seeded {
		return seeded{branch: branch, offset: worktree.AheadBehind{Behind: 1}, changes: 1}
	}

	_, warnings, err := fastforward.Decide(factsOf(seeded{branch: "main"}, behindDirty("feat/a"), behindDirty("feat/b")), fastforward.Request{})

	require.NoError(t, err)
	kept := keptOf(warnings)
	require.Len(t, kept, 2)
	assert.Nil(t, kept[0].FollowUp)
	assert.Equal(t, domain.Params{"worktree": "feat/b", "blockers": "dirty"}, kept[1].Params)
	assert.Equal(t, &domain.FollowUp{Command: fastforward.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/a", "feat/b"}}, {Path: fastforward.PathForce, Values: []string{"true"}}}, Code: fastforward.CodeForce, Message: "fast-forward them anyway"}, kept[1].FollowUp)
}

func TestAWorktreeLeftAsItIsSaysWhatMovesItOn(t *testing.T) {
	cases := map[string]struct {
		worktree seeded
		code     string
		command  string
	}{
		"diverged": {worktree: seeded{branch: "feat/x", offset: worktree.AheadBehind{Ahead: 1, Behind: 2}}, code: fastforward.CodeDiverged, command: domain.CommandGo},
		"busy":     {worktree: seeded{branch: "feat/x", offset: worktree.AheadBehind{Behind: 1}, state: worktree.Bisecting{Branch: "feat/x"}}, code: fastforward.CodeBusy, command: domain.CommandGo},
		"missing":  {worktree: seeded{branch: "feat/x", missing: true, state: worktree.Rebasing{Branch: "feat/x"}}, code: fastforward.CodeMissing, command: domain.CommandClean},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, warnings, err := fastforward.Decide(factsOf(seeded{branch: "main"}, c.worktree), naming("feat/x"))

			require.NoError(t, err)
			require.Equal(t, []string{c.code}, codes(warnings))
			assert.Equal(t, c.command, warnings[0].FollowUp.Command)
		})
	}
}

func TestAnAheadWorktreeHasNothingToWarnOf(t *testing.T) {
	_, warnings, err := fastforward.Decide(factsOf(seeded{branch: "main"}, seeded{branch: "feat/x", offset: worktree.AheadBehind{Ahead: 3}}), naming("feat/x"))

	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestAPlanThatMovesNothingIsEmpty(t *testing.T) {
	moving := fastforward.Advance{Worktree: behind, Phases: []domain.Phase{domain.PhaseFastForward}}
	kept := fastforward.Advance{Worktree: dirty, Blockers: []fastforward.Blocker{fastforward.BlockerDirty}}
	cases := map[string]struct {
		plan  fastforward.Plan
		empty bool
	}{
		"none":      {plan: fastforward.Plan{}, empty: true},
		"kept_only": {plan: fastforward.Plan{Worktrees: []fastforward.Advance{kept}}, empty: true},
		"mixed":     {plan: fastforward.Plan{Worktrees: []fastforward.Advance{moving, kept}}},
		"forced":    {plan: fastforward.Plan{Force: true, Worktrees: []fastforward.Advance{{Worktree: dirty, Blockers: kept.Blockers, Phases: moving.Phases}}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.empty, fastforward.PlanEmpty(c.plan))
		})
	}
}
