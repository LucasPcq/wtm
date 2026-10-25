package provision_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheEnvStrategyIsTheOneGivenElseRememberedElseConfigDefault(t *testing.T) {
	remembered := map[string]string{project.RememberEnvStrategy: string(project.EnvParent)}
	cases := map[string]struct {
		given      provision.Given
		remembered map[string]string
		want       domain.Answer[project.EnvStrategy]
		strategy   project.EnvStrategy
	}{
		"given":                     {provision.Given{EnvFrom: project.EnvMain}, remembered, domain.Answer[project.EnvStrategy]{Value: project.EnvMain, Origin: domain.OriginRequest}, project.EnvMain},
		"config_default_given":      {provision.Given{EnvFrom: provision.EnvFromConfig}, remembered, domain.Answer[project.EnvStrategy]{Value: provision.EnvFromConfig, Origin: domain.OriginRequest}, project.EnvExample},
		"remembered":                {provision.Given{}, remembered, domain.Answer[project.EnvStrategy]{Value: project.EnvParent, Origin: domain.OriginRemembered}, project.EnvParent},
		"asked_again":               {provision.Given{Ask: true}, remembered, domain.Answer[project.EnvStrategy]{Value: provision.EnvFromConfig, Origin: domain.OriginDefault}, project.EnvExample},
		"config_default_remembered": {provision.Given{}, map[string]string{project.RememberEnvStrategy: project.RememberEnvConfig}, domain.Answer[project.EnvStrategy]{Value: provision.EnvFromConfig, Origin: domain.OriginRemembered}, project.EnvExample},
		"nothing_given":             {provision.Given{}, nil, domain.Answer[project.EnvStrategy]{Value: provision.EnvFromConfig, Origin: domain.OriginDefault}, project.EnvExample},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			questions := provision.NewQuestions(provision.QuestionsParams{Given: c.given, Repository: repository(c.remembered)})

			assert.Equal(t, c.want, questions.EnvFrom.Answer)
			assert.Equal(t, c.strategy, questions.Answers().EnvFrom())
		})
	}
}

func TestTheConfigDefaultIsTheFirstEnvOptionNamingTheConfiguredStrategy(t *testing.T) {
	fields := provision.NewQuestions(provision.QuestionsParams{Repository: repository(nil)}).Fields("")

	field, isSelect := fields[0].(domain.SelectField)
	require.True(t, isSelect)
	assert.Equal(t, []domain.Option{
		{Value: "config", Label: "config default (example)", Short: "config default"},
		{Value: "example", Label: "example — from .env.example"},
		{Value: "main", Label: "main — the main checkout's .env"},
		{Value: "parent", Label: "parent — the source worktree's .env"},
	}, field.Options)
}

func TestAskingAgainStillShowsWhatIsRemembered(t *testing.T) {
	remembered := map[string]string{project.RememberEnvStrategy: project.RememberEnvConfig, project.RememberSourceUpdate: project.SourceUpdateFastForward}
	questions := provision.NewQuestions(provision.QuestionsParams{Given: provision.Given{Ask: true}, Repository: repository(remembered), Behind: []string{"main"}})

	assert.Equal(t, provision.EnvFromConfig, questions.EnvFrom.Remembered)
	assert.Equal(t, new(true), questions.FastForward.Remembered)
	assert.Equal(t, domain.OriginDefault, questions.FastForward.Origin)
	assert.True(t, questions.Given().Ask, "a request settled again sets the remembered answers aside as this one did")
}

func TestAWorktreeThatIsNotCreatedIsAskedNothingAndRemembersNothing(t *testing.T) {
	remembered := map[string]string{project.RememberEnvStrategy: string(project.EnvParent), project.RememberIsolation: string(project.Verbatim)}
	questions := provision.NewQuestions(provision.QuestionsParams{Given: provision.Given{Remember: []string{"env_from", "isolation", "fast_forward"}}, Repository: repository(remembered), Exists: "exists"})

	for _, field := range questions.Fields("") {
		assert.Equal(t, "exists", field.Spec().Skip, field.Spec().Path)
		assert.False(t, questions.Remembers(field.Spec().Path), field.Spec().Path)
	}
	assert.Empty(t, questions.EnvFrom.Value)
	assert.Empty(t, questions.Isolation.Value)
	assert.Empty(t, questions.Given().Remember)
}

