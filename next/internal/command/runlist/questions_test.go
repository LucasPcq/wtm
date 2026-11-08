package runlist_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runlist"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheFormOffersEachWorktreeTheCallersAnswered(t *testing.T) {
	q, err := usecase(seed{Run: declared}).Questions(t.Context(), runlist.Request{}.From(domain.Caller{Cwd: featA}))

	require.NoError(t, err)
	field, ok := q.Form().Fields[0].(domain.SelectField)
	require.True(t, ok)
	assert.Equal(t, "feat/a", field.Value)
	assert.Len(t, field.Options, 2)
}

func TestTheWorktreeIsAnAnswer(t *testing.T) {
	asked := runlist.Request{Worktree: "feat/a"}

	folded, err := domain.Fold(runlist.Request{}, domain.AnswersOf(runlist.Questions{}.Form(), asked))

	require.NoError(t, err)
	assert.Equal(t, asked, folded)
}
