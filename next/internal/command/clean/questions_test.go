package clean_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheWorktreesOfferedAreEveryOneButTheMainWorktree(t *testing.T) {
	w := newWorld(t, seed{})

	form := w.questions(t, clean.Request{}).Form()

	worktrees, ok := form.Fields[0].(domain.MultiSelectField)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{plain, parent, child, grandchild, dirty, locked, pushed, local, w.at("detached")}, valuesOf(worktrees.Options))
	assert.Equal(t, map[string][]string{domain.PathWorktree: {domain.CodeRequired}}, fieldCodes(form.Problems))
}

func TestReparentIsAskedOnlyWhenAChildIsLeftBehind(t *testing.T) {
	w := newWorld(t, seed{})

	alone := w.questions(t, removing(plain))
	parenting := w.questions(t, removing(parent))
	given := w.questions(t, clean.Request{Worktrees: []string{plain}, ReparentChildren: true})

	assert.Equal(t, domain.SkipNoOrphan, skipOf(alone.Form(), clean.PathReparentChildren))
	assert.Empty(t, skipOf(parenting.Form(), clean.PathReparentChildren))
	assert.Equal(t, []clean.Move{{Branch: child, From: parent, To: "main"}}, parenting.Orphans)
	assert.Empty(t, skipOf(given.Form(), clean.PathReparentChildren), "a flag given never makes its line disappear")
	assert.Equal(t, domain.OriginRequest, given.Reparent.Origin)
}

func TestForceIsAskedOnlyWhenAWorktreeIsUnsafe(t *testing.T) {
	w := newWorld(t, seed{})

	safe := w.questions(t, removing(plain))
	unsafe := w.questions(t, removing(plain, dirty))

	assert.Equal(t, domain.SkipSafe, skipOf(safe.Form(), clean.PathForce))
	assert.Empty(t, skipOf(unsafe.Form(), clean.PathForce))
	require.Len(t, unsafe.Unsafe, 1)
	assert.Equal(t, []clean.Blocker{clean.BlockerDirty}, unsafe.Unsafe[0].Blockers)
	assert.False(t, unsafe.Force.Value)
}

func TestAnEntryBlankRepeatedOrTheMainWorktreeIsAProblemOfItsOwn(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := clean.Resolve(w.facts(t, removing(plain, " ", plain, "main")), removing(plain, " ", plain, "main"))

	assert.Equal(t, map[string][]string{
		"worktree[1]": {domain.CodeRequired},
		"worktree[2]": {domain.CodeDistinct},
		"worktree[3]": {clean.CodeMainWorktree},
	}, fieldCodes(failureOf(t, err).Fields))
}

func TestTheAnswersOfTheFormGiveBackTheRequest(t *testing.T) {
	w := newWorld(t, seed{})
	req := clean.Request{Worktrees: []string{parent, dirty}, ReparentChildren: true, Force: true}

	folded, err := domain.Fold(clean.Request{}, w.questions(t, req).Form().Answers())

	require.NoError(t, err)
	assert.Equal(t, req, folded)
	for _, path := range []string{domain.PathWorktree, clean.PathReparentChildren, clean.PathForce} {
		again, err := clean.Request{}.Answer(path, req.Values(path))
		require.NoError(t, err)
		assert.Equal(t, req.Values(path), again.Values(path), path)
	}
}

func TestAPathNotOfTheRequestIsRefused(t *testing.T) {
	_, err := clean.Request{}.Answer("keep_data", []string{"true"})

	assert.Equal(t, map[string][]string{"keep_data": {domain.CodeUnknown}}, fieldCodes(failureOf(t, err).Fields))
}