func TestAnIsolationLeftUnansweredIsTheRememberedOneElseTheRunsElseIsolated(t *testing.T) {
	isolating := project.Run{EnvPorts: []project.EnvPortLink{{}}}
	verbatim := map[string]string{project.RememberIsolation: string(project.Verbatim)}
	cases := map[string]struct {
		remembered map[string]string
		run        project.Run
		want       domain.Answer[project.Isolation]
	}{
		"remembered":       {verbatim, isolating, domain.Answer[project.Isolation]{Value: project.Verbatim, Origin: domain.OriginRemembered}},
		"nothingToIsolate": {verbatim, project.Run{Isolation: project.Isolated}, domain.Answer[project.Isolation]{Value: project.Isolated, Origin: domain.OriginConfig}},
		"run":              {nil, project.Run{Isolation: project.Verbatim}, domain.Answer[project.Isolation]{Value: project.Verbatim, Origin: domain.OriginConfig}},
		"default":          {nil, isolating, domain.Answer[project.Isolation]{Value: project.Isolated, Origin: domain.OriginDefault}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			repo := repository(c.remembered)
			repo.Run = c.run

			assert.Equal(t, c.want, provision.NewQuestions(provision.QuestionsParams{Repository: repo}).Isolation.Answer)
		})
	}
}

func TestTheFastForwardIsTheRequestsElseNoneWhenSkippedElseTheRememberedElseNo(t *testing.T) {
	ff := map[string]string{project.RememberSourceUpdate: project.SourceUpdateFastForward}
	keep := map[string]string{project.RememberSourceUpdate: project.SourceUpdateKeep}
	main := []string{"main"}
	notBehind := "nothing to fast-forward: the source is not behind origin"
	cases := map[string]struct {
		given      *bool
		behind     []string
		remembered map[string]string
		want       domain.Confirm
	}{
		"given":          {new(true), nil, keep, domain.Confirm{Answer: domain.Answer[bool]{Value: true, Origin: domain.OriginRequest}, Skip: notBehind, Remembered: new(false)}},
		"skipped":        {nil, nil, ff, domain.Confirm{Skip: notBehind, Remembered: new(true)}},
		"remembered":     {nil, main, ff, domain.Confirm{Answer: domain.Answer[bool]{Value: true, Origin: domain.OriginRemembered}, Remembered: new(true)}},
		"rememberedKeep": {nil, main, keep, domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginRemembered}, Remembered: new(false)}},
		"default":        {nil, main, nil, domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			params := provision.QuestionsParams{Given: provision.Given{FastForward: c.given}, Repository: repository(c.remembered), Behind: c.behind}

			assert.Equal(t, c.want, provision.NewQuestions(params).FastForward)
		})
	}
}

func TestTheIsolationIsSkippedWithWhyRunTomlIsNotRead(t *testing.T) {
	repo := repository(nil)
	repo.RunErr = errors.New("run.toml: line 3")

	questions := provision.NewQuestions(provision.QuestionsParams{Repository: repo})

	assert.Equal(t, "run.toml could not be read", questions.Isolation.Skip)
	assert.Equal(t, domain.Answer[project.Isolation]{Value: project.Isolated, Origin: domain.OriginDefault}, questions.Isolation.Answer)
}

func TestAnIsolationGivenOverAnUnreadableRunTomlIsRememberedWhenAsked(t *testing.T) {
	repo := repository(nil)
	repo.RunErr = errors.New("run.toml: line 3")

	questions := provision.NewQuestions(provision.QuestionsParams{Given: provision.Given{Isolation: project.Verbatim, Remember: []string{"isolation"}}, Repository: repo})

	assert.True(t, questions.Remembers("isolation"))
	assert.Equal(t, []string{"isolation"}, questions.Given().Remember)
	existing := provision.NewQuestions(provision.QuestionsParams{Given: provision.Given{Isolation: project.Verbatim, Remember: []string{"isolation"}}, Repository: repo, Exists: "exists"})
	assert.False(t, existing.Remembers("isolation"), "a worktree not created remembers nothing")
}

func TestARememberedIsolationHoldsWhenRunTomlIsNotRead(t *testing.T) {
	repo := repository(map[string]string{project.RememberIsolation: string(project.Verbatim)})
	repo.RunErr = errors.New("run.toml: line 3")

	questions := provision.NewQuestions(provision.QuestionsParams{Repository: repo})

	assert.Equal(t, domain.Answer[project.Isolation]{Value: project.Verbatim, Origin: domain.OriginRemembered}, questions.Isolation.Answer)
}

func TestTheFastForwardIsSkippedWhenNothingIsBehind(t *testing.T) {
	questions := provision.NewQuestions(provision.QuestionsParams{Given: provision.Given{Remember: []string{"fast_forward", "env_from", "env_from"}}, Repository: repository(nil)})

	assert.Equal(t, "nothing to fast-forward: the source is not behind origin", questions.FastForward.Skip)
	assert.False(t, questions.Remembers("fast_forward"))
	assert.Equal(t, []string{"env_from"}, questions.Given().Remember)
}

