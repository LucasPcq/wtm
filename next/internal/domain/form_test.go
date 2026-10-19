package domain

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheFirstFieldAnsweringAPathSpellsItsFlagAndItsHelpItsUsage(t *testing.T) {
	form := Form{Fields: []Field{
		SelectField{FieldSpec: FieldSpec{Path: "to", Flag: "--to", Help: "Target branch"}},
		TextField{FieldSpec: FieldSpec{Path: "to", Flag: "--branch", Help: "Name for the branch"}},
		ConfirmField{FieldSpec: FieldSpec{Path: "keep", Flag: "--keep", Help: "Copy rather than move"}},
	}}

	flags, usages := form.Flags()

	assert.Equal(t, map[string]string{"to": "--to", "keep": "--keep"}, flags)
	assert.Equal(t, map[string]string{"to": "Target branch", "keep": "Copy rather than move"}, usages)
}

func TestAFormWithARememberableFieldHasRemember(t *testing.T) {
	form := Form{Fields: []Field{
		SelectField{FieldSpec: FieldSpec{Path: "isolation", Flag: "--isolation"}, Rememberable: true},
		ConfirmField{FieldSpec: FieldSpec{Path: "fast_forward", Flag: "--ff"}, Rememberable: true},
	}}

	flags, usages := form.Flags()

	assert.Equal(t, "--remember", flags[PathRemember])
	assert.Equal(t, "The answers to always use in this repository: isolation, fast_forward (repeatable)", usages[PathRemember])
}

func TestOnlyASelectOrAConfirmationIsRemembered(t *testing.T) {
	form := Form{Fields: []Field{
		TextListField{FieldSpec: FieldSpec{Path: "branches"}},
		SelectField{FieldSpec: FieldSpec{Path: "isolation"}, Rememberable: true, Remember: true},
		SelectField{FieldSpec: FieldSpec{Path: "from"}},
		ConfirmField{FieldSpec: FieldSpec{Path: "fast_forward"}, Rememberable: true},
	}}

	assert.Equal(t, []string{"isolation", "fast_forward"}, form.Rememberable())
	assert.Equal(t, []string{"isolation"}, form.Remembering())
}

func TestAFieldsProblemsAreThoseOfItAndOfItsEntries(t *testing.T) {
	problems := []FieldError{{Path: "branches"}, {Path: "branches[1]"}, {Path: "branches_extra"}, {Path: "from"}}

	assert.Equal(t, []FieldError{{Path: "branches"}, {Path: "branches[1]"}}, ProblemsOf(problems, "branches"))
}

func TestEachKindAnswersAsItsCommandsRequestTakesIt(t *testing.T) {
	assert.Equal(t, []string{"main"}, SelectField{Value: "main"}.Answer())
	assert.Equal(t, []string{"a", "b"}, MultiSelectField{Picked: []string{"a", "b"}}.Answer())
	assert.Equal(t, []string{"feat/x"}, TextField{Value: "feat/x"}.Answer())
	assert.Equal(t, []string{"x", "y"}, TextListField{Entries: []string{"x", "y"}}.Answer())
	assert.Equal(t, []string{"true"}, ConfirmField{Value: true}.Answer())
}

func TestAConfirmationTakesTrueOrFalseOnly(t *testing.T) {
	yes, err := Yes("keep", ConfirmField{Value: true}.Answer())
	require.NoError(t, err)
	assert.True(t, yes)

	for _, values := range [][]string{{"yes"}, {""}, {}, {"true", "false"}} {
		_, err := Yes("keep", values)
		failure, ok := errors.AsType[*Error](err)
		require.True(t, ok, values)
		assert.Equal(t, "keep", failure.Fields[0].Path, values)
	}
}

func TestOneTakesASingleValueOfItsType(t *testing.T) {
	value, err := One[Origin]("origin", []string{"config"})
	require.NoError(t, err)
	assert.Equal(t, OriginConfig, value)

	_, err = One[string]("from", []string{"a", "b"})
	failure, ok := errors.AsType[*Error](err)
	require.True(t, ok)
	assert.Equal(t, CodeOneValue, failure.Fields[0].Code)
}

