package runps_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runps"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheRepositoryIsAnAnswer(t *testing.T) {
	asked := runps.Request{Repo: "/acme"}

	folded, err := domain.Fold(runps.Request{}, domain.AnswersOf(runps.Questions{}.Form(), asked))

	require.NoError(t, err)
	assert.Equal(t, asked, folded)
}

func TestAPathRunPsDoesNotAskIsRefused(t *testing.T) {
	_, err := runps.Request{}.Answer("worktree", []string{"feat/a"})

	assert.Error(t, err)
}
