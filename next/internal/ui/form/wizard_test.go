package form_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachAskedStepIsEnteredThenTheRecapApplies(t *testing.T) {
	f := newFake()
	d := open(t, f.script())

	assert.Contains(t, d.Screen(), "Step 1/3 • First")
	assert.Contains(t, d.Screen(), "What First asks")
	assert.Contains(t, d.Screen(), "↑↓ navigate • / filter • enter confirm • esc cancel")

	d.Keys("enter")
	assert.Contains(t, d.Screen(), "Step 2/3 • Second")
	assert.Contains(t, d.Screen(), "✓ First: yes")
	assert.Contains(t, d.Screen(), "esc back")

	d.Keys("down", "enter")
	screen := d.Screen()
	assert.Contains(t, screen, "Step 3/3 • Confirm")
	assert.Contains(t, screen, "✓ Second: no")
	assert.Contains(t, screen, "plan: yes, no")

	d.Keys("enter")
	assert.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Empty(t, d.Wizard.View())
	assert.Equal(t, 3, f.observed)
}

func TestASettledStepIsNotAskedAndSaysWhatSettledIt(t *testing.T) {
	cases := map[string]struct {
		Standing form.Standing
		Line     string
	}{
		"a flag":             {Standing: form.Standing{Value: "x", Given: true, Flag: "--first"}, Line: "✓ First: x · --first"},
		"a remembered value": {Standing: form.Standing{Value: "x", Origin: domain.OriginRemembered}, Line: "✓ First: x · remembered"},
		"a skip":             {Standing: form.Standing{Skip: "not needed"}, Line: "· First — not needed"},
	}
	for name, c := range cases {
		f := newFake()
		f.first.standing = c.Standing
		d := open(t, f.script())

		assert.Equal(t, 1, d.Wizard.Current(), name)
		assert.Contains(t, d.Screen(), c.Line, name)
		assert.Contains(t, d.Screen(), "esc cancel", name)
	}
}

func TestAnErredStepIsAskedEvenWhenAFlagSettledIt(t *testing.T) {
	f := newFake()
	f.first.standing = form.Standing{Value: "x", Given: true, Flag: "--first", Erred: true}
	d := open(t, f.script())

	assert.Equal(t, 0, d.Wizard.Current())
}

func TestAKeptDefaultIsItsValueAlone(t *testing.T) {
	f := newFake()
	f.first.standing = form.Standing{Value: "yes", Origin: domain.OriginConfig}
	d := open(t, f.script())

	d.Keys("enter")

	assert.Contains(t, d.Screen(), "✓ First: yes\n")
}

func TestBackReturnsToTheLastAskedStepAndResetsTheOnesAfter(t *testing.T) {
	f := newFake()
	d := open(t, f.script())
	d.Keys("enter", "enter")
	require.Equal(t, 2, d.Wizard.Current())

	d.Keys("esc")

	assert.Equal(t, 1, d.Wizard.Current())
	assert.Zero(t, f.second.resets)
	d.Keys("esc")
	assert.Equal(t, 0, d.Wizard.Current())
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
		d := open(t, f.script())

		d.Keys(keys...)

		assert.Equal(t, form.Cancelled, d.Wizard.Ending(), name)
	}
}

func TestAPreviewThatFailsSaysWhyAndOffersOnlyToCancel(t *testing.T) {
	f := newFake()
	f.previewErr = domain.Invalid([]domain.FieldError{{Path: "first", Code: "required", Template: "{path} is required"}})
	d := open(t, f.script())

	d.Keys("enter", "enter")

	screen := d.Screen()
	assert.Contains(t, screen, "Step 3/3 • Cannot apply this plan")
	assert.Contains(t, screen, "✗ the request is invalid\n  ┃      FIRST is required")
	assert.NotContains(t, screen, "Yes, apply")
	d.Keys("enter")
	assert.Equal(t, form.Cancelled, d.Wizard.Ending())
}

func TestAFieldAFailedPreviewRefusesIsFollowedByItsStep(t *testing.T) {
	f := newFake()
	f.previewErr = domain.Invalid([]domain.FieldError{{Path: "first", Code: "taken", Template: "{path} is taken", FollowUp: &domain.FollowUp{Command: "go", Message: "jump in"}}})
	script := f.script()
	script.CommandLine = formtest.CommandLine
	d := open(t, script)

	d.Keys("enter", "enter")

	assert.Contains(t, d.Screen(), "FIRST is taken\n  ┃      → wtm go   jump in")
}

