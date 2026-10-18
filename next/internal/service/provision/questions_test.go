package provision_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheEnvStrategyIsTheOneGivenElseRememberedElseConfigured(t *testing.T) {
	remembered := map[string]string{project.RememberEnvStrategy: string(project.EnvParent)}
	cases := map[string]struct {
		params provision.QuestionsParams
		want   domain.Answer[project.EnvStrategy]
	}{
		"given":                            {provision.QuestionsParams{Given: provision.Given{EnvFrom: project.EnvMain}, Remembered: remembered, EnvStrategy: project.EnvExample}, domain.Answer[project.EnvStrategy]{Value: project.EnvMain, Origin: domain.OriginRequest}},
		"remembered":                       {provision.QuestionsParams{Remembered: remembered, EnvStrategy: project.EnvExample}, domain.Answer[project.EnvStrategy]{Value: project.EnvParent, Origin: domain.OriginRemembered}},
		"asked again":                      {provision.QuestionsParams{Remembered: remembered, Ask: true, EnvStrategy: project.EnvExample}, domain.Answer[project.EnvStrategy]{Value: project.EnvExample, Origin: domain.OriginConfig}},
		"remembered as the config default": {provision.QuestionsParams{Remembered: map[string]string{project.RememberEnvStrategy: project.RememberEnvConfig}, EnvStrategy: project.EnvMain}, domain.Answer[project.EnvStrategy]{Value: project.EnvMain, Origin: domain.OriginRemembered}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			questions := provision.NewQuestions(c.params)

			assert.Equal(t, c.want, questions.EnvFrom.Answer)
			assert.Equal(t, c.want.Value, questions.Answers().EnvFrom())
		})
	}
}

func TestAskingAgainStillShowsWhatIsRemembered(t *testing.T) {
	questions := provision.NewQuestions(provision.QuestionsParams{Remembered: map[string]string{project.RememberEnvStrategy: string(project.EnvParent), project.RememberSourceUpdate: project.SourceUpdateFastForward}, Ask: true})

	assert.Equal(t, project.EnvParent, questions.EnvFrom.Remembered)
	assert.Equal(t, new(true), questions.FastForward.Remembered)
	assert.Equal(t, domain.OriginDefault, questions.FastForward.Origin)
}

func TestAWorktreeThatIsNotCreatedIsAskedNothingAndRemembersNothing(t *testing.T) {
	remembered := map[string]string{project.RememberEnvStrategy: string(project.EnvParent), project.RememberIsolation: string(project.Verbatim)}
	questions := provision.NewQuestions(provision.QuestionsParams{Remembered: remembered, EnvStrategy: project.EnvMain, Exists: "exists", Remember: []string{"env_from", "isolation", "fast_forward"}})

	for _, field := range questions.Fields(provision.Wording{}) {
		assert.Equal(t, "exists", field.Spec().Skip, field.Spec().Path)
		assert.False(t, questions.Remembers(field.Spec().Path), field.Spec().Path)
	}
	assert.Empty(t, questions.EnvFrom.Value)
	assert.Empty(t, questions.Isolation.Value)
}

func TestTheFastForwardIsSkippedWhenNothingIsBehind(t *testing.T) {
	questions := provision.NewQuestions(provision.QuestionsParams{NotBehind: "up to date", Remember: []string{"fast_forward", "env_from"}})

	assert.Equal(t, "up to date", questions.FastForward.Skip)
	assert.False(t, questions.Remembers("fast_forward"))
	assert.True(t, questions.Remembers("env_from"))
}

func TestAnUnknownStrategyOrAGivenIsolationOutsideTheListIsAProblem(t *testing.T) {
	questions := provision.NewQuestions(provision.QuestionsParams{Given: provision.Given{EnvFrom: "copy", Isolation: "shared"}})

	problems := questions.Problems()

	require.Len(t, problems, 2)
	assert.Equal(t, "env_from", problems[0].Path)
	assert.Equal(t, "isolation", problems[1].Path)
}

func TestEachAnswerOfANewWorktreeReadsBackAsAnswerTakesIt(t *testing.T) {
	given := provision.Given{EnvFrom: project.EnvParent, Isolation: project.Verbatim, FastForward: new(false)}

	got := provision.Given{}
	for _, field := range (provision.Questions{}).Fields(provision.Wording{}) {
		var err error
		got, err = got.Answer(field.Spec().Path, given.Values(field.Spec().Path))
		require.NoError(t, err)
	}

	assert.Equal(t, given, got)
	_, err := got.Answer("branches", []string{"a"})
	assert.Error(t, err)
}

func TestTheAnswersToRememberAreKeyedAsConfigTomlKeepsThem(t *testing.T) {
	answers := provision.NewQuestions(provision.QuestionsParams{Given: provision.Given{EnvFrom: project.EnvMain, Isolation: project.Verbatim, FastForward: new(true)}}).Answers()

	assert.Equal(t, map[string]string{"env_strategy": "main", "isolation": "verbatim", "source_update": "ff"}, answers.Remembered([]string{"env_from", "isolation", "fast_forward"}))
	assert.Equal(t, map[string]string{"isolation": "verbatim"}, answers.Remembered([]string{"isolation"}))
}
