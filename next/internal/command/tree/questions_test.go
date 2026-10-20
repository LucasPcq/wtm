package tree_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThePullRequestsAreAnAnswerOfTheRequest(t *testing.T) {
	withPRs := tree.Request{WithPRs: true}

	folded, err := domain.Fold(tree.Request{}, domain.AnswersOf(tree.Questions{}.Form(), withPRs))

	require.NoError(t, err)
	assert.Equal(t, withPRs, folded)
}

func TestAPathTreeDoesNotAskIsRefused(t *testing.T) {
	_, err := tree.Request{}.Answer("fetch", []string{"true"})

	assert.Error(t, err)
}
