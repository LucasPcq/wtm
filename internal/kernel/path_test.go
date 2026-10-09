package kernel_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestAFieldsPathIsItsJSONNameDottedThroughNestedStructs(t *testing.T) {
	want := []string{"branches", "from", "isolation", "push", "no_push", "url_host", "url_port", "decisions", "target.from"}
	if got := kernel.Paths[request](); !slices.Equal(got, want) {
		t.Errorf("Paths = %v, want %v", got, want)
	}
}

func TestAnUnexportedOrIgnoredFieldHasNoPath(t *testing.T) {
	type hidden struct {
		Shown    string `json:"shown"`
		Ignored  string `json:"-"`
		Untagged string
		private  string
	}
	_ = hidden{}.private
	if got := kernel.Paths[hidden](); !slices.Equal(got, []string{"shown", "Untagged"}) {
		t.Errorf("Paths = %v", got)
	}
}

func TestARequestThatIsNotAStructHasNoPath(t *testing.T) {
	if got := kernel.Paths[string](); got != nil {
		t.Errorf("Paths = %v", got)
	}
}

func TestEachGoTypeReadsAsItsPartOfAValue(t *testing.T) {
	req := request{
		Branches:  []string{"a", "b"},
		From:      "main",
		Isolation: "verbatim",
		Push:      true,
		Decisions: map[string]string{"PORT": "keep"},
		Target:    target{From: "dev"},
	}
	cases := []struct {
		path string
		want kernel.Value
	}{
		{"branches", kernel.Value{List: []string{"a", "b"}}},
		{"from", kernel.Value{Text: "main"}},
		{"isolation", kernel.Value{Text: "verbatim"}},
		{"push", kernel.Value{Bool: true}},
		{"decisions", kernel.Value{Decisions: map[string]string{"PORT": "keep"}}},
		{"target.from", kernel.Value{Text: "dev"}},
		{"url_host", kernel.Value{}},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			got, err := kernel.Get(req, c.path)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Get = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestSetWritesACopyAndLeavesTheRequestAsItWas(t *testing.T) {
	original := request{Branches: []string{"a"}}
	cases := []struct {
		path  string
		value kernel.Value
		check func(request) bool
	}{
		{"target.from", kernel.Value{Text: "main"}, func(r request) bool { return r.Target.From == "main" }},
		{"isolation", kernel.Value{Text: "verbatim"}, func(r request) bool { return r.Isolation == "verbatim" }},
		{"push", kernel.Value{Bool: true}, func(r request) bool { return r.Push }},
		{"branches", kernel.Value{List: []string{"x", "y"}}, func(r request) bool { return slices.Equal(r.Branches, []string{"x", "y"}) }},
		{"decisions", kernel.Value{Decisions: map[string]string{"PORT": "take"}}, func(r request) bool { return r.Decisions["PORT"] == "take" }},
		{"branches", kernel.Value{}, func(r request) bool { return r.Branches == nil }},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			updated, err := kernel.Set(original, c.path, c.value)
			if err != nil {
				t.Fatal(err)
			}
			if !c.check(updated) {
				t.Errorf("Set(%q) = %+v", c.path, updated)
			}
			if !slices.Equal(original.Branches, []string{"a"}) || original.Target.From != "" || original.Push {
				t.Errorf("the original changed: %+v", original)
			}
		})
	}
}

func TestAPathThatNamesNoFieldIsAnError(t *testing.T) {
	for _, path := range []string{"nope", "from.deeper", "target.nope", ""} {
		if _, err := kernel.Get(request{}, path); err == nil {
			t.Errorf("Get(%q) found a field", path)
		}
		if _, err := kernel.Set(request{}, path, kernel.Value{}); err == nil {
			t.Errorf("Set(%q) found a field", path)
		}
	}
}

func TestATypeAValueCannotHoldIsAnError(t *testing.T) {
	type counted struct {
		Count int `json:"count"`
	}
	if _, err := kernel.Get(counted{}, "count"); err == nil {
		t.Error("an int read as a Value")
	}
	if _, err := kernel.Set(counted{}, "count", kernel.Value{Text: "1"}); err == nil {
		t.Error("an int written from a Value")
	}
}

func TestAValueIsZeroWhenNoneOfItsPartsIsSet(t *testing.T) {
	if !kernel.IsZero(kernel.Value{}) || !kernel.IsZero(kernel.Value{List: []string{}}) {
		t.Error("an empty value is not zero")
	}
	for _, value := range []kernel.Value{{Text: "x"}, {Bool: true}, {List: []string{"x"}}, {Decisions: map[string]string{"k": "v"}}} {
		if kernel.IsZero(value) {
			t.Errorf("%+v is zero", value)
		}
	}
}
