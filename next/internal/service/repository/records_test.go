package repository_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"

	"github.com/stretchr/testify/assert"
)

func TestEachRecordIsReadOneMissingHasNoEntryOneThatDoesNotReadItsError(t *testing.T) {
	meta := storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{"feat/a": {SourceBranch: "main"}}, Unreadable: []string{"feat/b"}})

	records, errs := repository.Records(meta, []string{"main", "feat/a", "feat/b", ""})

	assert.Equal(t, map[string]worktree.Meta{"feat/a": {SourceBranch: "main"}}, records)
	assert.Len(t, errs, 1)
	assert.ErrorContains(t, errs["feat/b"], "does not read")
}
