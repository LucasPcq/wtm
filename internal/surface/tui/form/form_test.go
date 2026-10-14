package form

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestAConfirmedRecapAppliesThePlanItShowed(t *testing.T) {
	w := newWorld(t, &repo{}, worldParams{})

	reply, err := w.script(t, func(d *driver) {
		fillAll(d)
		assert.Contains(t, d.view(), `{"branches":["feat/a"],"from":"main"}`)
		d.key("enter")
	})

	require.NoError(t, err)
	applied, ok := reply.(dispatch.CommandReply)
	require.True(t, ok)
	require.NotNil(t, applied.Outcome)
	assert.Equal(t, kernel.StatusDone, applied.Outcome.Items[0].Status)
	assert.Equal(t, []string{"feat/a"}, w.Repo.worktrees)
	assert.NotEmpty(t, w.Progress.Seen(), "the run draws its progress on the view the command line gave")
}

func TestADryRunAsksWhatIsMissingThenReturnsThePlanWithoutConfirming(t *testing.T) {
	w := newWorld(t, &repo{}, worldParams{Options: dispatch.Options{DryRun: true}})

	reply, err := w.script(t, fillAll)

	require.NoError(t, err)
	previewed, ok := reply.(dispatch.CommandReply)
	require.True(t, ok)
	assert.Nil(t, previewed.Outcome)
	assert.JSONEq(t, `{"branches":["feat/a"],"from":"main"}`, string(previewed.Plan.Detail))
	assert.Empty(t, w.Repo.worktrees)
}

func TestADryRunTheFlagsAnswerWholeOpensNoWizard(t *testing.T) {
	w := newWorld(t, &repo{}, worldParams{
		Request: `{"branches":["feat/a"],"from":"main","isolation":"isolated","push":true,"message":"m","hooks":["install"],"order":["api"],"env":{".env":"copy"}}`,
		Options: dispatch.Options{DryRun: true},
	})

	reply, err := w.script(t)

	require.NoError(t, err)
	assert.IsType(t, dispatch.CommandReply{}, reply)
}

func TestBackingOutIsACancelledRun(t *testing.T) {
	cases := map[string]func(*driver){
		"ctrl+c":         func(d *driver) { d.key("ctrl+c") },
		"esc first step": func(d *driver) { d.key("esc") },
		"no, cancel": func(d *driver) {
			fillAll(d)
			d.key("down", "enter")
		},
	}
	for name, backOut := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t, &repo{}, worldParams{})

			reply, err := w.script(t, backOut)

			assert.Nil(t, reply)
			var failure kernel.Error
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, kernel.KindCancelled, failure.Kind())
			assert.Empty(t, w.Repo.worktrees)
		})
	}
}

func TestFactsThatChangedBeforeTheConfirmationBringTheRecapBack(t *testing.T) {
	r := &repo{}
	w := newWorld(t, r, worldParams{})

	reply, err := w.script(t,
		func(d *driver) {
			fillAll(d)
			r.changed("feat/elsewhere")
			d.key("enter")
		},
		func(d *driver) {
			assert.Contains(t, d.view(), "the facts changed since the preview")
			assert.Contains(t, d.trail(), "Branches: feat/a")
			d.key("enter")
		},
	)

	require.NoError(t, err)
	assert.IsType(t, dispatch.CommandReply{}, reply)
	assert.Equal(t, []string{"feat/a"}, r.worktrees)
}
