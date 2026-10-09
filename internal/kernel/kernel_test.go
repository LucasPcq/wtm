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

// The whole contract on one command: the example `create` of fixture_test.go,
// driven the way dispatch and a form drive a command.

func TestACommandGoesFromAnEmptyFormToItsOutcome(t *testing.T) {
	ctx := context.Background()
	d := &disk{}
	create := createCommand(d)
	fill := func(req request, f facts) kernel.Form[request] {
		return evaluate(t, kernel.EvaluateParams[request, facts]{Fields: create.Fields, Rules: create.Rules, Request: req, Facts: f})
	}

	// Observe reads the repository once; nothing the user types reads it again.
	f, err := create.Observe(ctx, kernel.ObserveScope{Dir: "/repo"})
	require.NoError(t, err)

	// An empty request names what is missing: the 422 an agent gets.
	empty := fill(request{}, f)
	assert.False(t, empty.Complete)
	assert.Equal(t, []kernel.FieldError{{Path: "branches", Code: kernel.CodeRequired}}, kernel.Invalid(empty.Errors).Fields)

	// One of the branches exists: there is no safe source to default to, so
	// the rule asks for one, and says why.
	partial := fill(request{Branches: []string{"feat/old", "feat/new"}}, f)
	assert.Equal(t, []kernel.FieldError{{Path: "from", Code: kernel.CodeRequired, Params: kernel.Params{kernel.ParamBecause: "test.branch_exists"}}}, partial.Errors)

	// The form offers sources: a branch being created arrives disabled, with its reason.
	refusal, err := kernel.Disabled(kernel.DisabledParams[request, facts]{
		Rules: create.Rules, Request: partial.Request, Facts: f, Path: "from", Candidate: kernel.Text("feat/new"),
	})
	require.NoError(t, err)
	require.NotNil(t, refusal)
	assert.Equal(t, kernel.CodeSelfParent, refusal.Code)

	// The user picks main: the form is complete, isolation taken from its default.
	complete := fill(request{Branches: []string{"feat/old", "feat/new"}, From: "main"}, f)
	require.True(t, complete.Complete, "errors: %v", complete.Errors)
	assert.Equal(t, isolation("isolated"), complete.Request.Isolation)
	assert.Equal(t, kernel.OriginDefault, complete.States[2].Origin)

	// The command names its locks: the repository, and each branch.
	assert.Len(t, create.Locks(complete.Request), 3)

	// It plans, then applies, saying what it does as it goes.
	plan, err := create.Plan(ctx, complete.Request, f)
	require.NoError(t, err)
	progress := &recorder{}
	outcome, err := create.Apply(ctx, kernel.ApplyInput[request, facts, createPlan]{Request: complete.Request, Facts: f, Plan: plan, Emit: progress})
	require.NoError(t, err)
	assert.Equal(t, []string{"feat/old:done/", "feat/new:done/"}, outcomes(outcome.Items))
	assert.Equal(t, "/wt/feat/new configured", outcome.Items[1].Detail)
	lines := progress.lines()
	assert.Equal(t, "feat/old unit.started", lines[0])
	assert.Equal(t, "feat/new unit.finished done", lines[len(lines)-1])
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
