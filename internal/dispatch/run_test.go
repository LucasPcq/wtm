package dispatch_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
	"github.com/LucasPcq/wtm/internal/testutil/dispatchtest"
)

func TestARunAppliesThePlanAndReturnsItsOutcome(t *testing.T) {
	r := &repo{}
	ran := run(t.Context(), runParams{Bench: bench(t), Repo: r, Request: request{Branches: []string{"feat/a", "feat/b"}}})

	require.NoError(t, ran.Err)
	assert.Equal(t, kernel.OpID("op-1"), ran.Result.Op)
	assert.NotEmpty(t, ran.Result.Fingerprint)
	assert.Equal(t, plan{Branches: []string{"feat/a", "feat/b"}, Isolation: isolated}, ran.Result.Plan.Detail)
	require.NotNil(t, ran.Result.Outcome)
	assert.Equal(t, []kernel.Item[string]{
		{Subject: "feat/a", Status: kernel.StatusDone, Detail: "/wt/feat/a"},
		{Subject: "feat/b", Status: kernel.StatusDone, Detail: "/wt/feat/b"},
	}, ran.Result.Outcome.Items)
	assert.Empty(t, ran.Result.Outcome.Warnings)
	assert.Equal(t, []string{"feat/a", "feat/b"}, r.listed())
}

func TestAnIncompleteRequestIsA422NamingItsFieldsAndAppliesNothing(t *testing.T) {
	r := &repo{}
	ran := run(t.Context(), runParams{Bench: bench(t), Repo: r, Request: request{Isolation: "copied"}})

	var invalid *kernel.InvalidError
	require.ErrorAs(t, ran.Err, &invalid)
	assert.Equal(t, []kernel.FieldError{
		{Path: "branches", Code: kernel.CodeRequired},
		{Path: "isolation", Code: kernel.CodeOneOf, Params: kernel.Params{kernel.ParamValue: "copied"}, Accepted: []string{isolated, verbatim}},
	}, invalid.Fields)
	assert.Empty(t, r.listed())
}

func TestADryRunWritesNothingAndReturnsThePlanWithItsFingerprint(t *testing.T) {
	b, r := bench(t), &repo{}
	snapshot, err := handle(b, r).Observe(t.Context())
	require.NoError(t, err)

	ran := run(t.Context(), runParams{Bench: b, Repo: r, Request: request{Branches: []string{"feat/a"}}, Options: dispatch.Options{DryRun: true}})

	require.NoError(t, ran.Err)
	assert.Nil(t, ran.Result.Outcome)
	assert.Equal(t, plan{Branches: []string{"feat/a"}, Isolation: isolated}, ran.Result.Plan.Detail)
	assert.Equal(t, snapshot.Fingerprint, ran.Result.Fingerprint)
	assert.Empty(t, r.listed())
	assert.Empty(t, b.Bus.Events(), "a dry run published an operation")
	assert.Empty(t, ran.Progress)
}

func TestARunHandedItsPreviewFingerprintApplies(t *testing.T) {
	b, r := bench(t), &repo{}
	preview := run(t.Context(), runParams{Bench: b, Repo: r, Request: request{Branches: []string{"feat/a"}}, Options: dispatch.Options{DryRun: true}})

	ran := run(t.Context(), runParams{Bench: b, Repo: r, Request: request{Branches: []string{"feat/a"}}, Options: dispatch.Options{ExpectFingerprint: preview.Result.Fingerprint}})

	require.NoError(t, ran.Err)
	assert.Equal(t, []string{"feat/a"}, r.listed())
}

func TestFactsChangedSinceThePreviewAreA412PlanStale(t *testing.T) {
	b, r := bench(t), &repo{}
	preview := run(t.Context(), runParams{Bench: b, Repo: r, Request: request{Branches: []string{"feat/a"}}, Options: dispatch.Options{DryRun: true}})
	r.existing = []string{"feat/old"}

	ran := run(t.Context(), runParams{Bench: b, Repo: r, Request: request{Branches: []string{"feat/a"}}, Options: dispatch.Options{ExpectFingerprint: preview.Result.Fingerprint}})

	failure := problem(t, ran.Err)
	assert.Equal(t, kernel.KindPrecondition, failure.Kind())
	assert.Equal(t, dispatch.CodePlanStale, failure.Base().Code)
	assert.Equal(t, string(preview.Result.Fingerprint), failure.Base().Params[dispatch.ParamExpected])
	assert.Equal(t, string(ran.Result.Fingerprint), failure.Base().Params[dispatch.ParamFound])
	assert.Empty(t, r.listed())
}

func TestAStateFileFromANewerWtmIsA412CarryingItsSchemas(t *testing.T) {
	newer := &statefile.NewerError{Schema: statefile.Schema{Path: "/repo/.git/wtm/worktrees/feat%2Fa/meta.json", Found: 3, Supported: 2}}
	r := &repo{applyErr: errors.Join(errors.New("writing meta.json"), newer)}

	ran := run(t.Context(), runParams{Bench: bench(t), Repo: r, Request: request{Branches: []string{"feat/a"}}})

	failure := problem(t, ran.Err)
	assert.Equal(t, kernel.KindPrecondition, failure.Kind())
	assert.Equal(t, dispatch.CodeStateSchemaNewer, failure.Base().Code)
	assert.Equal(t, kernel.Params{kernel.ParamPath: newer.Path, dispatch.ParamFound: "3", dispatch.ParamSupported: "2"}, failure.Base().Params)
	assert.ErrorIs(t, ran.Err, newer)
}

func TestARememberedAnswerIsUsedUnlessTheRunAsksAgain(t *testing.T) {
	cases := map[string]struct {
		options dispatch.Options
		want    string
	}{
		"remembered": {want: verbatim},
		"--ask":      {options: dispatch.Options{IgnoreRemembered: true}, want: isolated},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := &repo{remembered: verbatim}
			ran := run(t.Context(), runParams{Bench: bench(t), Repo: r, Request: request{Branches: []string{"feat/a"}}, Options: c.options})

			require.NoError(t, ran.Err)
			assert.Equal(t, c.want, ran.Result.Plan.Detail.Isolation)
		})
	}
}

func TestWithoutLocksTheRunGoesOnUnprotectedAndSaysSo(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(notADir, nil, 0o644))
	r := &repo{}

	ran := run(t.Context(), runParams{
		Bench:   dispatchtest.New(t, dispatchtest.BenchParams{StateDir: notADir}),
		Repo:    r,
		Request: request{Branches: []string{"feat/a"}},
	})

	require.NoError(t, ran.Err)
	require.NotNil(t, ran.Result.Outcome)
	assert.Equal(t, []kernel.Warning{{Code: dispatch.CodeLocksUnavailable}}, ran.Result.Outcome.Warnings)
	assert.Equal(t, []string{"feat/a"}, r.listed())
}
