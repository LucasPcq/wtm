package kernel_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/kernel"
)

type on = kernel.Rules[request, facts]

var branchExists = kernel.Condition[request, facts]{
	Code: "test.branch_exists",
	Holds: func(req request, f facts) bool {
		return slices.ContainsFunc(req.Branches, func(branch string) bool { return slices.Contains(f.Existing, branch) })
	},
}

func checkRules(t *testing.T, rules on, req request, f facts) []kernel.FieldError {
	t.Helper()
	problems, err := kernel.CheckRules(kernel.CheckRulesParams[request, facts]{Rules: rules, Request: req, Facts: f})
	if err != nil {
		t.Fatal(err)
	}
	return problems
}

type ruleCase struct {
	name  string
	req   request
	facts facts
	want  []kernel.FieldError
}

func runRuleCases(t *testing.T, rules on, cases []ruleCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := checkRules(t, rules, c.req, c.facts); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got  %+v\nwant %+v", got, c.want)
			}
		})
	}
}

func TestExclusiveAllowsAtMostOne(t *testing.T) {
	runRuleCases(t, on{}.Exclusive("push", "no_push", "url_host"), []ruleCase{
		{name: "none set"},
		{name: "one set", req: request{NoPush: true}},
		{
			name: "two set: the second is blamed",
			req:  request{Push: true, NoPush: true},
			want: []kernel.FieldError{{Path: "no_push", Code: kernel.CodeExclusiveWith, With: []string{"push"}}},
		},
		{
			name: "three set: each after the first is blamed, naming the others",
			req:  request{Push: true, NoPush: true, URLHost: "api"},
			want: []kernel.FieldError{
				{Path: "no_push", Code: kernel.CodeExclusiveWith, With: []string{"push", "url_host"}},
				{Path: "url_host", Code: kernel.CodeExclusiveWith, With: []string{"push", "no_push"}},
			},
		},
	})
}

func TestOneOfWantsExactlyOne(t *testing.T) {
	runRuleCases(t, on{}.OneOf("push", "no_push"), []ruleCase{
		{
			name: "none set: the first is required, with the others as alternatives",
			want: []kernel.FieldError{{Path: "push", Code: kernel.CodeRequired, With: []string{"no_push"}}},
		},
		{name: "one set", req: request{NoPush: true}},
		{
			name: "both set",
			req:  request{Push: true, NoPush: true},
			want: []kernel.FieldError{{Path: "no_push", Code: kernel.CodeExclusiveWith, With: []string{"push"}}},
		},
	})
}

func TestRequiresWantsItsNeedsWhenTheFieldIsSet(t *testing.T) {
	runRuleCases(t, on{}.Requires(kernel.RequiresRule{Field: "url_host", Needs: []string{"url_port", "from"}}), []ruleCase{
		{name: "the field unset needs nothing", req: request{URLPort: "8080"}},
		{name: "every need set", req: request{URLHost: "api", URLPort: "8080", From: "main"}},
		{
			name: "the needs missing are named",
			req:  request{URLHost: "api", URLPort: "8080"},
			want: []kernel.FieldError{{Path: "url_host", Code: kernel.CodeRequires, With: []string{"from"}}},
		},
	})
}

func TestRequiredWhenSaysWhyTheFieldIsRequired(t *testing.T) {
	runRuleCases(t, on{}.RequiredWhen("from", branchExists), []ruleCase{
		{
			name:  "the condition holds and the field is empty",
			req:   request{Branches: []string{"feat/x"}},
			facts: facts{Existing: []string{"feat/x"}},
			want:  []kernel.FieldError{{Path: "from", Code: kernel.CodeRequired, Params: map[string]string{kernel.ParamBecause: "test.branch_exists"}}},
		},
		{
			name:  "the condition holds and the field is set",
			req:   request{Branches: []string{"feat/x"}, From: "main"},
			facts: facts{Existing: []string{"feat/x"}},
		},
		{
			name:  "the condition does not hold",
			req:   request{Branches: []string{"feat/new"}},
			facts: facts{Existing: []string{"feat/x"}},
		},
	})
}

func TestDistinctBlamesEachRepeatAtItsIndex(t *testing.T) {
	runRuleCases(t, on{}.Distinct("branches"), []ruleCase{
		{name: "all different", req: request{Branches: []string{"a", "b"}}},
		{
			name: "repeats",
			req:  request{Branches: []string{"a", "b", "a", "b", "a"}},
			want: []kernel.FieldError{
				{Path: "branches[2]", Code: kernel.CodeDistinct, Params: map[string]string{kernel.ParamValue: "a"}},
				{Path: "branches[3]", Code: kernel.CodeDistinct, Params: map[string]string{kernel.ParamValue: "b"}},
				{Path: "branches[4]", Code: kernel.CodeDistinct, Params: map[string]string{kernel.ParamValue: "a"}},
			},
		},
	})
}

