package kernel_test

import (
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
	finished()
	report.UnitFinished(kernel.StatusDone)

	assert.Equal(t, []kernel.Progress{
		{Subject: "feat/x", Kind: kernel.ProgressUnitStarted},
		{Subject: "feat/x", Kind: kernel.ProgressPhaseStarted, Params: kernel.Params{kernel.ParamPhase: "hooks"}},
		{Subject: "feat/x", Kind: kernel.ProgressOutput, Line: "npm install"},
		{Subject: "feat/x", Kind: kernel.ProgressStatus, Code: "test.installing", Params: kernel.Params{"package": "left-pad"}},
		{Subject: "feat/x", Kind: kernel.ProgressPhaseFinished, Params: kernel.Params{kernel.ParamPhase: "hooks"}},
		{Subject: "feat/x", Kind: kernel.ProgressUnitFinished, Params: kernel.Params{kernel.ParamStatus: "done"}},
	}, got)
}

func TestAReporterWithNoEmitterSaysNothing(t *testing.T) {
	report := kernel.Report(nil, "feat/x")
	assert.NotPanics(t, func() {
		report.UnitStarted()
		report.Phase("hooks")()
		report.Output("line")
	})
}