func TestAStaleRunAsksNothingAgainAndSaysWhy(t *testing.T) {
	f := newFake()
	d := open(t, f.script())
	d.Keys("enter", "enter")

	again := start(d.Wizard.Again())

	assert.Equal(t, 2, again.Wizard.Current())
	screen := again.Screen()
	assert.Contains(t, screen, "✓ First: yes\n")
	assert.Contains(t, screen, "! the facts changed since the preview")
}

func TestADryRunAsksTheStepsAndNoConfirmation(t *testing.T) {
	f := newFake()
	d := open(t, dryRun(f.script()))

	assert.Contains(t, d.Screen(), "Step 1/2 • First")
	d.Keys("enter", "enter")

	assert.Equal(t, form.Finished, d.Wizard.Ending())
}

func TestTheRefreshKeyReobservesOnlyAStepThatRefreshes(t *testing.T) {
	f := newFake()
	d := open(t, f.script())

	d.Keys("r")

	assert.Equal(t, 1, f.observed)
	assert.NotContains(t, d.Screen(), "r refresh")
}

func TestADryRunWithNothingToAskAppliesWithoutDrawing(t *testing.T) {
	f := newFake()
	f.first.standing = form.Standing{Skip: "no"}
	f.second.standing = form.Standing{Skip: "no"}
	ios, _, _, out := output.Test()

	err := form.Run(t.Context(), dryRun(f.script()), ios.In, ios.Err)

	require.NoError(t, err)
	assert.EqualValues(t, 1, f.applied.Load())
	assert.Empty(t, out.String())
}

func TestAFailedObserveEndsTheRun(t *testing.T) {
	f := newFake()
	failed := errors.New("no repository")
	script := f.script()
	script.Observe = func(context.Context) (func(), error) { return func() {}, failed }
	ios, _, _, _ := output.Test()

	err := form.Run(t.Context(), dryRun(script), ios.In, ios.Err)

	assert.ErrorIs(t, err, failed)
	assert.Zero(t, f.applied.Load())
}

func TestAStepThatRefreshesIsReobservedOnItsKey(t *testing.T) {
	f := newFake()
	refreshing := &refreshingStep{fakeStep: f.first}
	script := f.script()
	script.Steps[0] = refreshing
	d := open(t, script)
	assert.Contains(t, d.Screen(), "enter confirm")
	assert.Contains(t, d.Screen(), "r refresh")

	d.Keys("r")

	assert.Equal(t, 2, f.observed)
	assert.Equal(t, 1, refreshing.refreshed)
	assert.Equal(t, 0, d.Wizard.Current())
}

func TestAnAnswerAndABackAreObservedOffTheLoopWhileTheStepSaysItIsReading(t *testing.T) {
	f := newFake()
	d := open(t, f.script())

	_, cmd := d.Wizard.Update(formtest.Key("enter"))
	assert.Equal(t, 1, f.observed)
	assert.Equal(t, 0, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "Reading the repository…")
	d.Run(cmd)
	assert.Equal(t, 2, f.observed)
	assert.Equal(t, 1, d.Wizard.Current())
	assert.NotContains(t, d.Screen(), "Reading the repository…")

	_, cmd = d.Wizard.Update(formtest.Key("esc"))
	assert.Equal(t, 2, f.observed)
	assert.Contains(t, d.Screen(), "Reading the repository…")
	d.Run(cmd)
	assert.Equal(t, 3, f.observed)
	assert.Equal(t, 0, d.Wizard.Current())
	assert.NotContains(t, d.Screen(), "Reading the repository…")
}

func TestKeysTypedWhileTheRepositoryIsReadAreReplayedInOrder(t *testing.T) {
	f := newFake()
	d := open(t, f.script())

	_, cmd := d.Wizard.Update(formtest.Key("enter"))
	d.Wizard.Update(formtest.Key("down"))
	d.Wizard.Update(formtest.Key("enter"))
	d.Run(cmd)

	assert.Equal(t, 2, d.Wizard.Current())
	assert.Equal(t, "no", f.second.answer)
	assert.Equal(t, 3, f.observed)
}

func TestGoingBackInARestartedFormAsksAgainTheStepsItResets(t *testing.T) {
	f := newFake()
	f.first.standing = form.Standing{Skip: "not yet"}
	d := open(t, f.script())
	d.Keys("down", "enter")
	require.Equal(t, 2, d.Wizard.Current())

	f.first.standing = form.Standing{}
	again := start(d.Wizard.Again())
	again.Keys("enter")
	require.Equal(t, 2, again.Wizard.Current())
	again.Keys("esc", "enter")

	assert.Equal(t, 1, again.Wizard.Current())
	assert.NotContains(t, again.Screen(), "✓ Second:")
}

