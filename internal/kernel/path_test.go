package kernel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestAFieldsPathIsItsJSONNameDottedThroughNestedStructs(t *testing.T) {
	assert.Equal(t,
		[]string{"branches", "from", "isolation", "push", "no_push", "url_host", "url_port", "decisions", "target.from"},
		kernel.Paths[request]())
}

func TestAnUnexportedOrIgnoredFieldHasNoPath(t *testing.T) {
	type hidden struct {
		Shown    string `json:"shown"`
		Ignored  string `json:"-"`
		Untagged string
		private  string
	}
	_ = hidden{}.private
	assert.Equal(t, []string{"shown", "Untagged"}, kernel.Paths[hidden]())
}

func TestARequestThatIsNotAStructHasNoPath(t *testing.T) {
	assert.Nil(t, kernel.Paths[string]())
}

func TestEachGoTypeReadsAsItsValue(t *testing.T) {
	req := request{
		Branches:  []string{"a", "b"},
		From:      "main",
		Isolation: "verbatim",
		Push:      true,
		Decisions: map[string]string{"PORT": "keep"},
		Target:    target{From: "dev"},
	}
	cases := map[string]kernel.Value{
		"branches":    kernel.List{"a", "b"},
		"from":        kernel.Text("main"),
		"isolation":   kernel.Text("verbatim"),
		"push":        kernel.Bool(true),
		"decisions":   kernel.Decisions{"PORT": "keep"},
		"target.from": kernel.Text("dev"),
		"url_host":    kernel.Text(""),
	}
	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			got, err := kernel.Get(req, path)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestSetWritesACopyAndLeavesTheRequestAsItWas(t *testing.T) {
	original := request{Branches: []string{"a"}}
	cases := []struct {
		path  string
		value kernel.Value
		want  request
	}{
		{"target.from", kernel.Text("main"), request{Branches: []string{"a"}, Target: target{From: "main"}}},
		{"isolation", kernel.Text("verbatim"), request{Branches: []string{"a"}, Isolation: "verbatim"}},
		{"push", kernel.Bool(true), request{Branches: []string{"a"}, Push: true}},
		{"branches", kernel.List{"x", "y"}, request{Branches: []string{"x", "y"}}},
		{"decisions", kernel.Decisions{"PORT": "take"}, request{Branches: []string{"a"}, Decisions: map[string]string{"PORT": "take"}}},
		{"branches", nil, request{}},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			got, err := kernel.Set(original, c.path, c.value)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
			assert.Equal(t, request{Branches: []string{"a"}}, original, "the original changed")
		})
	}
}

func TestSetRefusesAValueOfAnotherShapeThanTheField(t *testing.T) {
	for path, value := range map[string]kernel.Value{
		"from":      kernel.List{"main"},
		"push":      kernel.Text("true"),
		"branches":  kernel.Text("a"),
		"decisions": kernel.Bool(true),
	} {
		_, err := kernel.Set(request{}, path, value)
		assert.Error(t, err, "%T into %s", value, path)
	}
}

func TestAPathThatNamesNoFieldIsAnError(t *testing.T) {
	for _, path := range []string{"nope", "from.deeper", "target.nope", ""} {
		_, err := kernel.Get(request{}, path)
		assert.Error(t, err, "Get(%q)", path)
		_, err = kernel.Set(request{}, path, nil)
		assert.Error(t, err, "Set(%q)", path)
	}
}

func TestATypeNoValueCanHoldIsAnError(t *testing.T) {
	type counted struct {
		Count int `json:"count"`
	}
	_, err := kernel.Get(counted{}, "count")
	assert.Error(t, err)
	_, err = kernel.Set(counted{}, "count", kernel.Text("1"))
	assert.Error(t, err)
}

func TestAValueIsZeroWhenItHoldsNothing(t *testing.T) {
	for _, zero := range []kernel.Value{nil, kernel.Text(""), kernel.Bool(false), kernel.List{}, kernel.Decisions{}} {
		assert.True(t, kernel.IsZero(zero), "%#v", zero)
	}
	for _, set := range []kernel.Value{kernel.Text("x"), kernel.Bool(true), kernel.List{"x"}, kernel.Decisions{"k": "v"}} {
		assert.False(t, kernel.IsZero(set), "%#v", set)
	}
}

func TestWithinKnowsAnEntryAndANestedFieldBelongToTheirParent(t *testing.T) {
	cases := []struct {
		path, parent string
		within       bool
	}{
		{"branches", "branches", true},
		{"branches[2]", "branches", true},
		{"decisions[PORT]", "decisions", true},
		{"target.from", "target", true},
		{"branchesx", "branches", false},
		{"from", "target", false},
		{"target", "target.from", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.within, kernel.Within(kernel.PathIn{Path: c.path, Parent: c.parent}), "%s in %s", c.path, c.parent)
	}
}

func TestIndexPathNamesAnEntryOfAList(t *testing.T) {
	assert.Equal(t, "branches[2]", kernel.IndexPath("branches", 2))
}
