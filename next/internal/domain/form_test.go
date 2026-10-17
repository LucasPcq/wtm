package domain

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheFirstFieldAnsweringAPathSpellsItsFlagAndUsage(t *testing.T) {
	form := Form{Fields: []Field{
		SelectField{FieldSpec: FieldSpec{Path: "to", Flag: "--to", Help: "Where to move", Usage: "Target branch"}},
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
	yes, err := Yes(AnswerParams{Path: "keep", Values: ConfirmField{Value: true}.Answer()})
	require.NoError(t, err)
	assert.True(t, yes)

	for _, values := range [][]string{{"yes"}, {""}, {}, {"true", "false"}} {
		_, err := Yes(AnswerParams{Path: "keep", Values: values})
		failure, ok := errors.AsType[*Error](err)
		require.True(t, ok, values)
		assert.Equal(t, "keep", failure.Fields[0].Path, values)
	}
}

func TestOneTakesASingleValueOfItsType(t *testing.T) {
	value, err := One[Origin](AnswerParams{Path: "origin", Values: []string{"config"}})
	require.NoError(t, err)
	assert.Equal(t, OriginConfig, value)

	_, err = One[string](AnswerParams{Path: "from", Values: []string{"a", "b"}})
	failure, ok := errors.AsType[*Error](err)
	require.True(t, ok)
	assert.Equal(t, CodeOneValue, failure.Fields[0].Code)
}

func TestAFormIsEncodedWithTheKindOfEachField(t *testing.T) {
	form := Form{
		Fields: []Field{
			TextListField{FieldSpec: FieldSpec{Path: "branches", Flag: "<branches>", Label: "Branches"}, Entries: []string{"feat/x"}},
			ConfirmField{FieldSpec: FieldSpec{Path: "keep", Flag: "--keep", Label: "Keep", Skip: "not asked"}, Yes: Option{Value: "true", Label: "Copy"}, No: Option{Value: "false", Label: "Move"}},
			SelectField{FieldSpec: FieldSpec{Path: "isolation", Flag: "--isolation", Label: "Isolation", Origin: OriginRemembered}, Value: "shared", Options: []Option{{Value: "shared", Label: "Shared", Badge: "remembered"}}, Remembered: "shared", Rememberable: true},
			MultiSelectField{FieldSpec: FieldSpec{Path: "files", Flag: "--files", Label: "Files"}},
			TextField{FieldSpec: FieldSpec{Path: "to", Flag: "--to", Label: "Target", Usage: "Target branch"}, Value: "feat/y"},
		},
		Problems: []FieldError{{Path: "branches[0]", Code: "required", Template: "{path} is required"}},
	}

	data, err := json.Marshal(form)

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"fields": [
			{"kind": "text_list", "path": "branches", "flag": "<branches>", "label": "Branches", "help": "", "entries": ["feat/x"], "options": []},
			{"kind": "confirm", "path": "keep", "flag": "--keep", "label": "Keep", "help": "", "skip": "not asked", "value": false, "yes": {"value": "true", "label": "Copy"}, "no": {"value": "false", "label": "Move"}, "rememberable": false, "remember": false},
			{"kind": "select", "path": "isolation", "flag": "--isolation", "label": "Isolation", "help": "", "origin": "remembered", "value": "shared", "options": [{"value": "shared", "label": "Shared", "badge": "remembered"}], "remembered": "shared", "rememberable": true, "remember": false},
			{"kind": "multi_select", "path": "files", "flag": "--files", "label": "Files", "help": "", "picked": [], "options": []},
			{"kind": "text", "path": "to", "flag": "--to", "label": "Target", "help": "", "usage": "Target branch", "value": "feat/y"}
		],
		"problems": [{"path": "branches[0]", "code": "required", "flag": "branches[0]", "message": "branches[0] is required"}]
	}`, string(data))
}

func TestAFormWithoutProblemsHasAnEmptyList(t *testing.T) {
	data, err := json.Marshal(Form{})

	require.NoError(t, err)
	assert.JSONEq(t, `{"fields": [], "problems": []}`, string(data))
}
