package prune_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// B7: legacy's forced dry run listed a dirty worktree as merged, nothing said of what forcing it loses.
func TestTheRecapSaysWhyEachWorktreeIsDoneWithWhatForcingLosesAndWhatIsKept(t *testing.T) {
	w := newWorld(t, seed{})

	forced, _ := w.plan(t, prune.Request{Worktrees: []string{dirty, closed}, Force: true})
	unforced, _ := w.plan(t, prune.Request{Merged: true})

	assert.Equal(t, []domain.RecapRow{{Key: "prune", Label: "Prune", Path: domain.PathWorktree, Values: []domain.RecapValue{
		{Value: dirty, Note: teardown.NoteForced, Params: domain.Params{"blockers": "dirty", "reason": "merged"}, Text: dirty + " (merged #4; dirty, forced)"},
		{Value: closed, Params: domain.Params{"reason": "closed"}, Text: closed + " (closed #2)"},
	}}}, forced.Recap)
	assert.Equal(t, domain.RecapRow{Key: "kept", Label: "Kept", Path: prune.PathForce, Values: []domain.RecapValue{
		{Value: dirty, Note: prune.NoteKept, Params: domain.Params{"blockers": "dirty", "reason": "merged"}, Text: dirty + " (merged #4; dirty)"},
	}}, unforced.Recap[1])
}

func TestEachCandidateCarriesThePullRequestThatFinishedIt(t *testing.T) {
	w := newWorld(t, seed{})

	planned, _ := w.plan(t, prune.Request{})

	for _, c := range planned.Worktrees {
		switch c.Reason {
		case prune.ReasonGone:
			assert.Nil(t, c.PR, c.Worktree)
		case prune.ReasonMerged, prune.ReasonClosed:
			require.NotNil(t, c.PR, c.Worktree)
			assert.NotZero(t, c.PR.Number)
		}
	}
}

func TestThePlanLocksEachWorktreeRemovedAndEachChildReparented(t *testing.T) {
	w := newWorld(t, seed{})

	orphaning, _ := w.plan(t, pruning(merged))
	reparenting, _ := w.plan(t, prune.Request{Worktrees: []string{merged}, ReparentChildren: true})

	assert.Equal(t, domain.WorktreeLocks([]string{merged}), orphaning.Locks())
	assert.Equal(t, domain.WorktreeLocks([]string{merged, child}), reparenting.Locks())
	assert.Equal(t, domain.WorktreeLocks(nil), prune.Request{}.Locks(), "the plan names the worktrees a request naming none locks")
}

func TestTheFingerprintCoversTheHooksAndWhatTheirEnvironmentReads(t *testing.T) {
	w := newWorld(t, seed{})
	before, _ := w.plan(t, pruning(merged))
	writeFile(t, w.stateDir+"/config.toml", withHook("true"))
	hooked, _ := w.plan(t, pruning(merged))
	w.record(t, merged, worktree.Meta{SourceBranch: "main", Ordinal: 7})
	renumbered, _ := w.plan(t, pruning(merged))

	fingerprints := map[string]bool{}
	for _, planned := range []prune.Plan{before, hooked, renumbered} {
		fingerprint, err := operation.Fingerprint(planned.Fingerprinted())
		require.NoError(t, err)
		fingerprints[fingerprint] = true
	}
	assert.Len(t, fingerprints, 3)
}

func TestAPlanThatRemovesNothingIsEmpty(t *testing.T) {
	removing := prune.Candidate{Removal: teardown.Removal{Worktree: merged, Phases: []domain.Phase{domain.PhaseRemove}}}
	kept := prune.Candidate{Removal: teardown.Removal{Worktree: dirty, Blockers: []worktree.Blocker{worktree.BlockerDirty}}}
	cases := map[string]struct {
		plan  prune.Plan
		empty bool
	}{
		"none":      {plan: prune.Plan{}, empty: true},
		"kept_only": {plan: prune.Plan{Worktrees: []prune.Candidate{kept}}, empty: true},
		"mixed":     {plan: prune.Plan{Worktrees: []prune.Candidate{removing, kept}}},
		"forced":    {plan: prune.Plan{Force: true, Worktrees: []prune.Candidate{{Removal: teardown.Removal{Worktree: dirty, Blockers: kept.Blockers, Phases: removing.Phases}}}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.empty, prune.PlanEmpty(c.plan))
		})
	}
}
