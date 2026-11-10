package env_test

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/env"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"

	"github.com/stretchr/testify/require"
)

func (w world) run(t *testing.T, req env.Request, opts domain.RunOptions) env.Outcome {
	t.Helper()
	report := &operationtest.Recorder{}
	opts.Report = report.Add
	outcome, err := w.usecase.Run(t.Context(), req, opts)
	require.NoError(t, err)
	return outcome
}

func (w world) questions(t *testing.T, req env.Request) env.Questions {
	t.Helper()
	q, err := w.usecase.Questions(t.Context(), req)
	require.NoError(t, err)
	return q
}

func (w world) file(t *testing.T, worktree, target string) string {
	t.Helper()
	content, ok := w.store.File("/repo/.trees/" + worktree + "/" + target)
	require.True(t, ok, worktree+" "+target)
	return content
}

func naming(names ...string) env.Request {
	return env.Request{Worktrees: names}
}

func reconcileOf(t *testing.T, plan env.Plan, name string) env.Reconcile {
	t.Helper()
	at := slices.IndexFunc(plan.Worktrees, func(r env.Reconcile) bool { return r.Worktree == name })
	require.GreaterOrEqual(t, at, 0, name)
	return plan.Worktrees[at]
}

func optionOf(options []domain.Option, value string) domain.Option {
	return options[slices.IndexFunc(options, func(o domain.Option) bool { return o.Value == value })]
}

func codes(warnings []domain.Warning) []string {
	found := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		found = append(found, warning.Code)
	}
	return found
}
