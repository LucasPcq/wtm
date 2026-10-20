package events_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheRequestComesBackFromItsAnswers(t *testing.T) {
	for name, req := range map[string]events.Request{
		"repo": {Repo: "/repo"},
		"all":  {All: true},
	} {
		t.Run(name, func(t *testing.T) {
			folded, err := domain.Fold(events.Request{}, domain.AnswersOf(events.Questions{}.Form(), req))

			require.NoError(t, err)
			assert.Equal(t, req, folded)
		})
	}
}

func TestAFieldTheStreamDoesNotAskIsRefused(t *testing.T) {
	_, err := events.Request{}.Answer("branches", []string{"feat/a"})

	assert.Error(t, err)
}
