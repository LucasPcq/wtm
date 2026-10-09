package kernel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestEachCombinatorRefusesWithItsCode(t *testing.T) {
	exclusive := rules{}.Exclusive("push", "no_push", "url_host")
	oneOf := rules{}.OneOf("push", "no_push")
	requires := rules{}.Requires(kernel.RequiresRule{Field: "url_host", Needs: []string{"url_port", "from"}})
	requiredWhen := rules{}.RequiredWhen("from", branchExists)
	distinct := rules{}.Distinct("branches")
	notSelfParent := rules{}.NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "branches"})
	notSelfParentOfText := rules{}.NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "target.from"})

	cases := []struct {
		name  string
		rules rules
		req   request
		facts facts
		want  []kernel.FieldError
	}{
		{name: "exclusive: none set", rules: exclusive},
		{name: "exclusive: one set", rules: exclusive, req: request{NoPush: true}},
		{
			name: "exclusive: two set, the second is blamed", rules: exclusive,
			req:  request{Push: true, NoPush: true},
			want: []kernel.FieldError{{Path: "no_push", Code: kernel.CodeExclusiveWith, With: []string{"push"}}},
		},
		{
			name: "exclusive: three set, each after the first is blamed, naming the others", rules: exclusive,
			req: request{Push: true, NoPush: true, URLHost: "api"},
			want: []kernel.FieldError{
				{Path: "no_push", Code: kernel.CodeExclusiveWith, With: []string{"push", "url_host"}},
				{Path: "url_host", Code: kernel.CodeExclusiveWith, With: []string{"push", "no_push"}},
			},
		},
		{
			name: "one of: none set, one of them is required", rules: oneOf,
			want: []kernel.FieldError{{Path: "push", Code: kernel.CodeRequiredOneOf, With: []string{"no_push"}}},
		},
		{name: "one of: one set", rules: oneOf, req: request{NoPush: true}},
		{
			name: "one of: both set", rules: oneOf,
			req:  request{Push: true, NoPush: true},
			want: []kernel.FieldError{{Path: "no_push", Code: kernel.CodeExclusiveWith, With: []string{"push"}}},
		},
		{name: "requires: the field unset needs nothing", rules: requires, req: request{URLPort: "8080"}},
		{name: "requires: every need set", rules: requires, req: request{URLHost: "api", URLPort: "8080", From: "main"}},
		{
			name: "requires: the needs missing are named", rules: requires,
			req:  request{URLHost: "api", URLPort: "8080"},
			want: []kernel.FieldError{{Path: "url_host", Code: kernel.CodeRequires, With: []string{"from"}}},
		},
		{
			name: "required when: the condition holds and the field is empty, and it says why", rules: requiredWhen,
			req:   request{Branches: []string{"feat/x"}},
			facts: facts{Existing: []string{"feat/x"}},
			want:  []kernel.FieldError{{Path: "from", Code: kernel.CodeRequired, Params: kernel.Params{kernel.ParamBecause: "test.branch_exists"}}},
		},
		{
			name: "required when: the condition holds and the field is set", rules: requiredWhen,
			req:   request{Branches: []string{"feat/x"}, From: "main"},
			facts: facts{Existing: []string{"feat/x"}},
		},
		{
			name: "required when: the condition does not hold", rules: requiredWhen,
			req:   request{Branches: []string{"feat/new"}},
			facts: facts{Existing: []string{"feat/x"}},
		},
		{name: "distinct: all different", rules: distinct, req: request{Branches: []string{"a", "b"}}},
		{
			name: "distinct: each repeat is blamed at its index", rules: distinct,
			req: request{Branches: []string{"a", "b", "a", "b", "a"}},
			want: []kernel.FieldError{
				{Path: "branches[2]", Code: kernel.CodeDistinct, Params: kernel.Params{kernel.ParamValue: "a"}},
				{Path: "branches[3]", Code: kernel.CodeDistinct, Params: kernel.Params{kernel.ParamValue: "b"}},
				{Path: "branches[4]", Code: kernel.CodeDistinct, Params: kernel.Params{kernel.ParamValue: "a"}},
			},
		},
		{name: "not self parent: no parent", rules: notSelfParent, req: request{Branches: []string{"a"}}},
		{name: "not self parent: another parent", rules: notSelfParent, req: request{Branches: []string{"a"}, From: "main"}},
		{
			name: "not self parent: a parent among the children", rules: notSelfParent,
			req:  request{Branches: []string{"a", "b"}, From: "b"},
			want: []kernel.FieldError{{Path: "from", Code: kernel.CodeSelfParent, Params: kernel.Params{kernel.ParamValue: "b"}, With: []string{"branches"}}},
		},
		{
			name: "not self parent: a single child", rules: notSelfParentOfText,
			req:  request{Target: target{From: "b"}, From: "b"},
			want: []kernel.FieldError{{Path: "from", Code: kernel.CodeSelfParent, Params: kernel.Params{kernel.ParamValue: "b"}, With: []string{"target.from"}}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, checkRules(t, c.rules, c.req, c.facts))
		})
	}
}