func TestTheSourceUpdateNamesTheBranchesItFastForwards(t *testing.T) {
	fields := provision.NewQuestions(provision.QuestionsParams{Repository: repository(nil), Behind: []string{"main", "topic"}}).Fields("")

	update, isConfirm := fields[2].(domain.ConfirmField)
	require.True(t, isConfirm)
	assert.Equal(t, domain.Option{Value: "true", Label: "Fast-forward main, topic to origin", Short: "fast-forward to origin"}, update.Yes)
	assert.Equal(t, domain.Option{Value: "false", Label: "Keep it as-is", Short: "keep as-is"}, update.No)
	assert.Empty(t, update.Skip)
}

func TestTheIsolationOptionsAreWordedForOneWorktreeOrSeveral(t *testing.T) {
	cases := map[string]struct {
		many bool
		want []string
	}{
		"one":     {false, []string{"isolated — its own ports and namespaces", "verbatim — the copied .env as is, one runs at a time"}},
		"several": {true, []string{"isolated — each its own ports and namespaces", "verbatim — the copied .env as is, one runs at a time"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			fields := provision.NewQuestions(provision.QuestionsParams{Repository: repository(nil), Many: c.many}).Fields("")

			isolation, isSelect := fields[1].(domain.SelectField)
			require.True(t, isSelect)
			assert.Equal(t, []string{"isolated", "verbatim"}, []string{isolation.Options[0].Value, isolation.Options[1].Value})
			assert.Equal(t, c.want, []string{isolation.Options[0].Label, isolation.Options[1].Label})
		})
	}
}

func TestAnUnknownStrategyAGivenIsolationOutsideTheListOrAFieldNoneRemembersIsAProblem(t *testing.T) {
	questions := provision.NewQuestions(provision.QuestionsParams{Given: provision.Given{EnvFrom: "copy", Isolation: "shared", Remember: []string{"env_from", "from"}}, Repository: repository(nil)})

	problems := questions.Problems()

	require.Len(t, problems, 3)
	assert.Equal(t, []string{"config", "example", "main", "parent"}, problems[0].Accepted)
	assert.Equal(t, "isolation", problems[1].Path)
	assert.Equal(t, "remember[1]", problems[2].Path)
}

func TestEachAnswerOfANewWorktreeReadsBackAsAnswerTakesIt(t *testing.T) {
	given := provision.Given{EnvFrom: provision.EnvFromConfig, Isolation: project.Verbatim, FastForward: new(false), Remember: []string{"isolation"}, Ask: true}

	got := provision.Given{}
	for _, path := range []string{"env_from", "isolation", "fast_forward", domain.PathRemember, domain.PathAsk} {
		var err error
		got, err = got.Answer(path, given.Values(path))
		require.NoError(t, err, path)
	}

	assert.Equal(t, given, got)
	_, err := got.Answer("branches", []string{"a"})
	assert.Error(t, err)
}

func TestTheAnswersToRememberAreKeyedAsConfigTomlKeepsThem(t *testing.T) {
	answers := provision.NewQuestions(provision.QuestionsParams{Given: provision.Given{EnvFrom: provision.EnvFromConfig, Isolation: project.Verbatim, FastForward: new(true)}, Repository: repository(nil)}).Answers()

	all, allFields := answers.Remembered([]string{"env_from", "isolation", "fast_forward"})
	assert.Equal(t, map[string]string{"env_strategy": "config", "isolation": "verbatim", "source_update": "ff"}, all)
	assert.Equal(t, []string{"env_from", "isolation", "fast_forward"}, allFields)
	one, oneField := answers.Remembered([]string{"isolation", "keep"})
	assert.Equal(t, map[string]string{"isolation": "verbatim"}, one)
	assert.Equal(t, []string{"isolation"}, oneField)
	assert.Equal(t, project.EnvExample, answers.EnvFrom())
}

func TestAnswersNoQuestionGaveAreRefusedBeforeAnythingIsMade(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	params := w.params(&recorder{})
	params.Worktree.Answers = provision.Answers{}

	outcome := w.service.Run(t.Context(), params)

	assert.Equal(t, domain.StatusFailed, outcome.Status)
	require.NotNil(t, outcome.Err)
	assert.Equal(t, domain.KindInternal, outcome.Err.Kind)
	assert.Equal(t, "provision.unasked", outcome.Err.Code)
	assert.NoDirExists(t, params.Worktree.Path)
}
