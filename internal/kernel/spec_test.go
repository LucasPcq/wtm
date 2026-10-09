package kernel_test

import (
	"reflect"
	"testing"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func checkSpec(t *testing.T, req request, specs ...kernel.FieldSpec) []kernel.FieldError {
	t.Helper()
	problems, err := kernel.CheckSpec(kernel.CheckSpecParams[request]{Request: req, Specs: specs})
	if err != nil {
		t.Fatal(err)
	}
	return problems
}

func TestEachConstraintRefusesWithItsCode(t *testing.T) {
	cases := []struct {
		name string
		spec kernel.FieldSpec
		req  request
		want []kernel.FieldError
	}{
		{
			name: "a required field with no value",
			spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldText, Required: true},
			want: []kernel.FieldError{{Path: "from", Code: kernel.CodeRequired}},
		},
		{
			name: "a value outside the enum",
			spec: kernel.FieldSpec{Path: "isolation", Type: kernel.FieldSelect, Constraints: kernel.Constraints{Enum: []string{"isolated", "verbatim"}}},
			req:  request{Isolation: "shared"},
			want: []kernel.FieldError{{Path: "isolation", Code: kernel.CodeOneOf, Params: map[string]string{kernel.ParamValue: "shared"}, Accepted: []string{"isolated", "verbatim"}}},
		},
		{
			name: "a text too short",
			spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldText, Constraints: kernel.Constraints{MinLen: 3}},
			req:  request{From: "ab"},
			want: []kernel.FieldError{{Path: "from", Code: kernel.CodeTooShort, Params: map[string]string{kernel.ParamLimit: "3"}}},
		},
		{
			name: "a text too long",
			spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldText, Constraints: kernel.Constraints{MaxLen: 3}},
			req:  request{From: "main"},
			want: []kernel.FieldError{{Path: "from", Code: kernel.CodeTooLong, Params: map[string]string{kernel.ParamLimit: "3"}}},
		},
		{
			name: "a text that does not match the pattern",
			spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldText, Constraints: kernel.Constraints{Pattern: `^[a-z/]+$`}},
			req:  request{From: "Main"},
			want: []kernel.FieldError{{Path: "from", Code: kernel.CodePattern, Params: map[string]string{kernel.ParamValue: "Main", kernel.ParamPattern: `^[a-z/]+$`}}},
		},
		{
			name: "a list with too few entries",
			spec: kernel.FieldSpec{Path: "branches", Type: kernel.FieldTextList, Constraints: kernel.Constraints{MinItems: 2}},
			req:  request{Branches: []string{"a"}},
			want: []kernel.FieldError{{Path: "branches", Code: kernel.CodeTooFew, Params: map[string]string{kernel.ParamLimit: "2"}}},
		},
		{
			name: "a list with too many entries",
			spec: kernel.FieldSpec{Path: "branches", Type: kernel.FieldTextList, Constraints: kernel.Constraints{MaxItems: 1}},
			req:  request{Branches: []string{"a", "b"}},
			want: []kernel.FieldError{{Path: "branches", Code: kernel.CodeTooMany, Params: map[string]string{kernel.ParamLimit: "1"}}},
		},
		{
			name: "a list checks each entry, by its index",
			spec: kernel.FieldSpec{Path: "branches", Type: kernel.FieldTextList, Constraints: kernel.Constraints{MinLen: 2}},
			req:  request{Branches: []string{"ok", "x", "fine", "y"}},
			want: []kernel.FieldError{
				{Path: "branches[1]", Code: kernel.CodeTooShort, Params: map[string]string{kernel.ParamLimit: "2"}},
				{Path: "branches[3]", Code: kernel.CodeTooShort, Params: map[string]string{kernel.ParamLimit: "2"}},
			},
		},
		{
			name: "decisions check each value, by its key, in key order",
			spec: kernel.FieldSpec{Path: "decisions", Type: kernel.FieldDecisions, Constraints: kernel.Constraints{Enum: []string{"keep", "take"}}},
			req:  request{Decisions: map[string]string{"PORT": "drop", "API": "lose", "DB": "keep"}},
			want: []kernel.FieldError{
				{Path: "decisions[API]", Code: kernel.CodeOneOf, Params: map[string]string{kernel.ParamValue: "lose"}, Accepted: []string{"keep", "take"}},
				{Path: "decisions[PORT]", Code: kernel.CodeOneOf, Params: map[string]string{kernel.ParamValue: "drop"}, Accepted: []string{"keep", "take"}},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := checkSpec(t, c.req, c.spec); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got  %+v\nwant %+v", got, c.want)
			}
		})
	}
}

func TestAValueThatHoldsEveryConstraintPasses(t *testing.T) {
	spec := kernel.FieldSpec{Path: "branches", Type: kernel.FieldTextList, Required: true, Constraints: kernel.Constraints{
		MinItems: 1, MaxItems: 3, MinLen: 2, MaxLen: 20, Pattern: `^[a-z/]+$`,
	}}
	if got := checkSpec(t, request{Branches: []string{"feat/x", "fix/y"}}, spec); got != nil {
		t.Errorf("got %+v", got)
	}
}

func TestAnEmptyOptionalFieldIsNotChecked(t *testing.T) {
	spec := kernel.FieldSpec{Path: "from", Type: kernel.FieldText, Constraints: kernel.Constraints{MinLen: 3, Enum: []string{"main"}}}
	if got := checkSpec(t, request{}, spec); got != nil {
		t.Errorf("got %+v", got)
	}
}

func TestABoolHasNothingToCheckButRequired(t *testing.T) {
	spec := kernel.FieldSpec{Path: "push", Type: kernel.FieldBool, Constraints: kernel.Constraints{Enum: []string{"never"}}}
	if got := checkSpec(t, request{Push: true}, spec); got != nil {
		t.Errorf("got %+v", got)
	}
}

func TestAConstraintTheCommandGotWrongIsAnErrorNotAFieldError(t *testing.T) {
	broken := []kernel.FieldSpec{
		{Path: "from", Type: kernel.FieldText, Constraints: kernel.Constraints{Pattern: "("}},
		{Path: "nope", Type: kernel.FieldText},
	}
	for _, spec := range broken {
		if _, err := kernel.CheckSpec(kernel.CheckSpecParams[request]{Request: request{From: "x"}, Specs: []kernel.FieldSpec{spec}}); err == nil {
			t.Errorf("%+v passed", spec)
		}
	}
}

func TestIndexPathNamesAnEntryOfAList(t *testing.T) {
	if got := kernel.IndexPath("branches", 2); got != "branches[2]" {
		t.Errorf("got %q", got)
	}
}
