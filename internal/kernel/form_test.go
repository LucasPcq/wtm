package kernel_test

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func evaluate(t *testing.T, params kernel.EvaluateParams[request, facts]) kernel.Form[request] {
	t.Helper()
	form, err := kernel.Evaluate(params)
	if err != nil {
		t.Fatal(err)
	}
	return form
}

func baseFallback(_ request, f facts) (kernel.Fallback, bool) {
	return kernel.Fallback{Value: kernel.Value{Text: f.Base}, Origin: kernel.OriginConfig}, f.Base != ""
}

func statusesOf(form kernel.Form[request]) []string {
	out := make([]string, 0, len(form.States))
	for _, state := range form.States {
		out = append(out, state.Path+":"+string(state.Status))
	}
	return out
}

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
	want := []string{"branches:provided", "from:defaulted", "isolation:skipped", "url_host:missing"}
	if got := statusesOf(form); !slices.Equal(got, want) {
		t.Errorf("states = %v, want %v", got, want)
	}
	if form.States[0].Origin != kernel.OriginRequest || form.States[1].Origin != kernel.OriginConfig {
		t.Errorf("origins = %s, %s", form.States[0].Origin, form.States[1].Origin)
	}
	if form.States[2].SkipReason != "test.nothing_to_isolate" {
		t.Errorf("skip reason = %q", form.States[2].SkipReason)
	}
	if !form.Complete {
		t.Errorf("a missing optional field keeps the form complete: %+v", form.Errors)
	}
}

func TestADefaultIsWrittenIntoTheRequest(t *testing.T) {
	form := evaluate(t, kernel.EvaluateParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{{Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect}, Default: baseFallback}},
		Facts:  facts{Base: "main"},
	})
	if form.Request.From != "main" || form.States[0].Value.Text != "main" {
		t.Errorf("request = %+v, state = %+v", form.Request, form.States[0])
	}
}

func TestAValueTheRequestCarriesWinsOverTheDefault(t *testing.T) {
	form := evaluate(t, kernel.EvaluateParams[request, facts]{
		Fields:  []kernel.FieldDef[request, facts]{{Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect}, Default: baseFallback}},
		Request: request{From: "dev"},
		Facts:   facts{Base: "main"},
	})
	if form.Request.From != "dev" || form.States[0].Status != kernel.FieldProvided {
		t.Errorf("form = %+v", form)
	}
}

func TestADefaultThatHasNoSafeValueLeavesTheFieldMissing(t *testing.T) {
	form := evaluate(t, kernel.EvaluateParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{{Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect, Required: true}, Default: baseFallback}},
	})
	if form.States[0].Status != kernel.FieldMissing || form.Complete || form.Errors[0].Code != kernel.CodeRequired {
		t.Errorf("form = %+v", form)
	}
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
	if form.States[1].Status != kernel.FieldSkipped {
		t.Errorf("states = %v", statusesOf(form))
	}
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
	if !form.Complete {
		t.Errorf("errors = %+v", form.Errors)
	}
}

func TestValidateRunsOnlyOnceTheConstraintsHold(t *testing.T) {
	notFound := func(req request, _ facts) []kernel.FieldError {
		return []kernel.FieldError{{Path: "from", Code: kernel.CodeNotFound, Params: map[string]string{kernel.ParamValue: req.From}}}
	}
	field := kernel.FieldDef[request, facts]{
		Spec:     kernel.FieldSpec{Path: "from", Type: kernel.FieldText, Constraints: kernel.Constraints{MinLen: 3}},
		Validate: notFound,
	}
	short := evaluate(t, kernel.EvaluateParams[request, facts]{Fields: []kernel.FieldDef[request, facts]{field}, Request: request{From: "x"}})
	long := evaluate(t, kernel.EvaluateParams[request, facts]{Fields: []kernel.FieldDef[request, facts]{field}, Request: request{From: "ghost"}})
	if len(short.Errors) != 1 || short.Errors[0].Code != kernel.CodeTooShort {
		t.Errorf("short = %+v", short.Errors)
	}
	if len(long.Errors) != 1 || long.Errors[0].Code != kernel.CodeNotFound {
		t.Errorf("long = %+v", long.Errors)
	}
}

func TestTheRulesRunOverTheRequestWithItsDefaultsAndAttachToTheirField(t *testing.T) {
	form := evaluate(t, kernel.EvaluateParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{
			{Spec: kernel.FieldSpec{Path: "branches", Type: kernel.FieldTextList}},
			{Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect}, Default: baseFallback},
		},
		Rules: on{}.
			Distinct("branches").
			NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "branches"}),
		Request: request{Branches: []string{"main", "main"}},
		Facts:   facts{Base: "main"},
	})
	if form.Complete || len(form.Errors) != 2 {
		t.Fatalf("errors = %+v", form.Errors)
	}
	if len(form.States[0].Errors) != 1 || form.States[0].Errors[0].Path != "branches[1]" {
		t.Errorf("an entry's error belongs to its list: %+v", form.States[0])
	}
	if len(form.States[1].Errors) != 1 || form.States[1].Errors[0].Code != kernel.CodeSelfParent {
		t.Errorf("the default from = main is its own parent: %+v", form.States[1])
	}
}

func TestAFieldOnAPathThatNamesNoFieldIsAnError(t *testing.T) {
	_, err := kernel.Evaluate(kernel.EvaluateParams[request, facts]{
		Fields: []kernel.FieldDef[request, facts]{{Spec: kernel.FieldSpec{Path: "nope"}}},
	})
	if err == nil {
		t.Error("an unknown path passed")
	}
}
