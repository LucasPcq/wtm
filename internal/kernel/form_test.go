package kernel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestEachFieldEndsProvidedDefaultedSkippedOrMissing(t *testing.T) {
	form := evaluate(t, kernel.EvaluateParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{
			{Spec: kernel.FieldSpec{Path: "branches", Type: kernel.FieldTextList}},
			{Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect}, Default: baseFallback},
			{
				Spec: kernel.FieldSpec{Path: "isolation", Type: kernel.FieldSelect},
				Skip: func(request, facts) (bool, kernel.Code) { return true, "test.nothing_to_isolate" },
			},
			{Spec: kernel.FieldSpec{Path: "url_host", Type: kernel.FieldText}},
		},
		Request: request{Branches: []string{"feat/x"}},
		Facts:   facts{Base: "main"},
	})
	assert.Equal(t, []string{"branches:provided", "from:defaulted", "isolation:skipped", "url_host:missing"}, states(form))
	assert.Equal(t, kernel.OriginRequest, form.States[0].Origin)
	assert.Equal(t, kernel.OriginConfig, form.States[1].Origin)
	assert.Equal(t, kernel.Code("test.nothing_to_isolate"), form.States[2].SkipReason)
	assert.True(t, form.Complete, "a missing optional field keeps the form complete")
}

func TestADefaultIsWrittenIntoTheRequest(t *testing.T) {
	form := evaluate(t, kernel.EvaluateParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{{Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect}, Default: baseFallback}},
		Facts:  facts{Base: "main"},
	})
	assert.Equal(t, "main", form.Request.From)
	assert.Equal(t, kernel.Text("main"), form.States[0].Value)
}

func TestAValueTheRequestCarriesWinsOverTheDefault(t *testing.T) {
	form := evaluate(t, kernel.EvaluateParams[request, facts]{
		Fields:  []kernel.FieldDef[request, facts]{{Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect}, Default: baseFallback}},
		Request: request{From: "dev"},
		Facts:   facts{Base: "main"},
	})
	assert.Equal(t, "dev", form.Request.From)
	assert.Equal(t, kernel.FieldProvided, form.States[0].Status)
}

func TestADefaultWithNoSafeValueLeavesARequiredFieldMissing(t *testing.T) {
	form := evaluate(t, kernel.EvaluateParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{{Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect, Required: true}, Default: baseFallback}},
	})
	assert.Equal(t, []string{"from:missing"}, states(form))
	assert.False(t, form.Complete)
	assert.Equal(t, []kernel.FieldError{{Path: "from", Code: kernel.CodeRequired}}, form.Errors)
}

func TestASkipSeesTheDefaultsOfTheFieldsBeforeIt(t *testing.T) {
	form := evaluate(t, kernel.EvaluateParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{
			{Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect}, Default: baseFallback},
			{
				Spec: kernel.FieldSpec{Path: "isolation", Type: kernel.FieldSelect, DependsOn: []string{"from"}},
				Skip: func(req request, _ facts) (bool, kernel.Code) { return req.From == "main", "test.from_main" },
			},
		},
		Facts: facts{Base: "main"},
	})
	assert.Equal(t, []string{"from:defaulted", "isolation:skipped"}, states(form))
}

func TestASkippedFieldIsNeitherRequiredNorChecked(t *testing.T) {
	form := evaluate(t, kernel.EvaluateParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{{
			Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldText, Required: true, Constraints: kernel.Constraints{MinLen: 10}},
			Skip: func(request, facts) (bool, kernel.Code) { return true, "test.skip" },
			Validate: func(request, facts) []kernel.FieldError {
				return []kernel.FieldError{{Path: "from", Code: "test.never"}}
			},
		}},
		Request: request{From: "x"},
	})
	assert.True(t, form.Complete)
	assert.Empty(t, form.Errors)
}

func TestValidateRunsOnlyOnceTheConstraintsHold(t *testing.T) {
	field := kernel.FieldDef[request, facts]{
		Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldText, Constraints: kernel.Constraints{MinLen: 3}},
		Validate: func(req request, _ facts) []kernel.FieldError {
			return []kernel.FieldError{{Path: "from", Code: kernel.CodeNotFound, Params: kernel.Params{kernel.ParamValue: req.From}}}
		},
	}
	short := evaluate(t, kernel.EvaluateParams[request, facts]{Fields: []kernel.FieldDef[request, facts]{field}, Request: request{From: "x"}})
	long := evaluate(t, kernel.EvaluateParams[request, facts]{Fields: []kernel.FieldDef[request, facts]{field}, Request: request{From: "ghost"}})
	require.Len(t, short.Errors, 1)
	assert.Equal(t, kernel.CodeTooShort, short.Errors[0].Code)
	require.Len(t, long.Errors, 1)
	assert.Equal(t, kernel.CodeNotFound, long.Errors[0].Code)
}

func TestTheRulesRunOverTheRequestWithItsDefaultsAndAttachToTheirField(t *testing.T) {
	form := evaluate(t, kernel.EvaluateParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{
			{Spec: kernel.FieldSpec{Path: "branches", Type: kernel.FieldTextList}},
			{Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect}, Default: baseFallback},
		},
		Rules: rules{}.
			Distinct("branches").
			NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "branches"}),
		Request: request{Branches: []string{"main", "main"}},
		Facts:   facts{Base: "main"},
	})
	assert.False(t, form.Complete)
	assert.Len(t, form.Errors, 2)
	assert.Equal(t, []kernel.FieldError{{Path: "branches[1]", Code: kernel.CodeDistinct, Params: kernel.Params{kernel.ParamValue: "main"}}},
		form.States[0].Errors, "an entry's error belongs to its list")
	require.Len(t, form.States[1].Errors, 1, "the default from = main is its own parent")
	assert.Equal(t, kernel.CodeSelfParent, form.States[1].Errors[0].Code)
}

func TestAFieldOnAPathThatNamesNoFieldIsAnError(t *testing.T) {
	_, err := kernel.Evaluate(kernel.EvaluateParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{{Spec: kernel.FieldSpec{Path: "nope"}}},
	})
	assert.Error(t, err)
}

func TestADefaultOfAnotherShapeThanTheFieldIsAnError(t *testing.T) {
	_, err := kernel.Evaluate(kernel.EvaluateParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{{
			Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldText},
			Default: func(request, facts) (kernel.Fallback, bool) {
				return kernel.Fallback{Value: kernel.List{"main"}, Origin: kernel.OriginDefault}, true
			},
		}},
	})
	assert.Error(t, err)
}