func TestAStalePlanAsksAgainAndAppliesUnderThePreviewOfTheNewForm(t *testing.T) {
	f := newFake()
	script := f.script()
	// both run in cmds, on goroutines of Bubble Tea's own
	var previews atomic.Int32
	script.Preview = func(context.Context) (func(), error) { previews.Add(1); return func() {}, nil }
	var (
		mu           sync.Mutex
		appliedAfter []int32
	)
	script.Apply = func(context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		appliedAfter = append(appliedAfter, previews.Load())
		if len(appliedAfter) == 1 {
			return errStale
		}
		return nil
	}
	ios, _, _, _ := output.Test()

	err := form.Run(t.Context(), script, enters{}, ios.Err)

	require.NoError(t, err)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int32{1, 2}, appliedAfter)
}

func TestAFailedApplyEndsTheRunWithoutAskingAgain(t *testing.T) {
	f := newFake()
	failed := errors.New("hook failed")
	f.applyErr = failed
	ios, _, _, _ := output.Test()

	err := form.Run(t.Context(), f.script(), enters{}, ios.Err)

	assert.ErrorIs(t, err, failed)
	assert.EqualValues(t, 1, f.applied.Load())
}

func TestAReadThatFailsAfterAnAnswerEndsTheRun(t *testing.T) {
	f := newFake()
	failed := errors.New("repository gone")
	script := f.script()
	var reads atomic.Int32
	script.Observe = failingAfterFirst(&reads, failed)
	ios, _, _, _ := output.Test()

	err := form.Run(t.Context(), script, enters{}, ios.Err)

	assert.ErrorIs(t, err, failed)
	assert.EqualValues(t, 2, reads.Load())
	assert.Zero(t, f.applied.Load())
}

func TestAKeyTypedBeforeAFailedReadQuitsIsNotAnswered(t *testing.T) {
	f := newFake()
	script := f.script()
	var reads atomic.Int32
	script.Observe = failingAfterFirst(&reads, errors.New("repository gone"))
	d := open(t, script)

	d.Keys("enter", "enter")

	assert.EqualValues(t, 2, reads.Load())
	assert.Equal(t, form.Failed, d.Wizard.Ending())
}

func TestAReadThatLandsAfterACancelLeavesTheWizardCancelled(t *testing.T) {
	cases := map[string]func(*form.Script){
		"dry_run": func(s *form.Script) { s.DryRun = true },
		"walk":    func(s *form.Script) { s.Preview = nil },
		"recap":   func(*form.Script) {},
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			script := newFake().script()
			edit(&script)
			d := open(t, script)
			d.Keys("enter")

			d.Keys("ctrl+c")
			d.Send(form.ReadLanded())

			assert.Equal(t, form.Cancelled, d.Wizard.Ending())
			assert.Equal(t, 1, d.Wizard.Current(), "the wizard moved on after the cancel")
		})
	}
}

func TestTheRecapNamesItsApplyAsTheScriptSays(t *testing.T) {
	f := newFake()
	script := f.script()
	script.ApplyLabel = func() string { return "Yes, extract" }
	d := open(t, script)

	d.Keys("enter", "enter")

	assert.Contains(t, d.Screen(), "Yes, extract")
	assert.NotContains(t, d.Screen(), "Yes, apply")
}

func TestAScriptWithoutPreviewEndsOnItsLastStepWithNoRecap(t *testing.T) {
	f := newFake()
	script := f.script()
	script.Preview = nil
	d := open(t, script)

	assert.Contains(t, d.Screen(), "Step 1/2 • First")
	d.Keys("enter", "enter")

	assert.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Zero(t, f.applied.Load())
}

func TestARefreshReadsWithTheScriptsRefreshWhenItHasOne(t *testing.T) {
	f := newFake()
	refreshing := &refreshingStep{fakeStep: f.first}
	refreshes := 0
	script := f.script()
	script.Steps[0] = refreshing
	script.Refresh = func(context.Context) (func(), error) { return func() { refreshes++ }, nil }
	d := open(t, script)

	d.Keys("r")

	assert.Equal(t, 1, f.observed)
	assert.Equal(t, 1, refreshes)
	assert.Equal(t, 1, refreshing.refreshed)
}

func TestWhatLandsLaterRebuildsTheStepAsked(t *testing.T) {
	f := newFake()
	refreshing := &refreshingStep{fakeStep: f.first}
	landed := 0
	script := f.script()
	script.Steps[0] = refreshing
	script.Later = func(context.Context) (func(), error) { return func() { landed++ }, nil }
	d := open(t, script)

	assert.Equal(t, 1, landed)
	assert.Equal(t, 1, refreshing.refreshed)
	assert.Equal(t, 0, d.Wizard.Current())
}
