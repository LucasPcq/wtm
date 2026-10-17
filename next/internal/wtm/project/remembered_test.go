package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEachRememberedQuestionOffersItsAnswers(t *testing.T) {
	cases := map[string][]string{
		RememberEnvStrategy:  {"example", "main", "parent"},
		RememberIsolation:    {"isolated", "verbatim"},
		RememberSourceUpdate: {SourceUpdateFastForward, SourceUpdateKeep},
	}
	for id, want := range cases {
		t.Run(id, func(t *testing.T) {
			values, known := RememberableValues(id)

			assert.True(t, known)
			assert.Equal(t, want, values)
		})
	}
}

func TestAQuestionNothingRemembersHasNoAnswers(t *testing.T) {
	values, known := RememberableValues("delete")

	assert.False(t, known)
	assert.Empty(t, values)
}

func TestTheRememberedIdsAndValuesAreTheOnesWtmWrites(t *testing.T) {
	assert.Equal(t, "env_strategy", RememberEnvStrategy)
	assert.Equal(t, "isolation", RememberIsolation)
	assert.Equal(t, "source_update", RememberSourceUpdate)
	assert.Equal(t, "ff", SourceUpdateFastForward)
	assert.Equal(t, "keep", SourceUpdateKeep)
}
