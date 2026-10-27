package teardown_test

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARemovalRunsItsHooksThenRemovesTheWorktreeItsBranchAndItsRecord(t *testing.T) {
	w := newWorld(seed{})
	facts := w.facts(t, onClean(), plain)

	removed, events := w.remove(t, facts, plain)

	assert.Equal(t, domain.StatusDone, removed.Status)
	assert.True(t, removed.Gone)
	require.Len(t, *w.hooks, 1)
	assert.Equal(t, at(plain), (*w.hooks)[0].WorkDir)
	assert.Contains(t, w.git.Calls(), "DeleteBranch "+plain)
	_, recorded := w.store.Meta(plain)
	assert.False(t, recorded)
	assert.IsType(t, domain.UnitStarted{}, events[0])
	assert.Contains(t, events, domain.Event(domain.UnitFinished{Subject: plain, Status: domain.StatusDone}))
}

func TestAFailingHookKeepsTheWorktreeWithTheCommandsCode(t *testing.T) {
	w := newWorld(seed{RunHooks: hookFails})
	facts := w.facts(t, onClean(), plain)

	removed, _ := w.remove(t, facts, plain)

	assert.Equal(t, domain.StatusFailed, removed.Status)
	assert.False(t, removed.Gone)
	assert.Equal(t, "sweep.hook_failed", removed.Err.Code)
	assert.NotContains(t, w.git.Calls(), "RemoveWorktree "+at(plain))
}

func TestAWorktreeWhoseDirectoryIsGoneRunsNoHookAndIsRemovedAllTheSame(t *testing.T) {
	w := newWorld(seed{Gone: []string{at(plain)}})
	facts := w.facts(t, onClean(), plain)

	removed, _ := w.remove(t, facts, plain)

	assert.True(t, w.removal(t, facts, plain).Missing)
	assert.Equal(t, domain.StatusDone, removed.Status)
	assert.Equal(t, []string{"sweep.hooks_skipped"}, codes(removed.Warnings))
	assert.Empty(t, *w.hooks)
}

func TestABranchThatCannotBeDeletedLeavesTheRemovalIncomplete(t *testing.T) {
	w := newWorld(seed{Fail: map[string]error{"DeleteBranch": errRefused}})
	facts := w.facts(t, base(), plain)

	removed, _ := w.remove(t, facts, plain)

	assert.Equal(t, domain.StatusFailed, removed.Status)
	assert.Equal(t, domain.ReasonIncomplete, removed.Reason)
	assert.Equal(t, "sweep.branch_kept", removed.Err.Code)
	assert.True(t, removed.Gone, "the worktree itself went")
}

func TestARemovalReachedAfterACancelIsSkipped(t *testing.T) {
	w := newWorld(seed{})
	facts := w.facts(t, base(), plain)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	removed, _ := w.removeIn(ctx, t, facts, plain)

	assert.Equal(t, domain.StatusSkipped, removed.Status)
	assert.Equal(t, domain.ReasonInterrupted, removed.Reason)
	assert.Empty(t, *w.hooks)
}

func TestFilesLeftOnDiskAreNamedAndTheWorktreeCountsAsRemoved(t *testing.T) {
	w := newWorld(seed{})
	w.service.Git = leaving{w.git}
	facts := w.facts(t, base(), plain)

	removed, _ := w.remove(t, facts, plain)

	assert.Equal(t, domain.StatusDone, removed.Status)
	assert.True(t, removed.Gone)
	assert.Equal(t, []string{"sweep.left_on_disk"}, codes(removed.Warnings))
	assert.Contains(t, removed.Warnings[0].Message, "sudo rm -rf "+at(plain))
}

func TestAWorktreeGitStillListsIsNotRemoved(t *testing.T) {
	w := newWorld(seed{Fail: map[string]error{"RemoveWorktree": errRefused}})
	facts := w.facts(t, base(), plain)

	removed, events := w.remove(t, facts, plain)

	assert.Equal(t, domain.StatusFailed, removed.Status)
	assert.False(t, removed.Gone)
	assert.Equal(t, "sweep.remove_failed", removed.Err.Code)
	for _, event := range events {
		_, isRemoved := event.(domain.WorktreeRemoved)
		assert.False(t, isRemoved, "no worktree went")
	}
}

func TestABranchNoWorktreeHoldsIsDeletedAloneWithItsRecord(t *testing.T) {
	w := newWorld(seed{})
	facts := w.facts(t, base())
	removal := facts.Removal(worktree.Worktree{State: worktree.OnBranch{Branch: alone}}, 0)
	var events []domain.Event

	removed := w.service.Remove(t.Context(), teardown.RemoveParams{Command: command, Facts: facts, Removal: removal, Report: func(e domain.Event) { events = append(events, e) }})

	assert.Equal(t, domain.StatusDone, removed.Status)
	assert.Contains(t, w.git.Calls(), "DeleteBranch "+alone)
	_, recorded := w.store.Meta(alone)
	assert.False(t, recorded)
	for _, event := range events {
		_, isRemoved := event.(domain.WorktreeRemoved)
		assert.False(t, isRemoved, "no worktree went")
	}
}

func TestHooksWhoseEnvironmentCannotBeReadRunWithoutItAndSaySo(t *testing.T) {
	w := newWorld(seed{StoreFail: map[string]error{"Read": errRefused}})
	config := onClean()
	facts := w.facts(t, config, plain)
	facts.Run = project.Run{Jobs: []project.Job{{Name: "db", Cmd: "docker compose up -d"}}}

	removed, _ := w.remove(t, facts, plain)

	assert.Equal(t, domain.StatusDone, removed.Status)
	assert.Equal(t, []string{"sweep.hook_env_unavailable"}, codes(removed.Warnings))
	require.Len(t, *w.hooks, 1)
	assert.Nil(t, (*w.hooks)[0].Env)
}
