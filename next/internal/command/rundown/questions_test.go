package rundown_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/rundown"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheWorktreesOfferedSayWhatEachRunsTheCallersPickedByDefault(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")

	questions, err := w.usecase.Questions(t.Context(), rundown.Request{}.From(domain.Caller{Cwd: w.at(featA)}))

	require.NoError(t, err)
	assert.Equal(t, []string{featA}, questions.Picked)
	assert.Equal(t, domain.OriginDefault, questions.Origin)
	require.Len(t, questions.Options, 3)
	assert.Equal(t, domain.Option{Value: featA, Tag: domain.TagCurrent, Up: []string{"api"}, Profile: "dev"}, questions.Options[1])
}

func TestAllSkipsTheWorktreesQuestion(t *testing.T) {
	questions := rundown.Questions{Request: rundown.Request{All: true}}

	assert.Equal(t, domain.SkipAll, questions.Form().Fields[0].Spec().Skip)
}

func TestARequestRoundTripsThroughItsAnswers(t *testing.T) {
	req := rundown.Request{Worktrees: []string{featA, featB}, Profile: "dev", All: true}

	answered := rundown.Request{}
	for _, field := range (rundown.Questions{}).Form().Fields {
		var err error
		answered, err = answered.Answer(field.Spec().Path, req.Values(field.Spec().Path))
		require.NoError(t, err)
	}

	assert.Equal(t, req, answered)
}
