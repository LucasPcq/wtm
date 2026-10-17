package form

import (
	"context"
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachAskedStepIsEnteredThenTheRecapApplies(t *testing.T) {
	f := newFake()
	d := open(t, Params{Script: f.script()})

	assert.Contains(t, d.screen(), "Step 1/3 • First")
	assert.Contains(t, d.screen(), "What First asks")
	assert.Contains(t, d.screen(), "↑↓ navigate • / filter • enter confirm • esc cancel")

	d.keys("enter")
	assert.Contains(t, d.screen(), "Step 2/3 • Second")
	assert.Contains(t, d.screen(), "✓ First: yes")
	assert.Contains(t, d.screen(), "esc back")

	d.keys("down", "enter")
	screen := d.screen()
	assert.Contains(t, screen, "Step 3/3 • Apply this plan?")
	assert.Contains(t, screen, "✓ Second: no")
	assert.Contains(t, screen, "plan: yes, no")

	d.keys("enter")
	assert.Equal(t, Finished, d.w.Ending())
	assert.Empty(t, d.w.View())
	assert.Equal(t, 3, f.observed)
}

func TestASettledStepIsNotAskedAndSaysWhatSettledIt(t *testing.T) {
	cases := map[string]struct {
		Standing Standing
		Line     string
	}{
		"a flag":             {Standing: Standing{Value: "x", Flag: "--first"}, Line: "✓ First: x · --first"},
		"a remembered value": {Standing: Standing{Value: "x", Origin: domain.OriginRemembered}, Line: "✓ First: x · remembered"},
		"a skip":             {Standing: Standing{Skip: "not needed"}, Line: "· First — not needed"},
	}
	for name, c := range cases {
		f := newFake()
		f.first.standing = c.Standing
		d := open(t, Params{Script: f.script()})

		assert.Equal(t, 1, d.w.Current(), name)
		assert.Contains(t, d.screen(), c.Line, name)
		assert.Contains(t, d.screen(), "esc cancel", name)
	}
}

func TestAnErredStepIsAskedEvenWhenAFlagSettledIt(t *testing.T) {
	f := newFake()
	f.first.standing = Standing{Value: "x", Flag: "--first", Erred: true}
	d := open(t, Params{Script: f.script()})

	assert.Equal(t, 0, d.w.Current())
}

func TestAKeptDefaultSaysWhereItCameFrom(t *testing.T) {
	f := newFake()
	f.first.standing = Standing{Value: "yes", Origin: domain.OriginConfig}
	d := open(t, Params{Script: f.script()})

	d.keys("enter")

	assert.Contains(t, d.screen(), "✓ First: yes · config")
}

func TestBackReturnsToTheLastAskedStepAndResetsTheOnesAfter(t *testing.T) {
	f := newFake()
	d := open(t, Params{Script: f.script()})
	d.keys("enter", "enter")
	require.Equal(t, 2, d.w.Current())

	d.keys("esc")

	assert.Equal(t, 1, d.w.Current())
	assert.Zero(t, f.second.resets)
	d.keys("esc")
	assert.Equal(t, 0, d.w.Current())
	assert.Equal(t, 1, f.second.resets)
	assert.Zero(t, f.first.resets)
}

func TestEachWayOutCancels(t *testing.T) {
	cases := map[string][]string{
		"esc on the first asked step": {"esc"},
		"ctrl+c":                      {"ctrl+c"},
		"no, cancel on the recap":     {"enter", "enter", "down", "enter"},
	}
	for name, keys := range cases {
		f := newFake()
		d := open(t, Params{Script: f.script()})

		d.keys(keys...)

		assert.Equal(t, Cancelled, d.w.Ending(), name)
	}
}

func TestAPreviewThatFailsSaysWhyAndOffersOnlyToCancel(t *testing.T) {
	f := newFake()
	f.previewErr = domain.Invalid([]domain.FieldError{{Path: "first", Code: "required", Template: "{path} is required"}})
	d := open(t, Params{Script: f.script()})

	d.keys("enter", "enter")

	screen := d.screen()
	assert.Contains(t, screen, "Step 3/3 • Cannot apply this plan")
	assert.Contains(t, screen, "✗ the request is invalid\n  ┃      FIRST is required")
	assert.NotContains(t, screen, "Yes, apply")
	d.keys("enter")
	assert.Equal(t, Cancelled, d.w.Ending())
}

func TestAStaleRunAsksNothingAgainAndSaysWhy(t *testing.T) {
	f := newFake()
	d := open(t, Params{Script: f.script()})
	d.keys("enter", "enter")

	again := start(d.w.Again())

	assert.Equal(t, 2, again.w.Current())
	screen := again.screen()
	assert.Contains(t, screen, "✓ First: yes\n")
	assert.Contains(t, screen, "! the facts changed since the preview")
}

func TestADryRunAsksTheStepsAndNoConfirmation(t *testing.T) {
	f := newFake()
	d := open(t, Params{Script: f.script(), DryRun: true})

	assert.Contains(t, d.screen(), "Step 1/2 • First")
	d.keys("enter", "enter")

	assert.Equal(t, Finished, d.w.Ending())
}

func TestTheRefreshKeyReobservesOnlyAStepThatRefreshes(t *testing.T) {
	f := newFake()
	d := open(t, Params{Script: f.script()})

	d.keys("r")

	assert.Equal(t, 1, f.observed)
	assert.NotContains(t, d.screen(), "r refresh")
}

func TestADryRunWithNothingToAskAppliesWithoutDrawing(t *testing.T) {
	f := newFake()
	f.first.standing = Standing{Skip: "no"}
	f.second.standing = Standing{Skip: "no"}
	ios, _, _, out := output.Test()

	err := Run(t.Context(), Params{Script: f.script(), DryRun: true}, ios.In, ios.Err)

	require.NoError(t, err)
	assert.Equal(t, 1, f.applied)
	assert.Empty(t, out.String())
}

func TestAFailedObserveEndsTheRun(t *testing.T) {
	f := newFake()
	failed := errors.New("no repository")
	script := f.script()
	script.Observe = func(context.Context) (func(), error) { return func() {}, failed }
	ios, _, _, _ := output.Test()

	err := Run(t.Context(), Params{Script: script, DryRun: true}, ios.In, ios.Err)

	assert.ErrorIs(t, err, failed)
	assert.Zero(t, f.applied)
}

func TestAStepThatRefreshesIsReobservedOnItsKey(t *testing.T) {
	f := newFake()
	refreshing := &refreshingStep{fakeStep: f.first}
	script := f.script()
	script.Steps[0] = refreshing
	d := open(t, Params{Script: script})
	assert.Contains(t, d.screen(), "enter confirm")
	assert.Contains(t, d.screen(), "r refresh")

	d.keys("r")

	assert.Equal(t, 2, f.observed)
	assert.Equal(t, 1, refreshing.refreshed)
	assert.Equal(t, 0, d.w.Current())
}

func TestAnAnswerAndABackAreObservedOffTheLoopWhileTheStepSaysItIsReading(t *testing.T) {
	f := newFake()
	d := open(t, Params{Script: f.script()})

	_, cmd := d.w.Update(keyOf("enter"))
	assert.Equal(t, 1, f.observed)
	assert.Equal(t, 0, d.w.Current())
	assert.Contains(t, d.screen(), "Reading the repository…")
	d.run(cmd)
	assert.Equal(t, 2, f.observed)
	assert.Equal(t, 1, d.w.Current())
	assert.NotContains(t, d.screen(), "Reading the repository…")

	_, cmd = d.w.Update(keyOf("esc"))
	assert.Equal(t, 2, f.observed)
	assert.Contains(t, d.screen(), "Reading the repository…")
	d.run(cmd)
	assert.Equal(t, 3, f.observed)
	assert.Equal(t, 0, d.w.Current())
	assert.NotContains(t, d.screen(), "Reading the repository…")
}

func TestKeysTypedWhileTheRepositoryIsReadAreReplayedInOrder(t *testing.T) {
	f := newFake()
	d := open(t, Params{Script: f.script()})

	_, cmd := d.w.Update(keyOf("enter"))
	d.w.Update(keyOf("down"))
	d.w.Update(keyOf("enter"))
	d.run(cmd)

	assert.Equal(t, 2, d.w.Current())
	assert.Equal(t, "no", f.second.answer)
	assert.Equal(t, 3, f.observed)
}

func TestGoingBackInARestartedFormAsksAgainTheStepsItResets(t *testing.T) {
	f := newFake()
	f.first.standing = Standing{Skip: "not yet"}
	d := open(t, Params{Script: f.script()})
	d.keys("down", "enter")
	require.Equal(t, 2, d.w.Current())

	f.first.standing = Standing{}
	again := start(d.w.Again())
	again.keys("enter")
	require.Equal(t, 2, again.w.Current())
	again.keys("esc", "enter")

	assert.Equal(t, 1, again.w.Current())
	assert.NotContains(t, again.screen(), "✓ Second:")
}

func TestAStalePlanAsksAgainAndAppliesUnderThePreviewOfTheNewForm(t *testing.T) {
	f := newFake()
	script := f.script()
	previews := 0
	script.Preview = func(context.Context) (func(), error) { previews++; return func() {}, nil }
	var appliedAfter []int
	script.Apply = func(context.Context) error {
		appliedAfter = append(appliedAfter, previews)
		if len(appliedAfter) == 1 {
			return errStale
		}
		return nil
	}
	ios, _, _, _ := output.Test()

	err := Run(t.Context(), Params{Script: script}, enters{}, ios.Err)

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, appliedAfter)
}

