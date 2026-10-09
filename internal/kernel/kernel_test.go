package kernel_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/kerneltest"
)

// The whole contract on one command: an example `create`, declared the way a
// command on the engine is, driven the way dispatch and a form drive it.

type createPlan struct {
	Branches []string
	From     string
}

func createFields() []kernel.FieldDef[request, facts] {
	return []kernel.FieldDef[request, facts]{
		{
			Spec: kernel.FieldSpec{
				Path: "branches", Type: kernel.FieldTextList, Label: "Branches", Required: true,
				Constraints: kernel.Constraints{Pattern: `^[a-z0-9/-]+$`},
			},
		},
		{
			Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect, Label: "Source", Choices: kernel.ChoicesSearch, DependsOn: []string{"branches"}},
			Default: func(req request, f facts) (kernel.Fallback, bool) {
				fresh := !branchExists.Holds(req, f)
				return kernel.Fallback{Value: kernel.Value{Text: f.Base}, Origin: kernel.OriginConfig}, fresh
			},
		},
		{
			Spec: kernel.FieldSpec{
				Path: "isolation", Type: kernel.FieldSelect, Label: "Isolation", Rememberable: true,
				Constraints: kernel.Constraints{Enum: []string{"isolated", "verbatim"}},
			},
			Default: func(request, facts) (kernel.Fallback, bool) {
				return kernel.Fallback{Value: kernel.Value{Text: "isolated"}, Origin: kernel.OriginDefault}, true
			},
		},
	}
}

func createRules() kernel.Rules[request, facts] {
	return on{}.
		Distinct("branches").
		RequiredWhen("from", branchExists).
		NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "branches"})
}

func createUnit(d *disk, branch string) kernel.Unit[string] {
	configure := phase("configure", func(_ context.Context, path string) (string, error) { return path + " configured", nil })
	return worktreeUnit(d, branch, configure)
}

func createCommand(d *disk) kernel.Command[request, facts, createPlan, string] {
	return kernel.Command[request, facts, createPlan, string]{
		Name: "create",
		Observe: func(context.Context, kernel.ObserveScope) (facts, error) {
			return facts{Existing: []string{"feat/old"}, Base: "main"}, nil
		},
		Fields: createFields(),
		Rules:  createRules(),
		Locks: func(req request) []kernel.LockKey {
			locks := []kernel.LockKey{{Scope: kernel.LockRepo, Mode: kernel.LockShared}}
			for _, branch := range req.Branches {
				locks = append(locks, kernel.LockKey{Scope: kernel.LockWorktree, Name: branch, Mode: kernel.LockExclusive})
			}
			return locks
		},
		Plan: func(_ context.Context, req request, _ facts) (kernel.Plan[createPlan], error) {
			return kernel.Plan[createPlan]{Detail: createPlan{Branches: req.Branches, From: req.From}}, nil
		},
		Apply: func(ctx context.Context, in kernel.ApplyInput[request, facts, createPlan]) (kernel.Outcome[string], error) {
			items := kernel.Each(ctx, kernel.EachParams[string, string]{
				Items:   in.Plan.Detail.Branches,
				Subject: func(branch string) string { return branch },
				Unit:    func(branch string) kernel.Unit[string] { return createUnit(d, branch) },
				Emit:    in.Emit,
				Shield:  in.Shield,
			})
			return kernel.Outcome[string]{Items: items}, nil
		},
	}
}

func TestACommandGoesFromAnEmptyFormToItsOutcome(t *testing.T) {
	ctx := context.Background()
	d := &disk{}
	create := createCommand(d)
	fill := func(req request, f facts) kernel.Form[request] {
		t.Helper()
		form, err := kernel.Evaluate(kernel.EvaluateParams[request, facts]{Fields: create.Fields, Rules: create.Rules, Request: req, Facts: f})
		if err != nil {
			t.Fatal(err)
		}
		return form
	}

	// Observe reads the repository once; nothing the user types reads it again.
	f, err := create.Observe(ctx, kernel.ObserveScope{Dir: "/repo"})
	if err != nil {
		t.Fatal(err)
	}

	// An empty request names what is missing: the 422 an agent gets.
	empty := fill(request{}, f)
	if invalid := kernel.Invalid(empty.Errors); empty.Complete || !reflect.DeepEqual(invalid.Fields, []kernel.FieldError{{Path: "branches", Code: kernel.CodeRequired}}) {
		t.Fatalf("empty form errors = %+v", empty.Errors)
	}

	// One of the branches exists: there is no safe source to default to, so
	// the rule asks for one, and says why.
	partial := fill(request{Branches: []string{"feat/old", "feat/new"}}, f)
	if partial.Complete || partial.Errors[0].Code != kernel.CodeRequired || partial.Errors[0].Params[kernel.ParamBecause] != "test.branch_exists" {
		t.Fatalf("partial form errors = %+v", partial.Errors)
	}

	// The form offers sources: one of the branches being created arrives disabled, with its reason.
	refusal, err := kernel.Disabled(kernel.DisabledParams[request, facts]{
		Rules: create.Rules, Request: partial.Request, Facts: f, Path: "from", Candidate: kernel.Value{Text: "feat/new"},
	})
	if err != nil || refusal == nil || refusal.Code != kernel.CodeSelfParent {
		t.Fatalf("feat/new as a source: %+v, %v", refusal, err)
	}

	// The user picks main: the form is complete, isolation taken from its default.
	complete := fill(request{Branches: []string{"feat/old", "feat/new"}, From: "main"}, f)
	if !complete.Complete || complete.Request.Isolation != "isolated" || complete.States[2].Origin != kernel.OriginDefault {
		t.Fatalf("complete form = %+v", complete)
	}

	// The command names its locks, plans, then applies.
	if got := len(create.Locks(complete.Request)); got != 3 {
		t.Errorf("locks = %d, want the repo and one per branch", got)
	}
	plan, err := create.Plan(ctx, complete.Request, f)
	if err != nil {
		t.Fatal(err)
	}
	progress := &recorder{}
	outcome, err := create.Apply(ctx, kernel.ApplyInput[request, facts, createPlan]{Request: complete.Request, Facts: f, Plan: plan, Emit: progress})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := statuses(outcome.Items), []string{"feat/old:done/", "feat/new:done/"}; !slices.Equal(got, want) {
		t.Errorf("outcome = %v, want %v", got, want)
	}
	if outcome.Items[1].Detail != "/wt/feat/new configured" {
		t.Errorf("detail = %q", outcome.Items[1].Detail)
	}
	if got := progress.lines(); got[0] != "feat/old unit.started" || got[len(got)-1] != "feat/new unit.finished done" {
		t.Errorf("progress = %v", got)
	}
}

func TestTheExampleCommandPassesTheChecksEveryCommandRuns(t *testing.T) {
	kerneltest.CheckDependsOn(t, kerneltest.DependsOnParams[request, facts]{
		Fields:  createFields(),
		Request: request{Branches: []string{"feat/old"}, From: "main", Isolation: "verbatim"},
		Facts:   facts{Existing: []string{"feat/old"}, Base: "main"},
	})
	d := &disk{entries: []string{"main"}}
	kerneltest.CheckSaga(t, kerneltest.SagaParams[string]{
		Saga:     func() kernel.Saga[string] { return createUnit(d, "feat/x").Saga },
		Snapshot: func() string { return strings.Join(d.list(), ",") },
	})
}
