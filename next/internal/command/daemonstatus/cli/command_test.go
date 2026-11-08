package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/require"
)

func TestTheHelpOfDaemonStatus(t *testing.T) {
	got := runStatus(t, world{}, "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "status_help", got.Stdout)
}

func TestDaemonStatusDrawsTheDaemonAndItsJobsByRepository(t *testing.T) {
	cases := map[string]world{
		"running":     running(run.Stamp{Protocol: run.ProtocolVersion, Version: "0.31.0", PID: 4242}, run.SkewNone),
		"skewed":      running(run.Stamp{Protocol: run.ProtocolVersion, Version: "0.30.9", PID: 4242}, run.SkewBuild),
		"not_running": {Daemon: run.Daemon{Socket: "/cfg/wtm/wtm.sock", Index: "/cfg/wtm/jobs.json", IndexFrozen: true}},
	}
	for name, w := range cases {
		t.Run(name, func(t *testing.T) {
			got := runStatus(t, w)

			require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
			formtest.Golden(t, "status_"+name, got.Stdout)
		})
	}
}

func TestDaemonStatusInJSONIsTheReadEnvelope(t *testing.T) {
	got := runStatus(t, running(run.Stamp{Protocol: run.ProtocolVersion, Version: "0.30.9", PID: 4242}, run.SkewBuild), "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_skewed", got.Stdout)
}