func TestAFailedApplyEndsTheRunWithoutAskingAgain(t *testing.T) {
	f := newFake()
	failed := errors.New("hook failed")
	f.applyErr = failed
	ios, _, _, _ := output.Test()

	err := Run(t.Context(), Params{Script: f.script()}, enters{}, ios.Err)

	assert.ErrorIs(t, err, failed)
	assert.Equal(t, 1, f.applied)
}

func TestAReadThatFailsAfterAnAnswerEndsTheRun(t *testing.T) {
	f := newFake()
	failed := errors.New("repository gone")
	script := f.script()
	reads := 0
	script.Observe = func(context.Context) (func(), error) {
		reads++
		if reads > 1 {
			return func() {}, failed
		}
		return func() {}, nil
	}
	ios, _, _, _ := output.Test()

	err := Run(t.Context(), Params{Script: script}, enters{}, ios.Err)

	assert.ErrorIs(t, err, failed)
	assert.Equal(t, 2, reads)
	assert.Zero(t, f.applied)
}

func TestTheRecapNamesItsApplyAsTheScriptSays(t *testing.T) {
	f := newFake()
	script := f.script()
	script.ApplyLabel = func() string { return "Yes, extract" }
	d := open(t, Params{Script: script})

	d.keys("enter", "enter")

	assert.Contains(t, d.screen(), "Yes, extract")
	assert.NotContains(t, d.screen(), "Yes, apply")
}