func TestNotSelfParentRefusesABranchAsItsOwnParent(t *testing.T) {
	selfParent := func(value, children string) []kernel.FieldError {
		return []kernel.FieldError{{Path: "from", Code: kernel.CodeSelfParent, Params: map[string]string{kernel.ParamValue: value}, With: []string{children}}}
	}
	runRuleCases(t, on{}.NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "branches"}), []ruleCase{
		{name: "no parent", req: request{Branches: []string{"a"}}},
		{name: "another parent", req: request{Branches: []string{"a"}, From: "main"}},
		{name: "a parent among the children", req: request{Branches: []string{"a", "b"}, From: "b"}, want: selfParent("b", "branches")},
	})
	runRuleCases(t, on{}.NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "target.from"}), []ruleCase{
		{name: "a single child", req: request{Target: target{From: "b"}, From: "b"}, want: selfParent("b", "target.from")},
	})
}

func TestRulesAddUpInTheOrderTheyAreChained(t *testing.T) {
	rules := on{}.Distinct("branches").Exclusive("push", "no_push")
	got := checkRules(t, rules, request{Branches: []string{"a", "a"}, Push: true, NoPush: true}, facts{})
	if len(got) != 2 || got[0].Code != kernel.CodeDistinct || got[1].Code != kernel.CodeExclusiveWith {
		t.Errorf("got %+v", got)
	}
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
	if got := checkRules(t, on{}.With(notMain), request{From: "main"}, facts{}); len(got) != 1 || got[0].Code != "test.not_main" {
		t.Errorf("got %+v", got)
	}
}

func TestChainingFromTheSameRulesNeverMixesTheChains(t *testing.T) {
	base := on{}.Distinct("branches")
	one := base.Exclusive("push", "no_push")
	two := base.OneOf("push", "no_push")
	if len(base) != 1 || one[1].Code != kernel.CodeExclusiveWith || two[1].Code != kernel.CodeRequired {
		t.Errorf("base %d rules, one[1] %s, two[1] %s", len(base), one[1].Code, two[1].Code)
	}
}

func TestARuleOnAPathThatNamesNoFieldIsAnError(t *testing.T) {
	broken := []on{
		on{}.Exclusive("push", "nope"),
		on{}.OneOf("nope"),
		on{}.Requires(kernel.RequiresRule{Field: "nope"}),
		on{}.RequiredWhen("nope", branchExists),
		on{}.Distinct("nope"),
		on{}.NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "nope"}),
	}
	for _, rules := range broken {
		_, err := kernel.CheckRules(kernel.CheckRulesParams[request, facts]{Rules: rules, Request: request{From: "x"}})
		if err == nil {
			t.Errorf("%s passed", rules[0].Code)
		}
	}
}

func TestAnOptionThatWouldBreakARuleArrivesDisabledWithItsReason(t *testing.T) {
	rules := on{}.
		NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "branches"}).
		Exclusive("push", "no_push")
	req := request{Branches: []string{"feat/x"}, NoPush: true}
	disabled := func(path string, candidate kernel.Value) *kernel.FieldError {
		t.Helper()
		refusal, err := kernel.Disabled(kernel.DisabledParams[request, facts]{Rules: rules, Request: req, Path: path, Candidate: candidate})
		if err != nil {
			t.Fatal(err)
		}
		return refusal
	}
	if refusal := disabled("from", kernel.Value{Text: "feat/x"}); refusal == nil || refusal.Code != kernel.CodeSelfParent {
		t.Errorf("feat/x as its own parent: %+v", refusal)
	}
	if refusal := disabled("from", kernel.Value{Text: "main"}); refusal != nil {
		t.Errorf("main is a fine parent: %+v", refusal)
	}
	if refusal := disabled("push", kernel.Value{Bool: true}); refusal == nil || refusal.Code != kernel.CodeExclusiveWith {
		t.Errorf("push beside no_push: %+v", refusal)
	}
}

func TestAnOptionIsNotBlamedForAProblemAlreadyThere(t *testing.T) {
	req := request{Push: true, NoPush: true}
	refusal, err := kernel.Disabled(kernel.DisabledParams[request, facts]{
		Rules: on{}.Exclusive("push", "no_push"), Request: req, Path: "push", Candidate: kernel.Value{Bool: true},
	})
	if err != nil || refusal != nil {
		t.Errorf("refusal = %+v, err = %v", refusal, err)
	}
}

func TestOnlyTheRulesReadingThePathCanDisableAnOption(t *testing.T) {
	req := request{Branches: []string{"a", "a"}}
	refusal, err := kernel.Disabled(kernel.DisabledParams[request, facts]{
		Rules: on{}.Distinct("branches"), Request: req, Path: "from", Candidate: kernel.Value{Text: "main"},
	})
	if err != nil || refusal != nil {
		t.Errorf("refusal = %+v, err = %v", refusal, err)
	}
}