func TestAFormIsEncodedWithTheKindOfEachField(t *testing.T) {
	form := Form{
		Fields: []Field{
			TextListField{FieldSpec: FieldSpec{Path: "branches", Flag: "<branches>", Label: "Branches"}, Entries: []string{"feat/x"}},
			ConfirmField{FieldSpec: FieldSpec{Path: "keep", Flag: "--keep", Label: "Keep", Skip: "not asked"}, Yes: Option{Value: "true", Label: "Copy", Short: "copy"}, No: Option{Value: "false", Label: "Move"}, NoFirst: true},
			SelectField{FieldSpec: FieldSpec{Path: "isolation", Flag: "--isolation", Label: "Isolation", Origin: OriginRemembered}, Value: "shared", Options: []Option{{Value: "shared", Label: "Shared", Badge: "remembered"}}, Remembered: "shared", Rememberable: true, Refreshable: true},
			MultiSelectField{FieldSpec: FieldSpec{Path: "files", Flag: "--files", Label: "Files"}, Empty: "Pick one"},
			TextField{FieldSpec: FieldSpec{Path: "to", Flag: "--to", Label: "Target", Help: "Target branch"}, Value: "feat/y"},
		},
		Problems: []FieldError{{Path: "branches[0]", Code: "required", Template: "{path} is required"}},
	}

	data, err := json.Marshal(form)

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"fields": [
			{"kind": "text_list", "path": "branches", "flag": "<branches>", "label": "Branches", "help": "", "entries": ["feat/x"], "options": []},
			{"kind": "confirm", "path": "keep", "flag": "--keep", "label": "Keep", "help": "", "skip": "not asked", "value": false, "yes": {"value": "true", "label": "Copy", "short": "copy"}, "no": {"value": "false", "label": "Move"}, "no_first": true, "rememberable": false, "remember": false},
			{"kind": "select", "path": "isolation", "flag": "--isolation", "label": "Isolation", "help": "", "origin": "remembered", "value": "shared", "options": [{"value": "shared", "label": "Shared", "badge": "remembered"}], "remembered": "shared", "rememberable": true, "remember": false, "refreshable": true},
			{"kind": "multi_select", "path": "files", "flag": "--files", "label": "Files", "help": "", "picked": [], "options": [], "empty": "Pick one"},
			{"kind": "text", "path": "to", "flag": "--to", "label": "Target", "help": "Target branch", "value": "feat/y"}
		],
		"problems": [{"path": "branches[0]", "code": "required", "flag": "branches[0]", "message": "branches[0] is required"}]
	}`, string(data))
}

func TestAFormWithoutProblemsHasAnEmptyList(t *testing.T) {
	data, err := json.Marshal(Form{})

	require.NoError(t, err)
	assert.JSONEq(t, `{"fields": [], "problems": []}`, string(data))
}

func TestAFormFoldsIntoTheRequestItsFieldsAnswer(t *testing.T) {
	form := Form{Fields: []Field{
		TextListField{FieldSpec: FieldSpec{Path: "branches"}, Entries: []string{"feat/a"}},
		SelectField{FieldSpec: FieldSpec{Path: "isolation"}, Value: "verbatim", Rememberable: true, Remember: true},
	}}

	req, err := Fold(answers{}, form.Answers())

	require.NoError(t, err)
	assert.Equal(t, answers{"branches": {"feat/a"}, "isolation": {"verbatim"}, PathRemember: {"isolation"}}, req)
}

func TestAFormWithNothingToRememberAnswersNoRemember(t *testing.T) {
	form := Form{Fields: []Field{ConfirmField{FieldSpec: FieldSpec{Path: "with_prs"}}}}

	assert.Equal(t, []Answered{{Path: "with_prs", Values: form.Fields[0].Answer()}}, form.Answers())
}

func TestARequestAnswersEachPathOnceThenWhatItRemembersAndAsks(t *testing.T) {
	form := Form{Fields: []Field{
		SelectField{FieldSpec: FieldSpec{Path: "to"}},
		TextField{FieldSpec: FieldSpec{Path: "to"}},
		TextField{FieldSpec: FieldSpec{Path: "from"}},
	}}
	req := answers{"to": {"feat/x"}, PathRemember: {"to"}, PathAsk: {"true"}}

	assert.Equal(t, []Answered{{Path: "to", Values: []string{"feat/x"}}, {Path: PathRemember, Values: []string{"to"}}, {Path: PathAsk, Values: []string{"true"}}}, AnswersOf(form, req))
}

func TestAFoldStopsAtTheFirstAnswerRefused(t *testing.T) {
	req, err := Fold(answers{}, []Answered{{Path: "from", Values: []string{"main"}}, {Path: "nope"}, {Path: "to", Values: []string{"x"}}})

	require.Error(t, err)
	assert.Equal(t, answers{"from": {"main"}}, req)
}
