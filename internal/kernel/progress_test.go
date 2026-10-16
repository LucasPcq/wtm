package kernel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestAReporterSaysEachThingAboutItsSubject(t *testing.T) {
	var got []kernel.Progress
	report := kernel.Report(kernel.EmitFunc(func(progress kernel.Progress) { got = append(got, progress) }), "feat/x")

	report.UnitStarted()
	finished := report.Phase("hooks")
	report.Output("npm install")
	report.Status("test.installing", kernel.Params{"package": "left-pad"})
	finished(false)
	report.UnitFinished(kernel.StatusDone)

	subject := kernel.Of{Subject: "feat/x"}
	assert.Equal(t, []kernel.Progress{
		&kernel.UnitStarted{Of: subject},
		&kernel.PhaseStarted{Of: subject, Phase: "hooks"},
		&kernel.Output{Of: subject, Line: "npm install"},
		&kernel.StatusChanged{Of: subject, Code: "test.installing", Params: kernel.Params{"package": "left-pad"}},
		&kernel.PhaseFinished{Of: subject, Phase: "hooks"},
		&kernel.UnitFinished{Of: subject, Status: kernel.StatusDone},
	}, got)
}

func TestAPhaseSaysWhenItFailed(t *testing.T) {
	var got []kernel.Progress
	report := kernel.Report(kernel.EmitFunc(func(progress kernel.Progress) { got = append(got, progress) }), "feat/x")

	report.Phase("hooks")(true)

	assert.Equal(t, &kernel.PhaseFinished{Of: kernel.Of{Subject: "feat/x"}, Phase: "hooks", Failed: true}, got[1])
}

func TestAReporterWithNoEmitterSaysNothing(t *testing.T) {
	report := kernel.Report(nil, "feat/x")
	assert.NotPanics(t, func() {
		report.UnitStarted()
		report.Phase("hooks")(false)
		report.Output("line")
	})
}

func TestAReporterKeepsTheWarningsAboutItsSubject(t *testing.T) {
	report := kernel.Report(nil, "feat/x")
	report.Warn(kernel.Warning{Code: "test.env_skipped", Params: kernel.Params{"file": ".env"}})
	assert.Equal(t, []kernel.Warning{{Code: "test.env_skipped", Params: kernel.Params{"file": ".env"}}}, report.Warnings())
}

func TestEachStepIsHandedTheReporterOfItsItem(t *testing.T) {
	progress := &recorder{}
	warning := kernel.Warning{Code: "test.env_skipped"}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Emit:  progress,
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{
				Before: []kernel.Prep{{Name: "stop jobs", Run: func(_ context.Context, report kernel.Reporter) error { report.Output("stopping"); return nil }}},
				Saga: kernel.Saga[string]{
					Steps:  []kernel.SagaStep{{Name: "env", Do: func(_ context.Context, report kernel.Reporter) error { report.Warn(warning); return nil }}},
					Commit: func(context.Context) (string, error) { return "/wt/a", nil },
				},
				Then: []kernel.Phase[string]{{Name: "hooks", Run: func(_ context.Context, report kernel.Reporter, path string) (string, error) {
					report.Output("installing")
					return path, nil
				}}},
			}
		},
	})
	assert.Equal(t, []kernel.Warning{warning}, items[0].Warnings, "a warning a step reports lands on its item")
	assert.Contains(t, progress.lines(), "a output stopping")
	assert.Contains(t, progress.lines(), "a output installing")
}

func TestAPhaseNamesTheLogItsOutputIsKeptIn(t *testing.T) {
	var started []kernel.PhaseStarted
	progress := kernel.EmitFunc(func(p kernel.Progress) {
		if phase, ok := p.(*kernel.PhaseStarted); ok {
			started = append(started, *phase)
		}
	})
	each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Emit:  progress,
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(&disk{}, branch)
			unit.Before = []kernel.Prep{{Name: "on_clean", Log: "/logs/clean", Run: func(context.Context, kernel.Reporter) error { return nil }}}
			unit.Then = []kernel.Phase[string]{{Name: "on_create", Log: "/logs/create", Run: func(_ context.Context, _ kernel.Reporter, path string) (string, error) { return path, nil }}}
			return unit
		},
	})
	logs := map[string]string{}
	for _, phase := range started {
		logs[phase.Phase] = phase.Log
	}
	assert.Equal(t, map[string]string{"on_clean": "/logs/clean", "worktree": "", "meta": "", "on_create": "/logs/create"}, logs)
}
