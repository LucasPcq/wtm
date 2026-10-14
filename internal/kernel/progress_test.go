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

func TestAReporterWithNoEmitterSaysNothing(t *testing.T) {
	report := kernel.Report(nil, "feat/x")
	assert.NotPanics(t, func() {
		report.UnitStarted()
		report.Phase("hooks")()
		report.Output("line")
	})
}

func TestAReporterKeepsTheWarningsAboutItsSubject(t *testing.T) {
	report := kernel.Report(nil, "feat/x")
	report.Warn(kernel.Warning{Code: "test.env_skipped", Params: kernel.Params{"file": ".env"}})
	assert.Equal(t, []kernel.Warning{{Code: "test.env_skipped", Params: kernel.Params{"file": ".env"}}}, report.Warnings())
}
