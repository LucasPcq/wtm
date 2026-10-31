package sync_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheDescendantsAreAskedOnlyWhenAWorktreeNamedHasSome(t *testing.T) {
	facts := factsOf(seeded{branch: "main"}, seeded{branch: "feat/a", parent: "main", behind: 1}, seeded{branch: "feat/b", parent: "feat/a"}, seeded{branch: "feat/c", parent: "feat/b"}, seeded{branch: "feat/x", parent: "main"})

	stack := sync.NewQuestions(facts, naming("feat/a"))
	alone := sync.NewQuestions(facts, naming("feat/x"))

	assert.Equal(t, map[string][]string{"feat/a": {"feat/b", "feat/c"}}, stack.Descendants.Under)
	assert.Empty(t, stack.Descendants.Skip)
	assert.True(t, stack.Descendants.Value)
	assert.Equal(t, domain.SkipNoDescendant, alone.Descendants.Skip)
}

func TestEachSwitchIsAskedOnlyWhenItChangesTheRun(t *testing.T) {
	facts := factsOf(seeded{branch: "main", origin: worktree.AheadBehind{Behind: 1}}, seeded{branch: "feat/a", parent: "main", behind: 1, changes: 2}, seeded{branch: "feat/x", parent: "main", offOrigin: true})

	dirty := sync.NewQuestions(facts, naming("feat/a"))
	clean := sync.NewQuestions(facts, naming("feat/x"))

	assert.Empty(t, dirty.Force.Skip)
	require.Len(t, dirty.Unsafe, 1)
	assert.Empty(t, dirty.KeepConflict.Skip)
	assert.Empty(t, dirty.FFParents.Skip)
	assert.Empty(t, dirty.Push.Skip)
	assert.Equal(t, domain.SkipSafe, clean.Force.Skip)
	assert.Equal(t, domain.SkipNothingToPush, clean.Push.Skip, "origin has no feat/x to push over")
}

func TestEachWorktreeOfThePickerSaysWhereItStandsOnItsParent(t *testing.T) {
	facts := factsOf(seeded{branch: "main"}, seeded{branch: "feat/a", parent: "main", behind: 2}, seeded{branch: "feat/r", parent: "main", state: worktree.Rebasing{Branch: "feat/r"}})

	q := sync.NewQuestions(facts, sync.Request{})

	require.Len(t, q.Worktrees.Options, 2)
	assert.Equal(t, domain.Option{Value: "feat/a", Parent: "main", Behind: 2, Origin: &worktree.Origin{State: worktree.UpToDate}}, q.Worktrees.Options[0])
	assert.Equal(t, worktree.OperationRebase, q.Worktrees.Options[1].Operation)
	assert.Equal(t, []string{"feat/a"}, q.Worktrees.Picked.Value)
	assert.Equal(t, domain.OriginDefault, q.Worktrees.Picked.Origin)
}

func TestTheAnswersOfTheFormRoundTripThroughTheRequest(t *testing.T) {
	req := sync.Request{Worktrees: []string{"feat/a"}, Descendants: new(false), Force: true, KeepConflict: true, FFParents: new(false), Push: true, NoFetch: true}

	answered := sync.Request{}
	for _, answer := range domain.AnswersOf(sync.Questions{}.Form(), req) {
		var err error
		answered, err = answered.Answer(answer.Path, answer.Values)
		require.NoError(t, err)
	}

	assert.Equal(t, req, answered)
}

// The wizard asks on origin as last fetched and the plan fetches first: a question skipped on stale refs left a named dirty worktree refused with no Force to answer.
func TestEachQuestionTheFetchCouldMakeMatterIsAsked(t *testing.T) {
	facts := factsOf(seeded{branch: "main"}, seeded{branch: "feat/a", parent: "main", changes: 1}, seeded{branch: "feat/x", parent: "main", offOrigin: true})

	q := sync.NewQuestions(facts, naming("feat/a"))

	require.Len(t, q.Unsafe, 1, "feat/a stands on main as last fetched, and may move once fetched")
	assert.Empty(t, q.Force.Skip)
	assert.Empty(t, q.KeepConflict.Skip)
	assert.Empty(t, q.FFParents.Skip, "main is on origin: the fetch may bring it commits")
	assert.Empty(t, q.Push.Skip)
}

func TestAWorktreeWhoseParentIsBehindOriginIsPickedByDefault(t *testing.T) {
	facts := factsOf(seeded{branch: "main", origin: worktree.AheadBehind{Behind: 1}}, seeded{branch: "feat/a", parent: "main"})

	q := sync.NewQuestions(facts, sync.Request{})

	assert.Equal(t, []string{"feat/a"}, q.Worktrees.Picked.Value)
}
