package list_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARefreshIsAnAnswerAsThePullRequestsAre(t *testing.T) {
	refresh := list.Request{WithPRs: true, Fetch: true}

	folded, err := domain.Fold(list.Request{}, domain.AnswersOf(list.Questions{}.Form(), refresh))

	require.NoError(t, err)
	assert.Equal(t, refresh, folded)
}

func TestAPathListDoesNotAskIsRefused(t *testing.T) {
	_, err := list.Request{}.Answer("branches", []string{"feat/a"})

	assert.Error(t, err)
}