func TestRulesAddUpInTheOrderTheyAreChained(t *testing.T) {
	chain := rules{}.Distinct("branches").Exclusive("push", "no_push")
	got := checkRules(t, chain, request{Branches: []string{"a", "a"}, Push: true, NoPush: true}, facts{})
	require.Len(t, got, 2)
	assert.Equal(t, kernel.CodeDistinct, got[0].Code)
	assert.Equal(t, kernel.CodeExclusiveWith, got[1].Code)
}

func TestWithAddsAHandWrittenRule(t *testing.T) {
	notMain := kernel.Rule[request, facts]{
		Code:  "test.not_main",
		Paths: []string{"from"},
		Check: func(req request, _ facts) ([]kernel.FieldError, error) {
			if req.From != "main" {
				return nil, nil
			}
			return []kernel.FieldError{{Path: "from", Code: "test.not_main"}}, nil
		},
	}
	assert.Equal(t, []kernel.FieldError{{Path: "from", Code: "test.not_main"}}, checkRules(t, rules{}.With(notMain), request{From: "main"}, facts{}))
}

func TestChainingFromTheSameRulesNeverMixesTheChains(t *testing.T) {
	base := rules{}.Distinct("branches")
	one := base.Exclusive("push", "no_push")
	two := base.OneOf("push", "no_push")
	assert.Len(t, base, 1)
	assert.Equal(t, kernel.CodeExclusiveWith, one[1].Code)
	assert.Equal(t, kernel.CodeRequiredOneOf, two[1].Code)
}

func TestARuleOnAPathThatNamesNoFieldIsAnError(t *testing.T) {
	broken := []rules{
		rules{}.Exclusive("push", "nope"),
		rules{}.OneOf("nope"),
		rules{}.Requires(kernel.RequiresRule{Field: "nope"}),
		rules{}.RequiredWhen("nope", branchExists),
		rules{}.Distinct("nope"),
		rules{}.NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "nope"}),
	}
	for _, chain := range broken {
		_, err := kernel.CheckRules(kernel.CheckRulesParams[request, facts]{Rules: chain, Request: request{From: "x"}})
		assert.Error(t, err, "%s", chain[0].Code)
	}
}

func TestAnOptionThatWouldBreakARuleArrivesDisabledWithItsReason(t *testing.T) {
	chain := rules{}.
		NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "branches"}).
		Exclusive("push", "no_push")
	req := request{Branches: []string{"feat/x"}, NoPush: true}
	cases := []struct {
		name      string
		path      string
		candidate kernel.Value
		refusal   kernel.Code
	}{
		{"a branch being created, as its own source", "from", kernel.Text("feat/x"), kernel.CodeSelfParent},
		{"another source", "from", kernel.Text("main"), ""},
		{"push beside no_push", "push", kernel.Bool(true), kernel.CodeExclusiveWith},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			refusal, err := kernel.Disabled(kernel.DisabledParams[request, facts]{Rules: chain, Request: req, Path: c.path, Candidate: c.candidate})
			require.NoError(t, err)
			if c.refusal == "" {
				assert.Nil(t, refusal)
				return
			}
			require.NotNil(t, refusal)
			assert.Equal(t, c.refusal, refusal.Code)
		})
	}
}

func TestAnOptionIsNotBlamedForAProblemAlreadyThere(t *testing.T) {
	refusal, err := kernel.Disabled(kernel.DisabledParams[request, facts]{
		Rules: rules{}.Exclusive("push", "no_push"), Request: request{Push: true, NoPush: true}, Path: "push", Candidate: kernel.Bool(true),
	})
	require.NoError(t, err)
	assert.Nil(t, refusal)
}

func TestOnlyTheRulesReadingThePathCanDisableAnOption(t *testing.T) {
	refusal, err := kernel.Disabled(kernel.DisabledParams[request, facts]{
		Rules: rules{}.Distinct("branches"), Request: request{Branches: []string{"a", "a"}}, Path: "from", Candidate: kernel.Text("main"),
	})
	require.NoError(t, err)
	assert.Nil(t, refusal)
}

func TestACandidateOfAnotherShapeThanTheFieldIsAnError(t *testing.T) {
	_, err := kernel.Disabled(kernel.DisabledParams[request, facts]{
		Rules: rules{}.Exclusive("push", "no_push"), Path: "push", Candidate: kernel.Text("yes"),
	})
	assert.Error(t, err)
}
