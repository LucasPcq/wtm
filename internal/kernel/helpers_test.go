package kernel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
)

// The few helpers the tests share. Everything a test file holds is a test.

// rules starts a chain of rules over the example request.
type rules = kernel.Rules[request, facts]

func checkRules(t *testing.T, chain rules, req request, f facts) []kernel.FieldError {
	t.Helper()
	problems, err := kernel.CheckRules(kernel.CheckRulesParams[request, facts]{Rules: chain, Request: req, Facts: f})
	require.NoError(t, err)
	return problems
}

// baseFallback defaults a field to the base branch of the facts, as config.
func baseFallback(_ request, f facts) (kernel.Fallback, bool) {
	return kernel.Fallback{Value: kernel.Text(f.Base), Origin: kernel.OriginConfig}, f.Base != ""
}

func checkSpec(t *testing.T, req request, specs ...kernel.FieldSpec) []kernel.FieldError {
	t.Helper()
	problems, err := kernel.CheckSpec(kernel.CheckSpecParams[request]{Request: req, Specs: specs})
	require.NoError(t, err)
	return problems
}

func evaluate(t *testing.T, params kernel.EvaluateParams[request, facts]) kernel.Form[request] {
	t.Helper()
	form, err := kernel.Evaluate(params)
	require.NoError(t, err)
	return form
}

// worktreeUnit is a unit shaped like create's: a saga that adds a worktree
// then its metadata, committed as the worktree's path, then the phases given.
func worktreeUnit(d *disk, branch string, then ...kernel.Phase[string]) kernel.Unit[string] {
	return kernel.Unit[string]{
		Saga: kernel.Saga[string]{
			Steps: []kernel.SagaStep{
				{Name: "worktree", Do: d.add(branch), Undo: d.remove(branch)},
				{Name: "meta", Do: d.add(branch + "/meta"), Undo: d.remove(branch + "/meta")},
			},
			Commit: func(context.Context) (string, error) { return "/wt/" + branch, nil },
		},
		Then: then,
	}
}

func phase(name string, run func(context.Context, string) (string, error)) kernel.Phase[string] {
	return kernel.Phase[string]{Name: name, Run: run}
}

// each runs Each over branches, each named by itself.
func each(ctx context.Context, params kernel.EachParams[string, string]) []kernel.Item[string] {
	params.Subject = func(branch string) string { return branch }
	return kernel.Each(ctx, params)
}

// outcomes reads items back as "subject:status/reason", one per item.
func outcomes(items []kernel.Item[string]) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Subject+":"+string(item.Status)+"/"+string(item.Reason))
	}
	return out
}

// states reads a form back as "path:status", one per field.
func states(form kernel.Form[request]) []string {
	out := make([]string, 0, len(form.States))
	for _, state := range form.States {
		out = append(out, state.Path+":"+string(state.Status))
	}
	return out
}
