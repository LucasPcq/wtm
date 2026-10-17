package domain

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
)

func TestAnIsolationLeftUnansweredIsTheRememberedOneElseTheRunsElseIsolated(t *testing.T) {
	isolating := project.Run{EnvPorts: []project.EnvPortLink{{}}}
	cases := map[string]struct {
		remembered project.Isolation
		run        project.Run
		want       Answer[project.Isolation]
	}{
		"remembered":                        {project.Verbatim, isolating, Answer[project.Isolation]{Value: project.Verbatim, Origin: OriginRemembered}},
		"remembered, nothing to isolate":    {project.Verbatim, project.Run{Isolation: project.Isolated}, Answer[project.Isolation]{Value: project.Isolated, Origin: OriginConfig}},
		"the run's":                         {"", project.Run{Isolation: project.Verbatim}, Answer[project.Isolation]{Value: project.Verbatim, Origin: OriginConfig}},
		"nothing remembered nor in the run": {"", isolating, Answer[project.Isolation]{Value: project.Isolated, Origin: OriginDefault}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, IsolationAnswer(c.remembered, c.run))
		})
	}
}

func TestTheFastForwardIsTheRequestsElseNoneWhenSkippedElseTheRememberedElseNo(t *testing.T) {
	cases := map[string]struct {
		params FastForwardParams
		want   Confirm
	}{
		"given":              {FastForwardParams{Given: new(true), Skip: "behind nothing", Remembered: project.SourceUpdateKeep}, Confirm{Answer: Answer[bool]{Value: true, Origin: OriginRequest}, Skip: "behind nothing"}},
		"skipped":            {FastForwardParams{Skip: "behind nothing", Remembered: project.SourceUpdateFastForward}, Confirm{Skip: "behind nothing"}},
		"remembered":         {FastForwardParams{Remembered: project.SourceUpdateFastForward}, Confirm{Answer: Answer[bool]{Value: true, Origin: OriginRemembered}}},
		"remembered to keep": {FastForwardParams{Remembered: project.SourceUpdateKeep}, Confirm{Answer: Answer[bool]{Origin: OriginRemembered}}},
		"default":            {FastForwardParams{}, Confirm{Answer: Answer[bool]{Origin: OriginDefault}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, FastForwardAnswer(c.params))
		})
	}
}

func TestAValueOutsideItsChoicesIsAFieldError(t *testing.T) {
	accepted := []string{"a", "b"}

	assert.Nil(t, OneOf(OneOfParams{Path: "mode", Value: "a", Accepted: accepted}))
	assert.Nil(t, OneOf(OneOfParams{Path: "mode", Accepted: accepted}))
	assert.Equal(t, []FieldError{{Path: "mode", Code: CodeOneOf, Template: "{path} must be one of {accepted}, not {value}", Params: Params{"value": "c"}, Accepted: accepted}}, OneOf(OneOfParams{Path: "mode", Value: "c", Accepted: accepted}))
}

func TestNamesSpellsEachValueInOrder(t *testing.T) {
	assert.Equal(t, []string{"isolated", "verbatim"}, Names(project.Isolations()))
}
