package kernel_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/kerneltest"
)

func TestACommandGoesFromAnEmptyFormToItsOutcome(t *testing.T) {
	ctx := context.Background()
	d := &disk{}
	create := createCommand(d)
	fill := func(t *testing.T, req request, f facts) kernel.Form[request] {
		return evaluate(t, kernel.EvaluateParams[request, facts]{Fields: create.Fields, Rules: create.Rules, Request: req, Facts: f})
	}
	var f facts
	var partial, complete kernel.Form[request]
	var plan kernel.Plan[createPlan]

	t.Run("Observe reads the repository once, nothing typed reads it again", func(t *testing.T) {
		var err error
		f, err = create.Observe(ctx)
		require.NoError(t, err)
	})

	t.Run("an empty request names what is missing, the 422 an agent gets", func(t *testing.T) {
		empty := fill(t, request{}, f)
		assert.False(t, empty.Complete)
		assert.Equal(t, []kernel.FieldError{{Path: "branches", Code: kernel.CodeRequired}}, kernel.Invalid(empty.Errors).Fields)
	})

	t.Run("a branch that exists leaves no safe source, so the rule asks for one and says why", func(t *testing.T) {
		partial = fill(t, request{Branches: []string{"feat/old", "feat/new"}}, f)
		assert.Equal(t, []kernel.FieldError{{Path: "from", Code: kernel.CodeRequired, Params: kernel.Params{kernel.ParamBecause: "test.branch_exists"}}}, partial.Errors)
	})

	t.Run("a source being created is offered disabled, with its reason", func(t *testing.T) {
		refusal, err := kernel.Disabled(kernel.DisabledParams[request, facts]{
			Rules: create.Rules, Request: partial.Request, Facts: f, Path: "from", Candidate: kernel.Text("feat/new"),
		})
		require.NoError(t, err)
		require.NotNil(t, refusal)
		assert.Equal(t, kernel.CodeSelfParent, refusal.Code)
	})

	t.Run("picking main completes the form, isolation from its default", func(t *testing.T) {
		complete = fill(t, request{Branches: []string{"feat/old", "feat/new"}, From: "main"}, f)
		require.True(t, complete.Complete, "errors: %v", complete.Errors)
		assert.Equal(t, isolation("isolated"), complete.Request.Isolation)
		assert.Equal(t, kernel.OriginDefault, complete.States[2].Origin)
	})

	t.Run("the command locks the repository and each branch", func(t *testing.T) {
		assert.Len(t, create.Locks(complete.Request), 3)
	})

	t.Run("it plans", func(t *testing.T) {
		var err error
		plan, err = create.Plan(ctx, complete.Request, f)
		require.NoError(t, err)
	})

	t.Run("it applies, saying what it does as it goes", func(t *testing.T) {
		progress := &recorder{}
		outcome, err := create.Apply(ctx, kernel.ApplyInput[request, facts, createPlan, string]{Request: complete.Request, Facts: f, Plan: plan, Emit: progress})
		require.NoError(t, err)
		assert.Equal(t, []string{"feat/old:done/", "feat/new:done/"}, outcomes(outcome.Items))
		assert.Equal(t, "/wt/feat/new configured", outcome.Items[1].Detail)
		lines := progress.lines()
		assert.Equal(t, "feat/old unit.started", lines[0])
		assert.Equal(t, "feat/new unit.finished done", lines[len(lines)-1])
	})
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
