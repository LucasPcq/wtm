package repository_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReparentingRecordsTheNewParentAndReportsTheWorktreeUpdated(t *testing.T) {
	meta := storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{"feat/b": {SourceBranch: "feat/a", Ordinal: 3}}})
	var events []domain.Event

	err := repository.Reparent(t.Context(), repository.ReparentParams{Meta: meta, Branch: "feat/b", Path: "/repo/.trees/feat-b", To: "main", Report: func(e domain.Event) { events = append(events, e) }})

	require.NoError(t, err)
	record, err := meta.Load("feat/b")
	require.NoError(t, err)
	assert.Equal(t, "main", record.SourceBranch)
	ordinal := 3
	assert.Equal(t, []domain.Event{domain.WorktreeUpdated{
		Worktree: worktree.Identity{Branch: "feat/b", Path: "/repo/.trees/feat-b", Parent: "main", Ordinal: &ordinal, Isolation: "isolated"},
		Changed:  []worktree.IdentityField{worktree.IdentityParent},
	}}, events)
}

func TestAParentThatCannotBeRecordedIsReportedNowhere(t *testing.T) {
	refused := errors.New("read-only")
	meta := storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{"feat/b": {SourceBranch: "feat/a"}}, Fail: map[string]error{"Update": refused}})
	var events []domain.Event

	err := repository.Reparent(t.Context(), repository.ReparentParams{Meta: meta, Branch: "feat/b", To: "main", Report: func(e domain.Event) { events = append(events, e) }})

	require.ErrorIs(t, err, refused)
	assert.Empty(t, events)
}
