package run

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

// backingOut stands in for a terminal whose user presses Esc at the first
// question.
func backingOut(t *testing.T) {
	t.Helper()
	previousTTY, previousPrompter := runctx.IsTTY, shared.InteractivePrompter
	runctx.IsTTY = func() bool { return true }
	shared.InteractivePrompter = func(context.Context, shared.FlowPrompterParams) flow.Prompter {
		return &flowtest.ScriptedPrompter{Abort: true}
	}
	t.Cleanup(func() { runctx.IsTTY, shared.InteractivePrompter = previousTTY, previousPrompter })
}

func abortableConfig() domain.RunConfig {
	published := func(name string, port int) domain.JobConfig {
		return domain.JobConfig{Name: name, Kind: domain.JobKindService, Cmd: "true",
			Ports: map[string]int{"PORT": port}, URL: &domain.JobURLConfig{Port: "PORT"}}
	}
	return domain.RunConfig{
		Jobs: []domain.JobConfig{published("api", 4100), published("web", 4203)},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api", "web"}},
			{Name: "back", Jobs: []string{"api"}},
		},
	}
}

// Backing out of a run command's question exits 19 like every other picker: the
// runner ends on the cancelled mark, which the root reads to turn its ErrAborted
// into that code.
func TestBackingOutOfARunCommandExitsCancelled(t *testing.T) {
	cases := [][]string{
		{domain.CmdUp},
		{domain.CmdStart},
		{domain.CmdOpen},
		{domain.CmdLogs},
		{domain.CmdStop},
		{domain.CmdDown},
		{domain.CmdAddressing},
		{domain.CmdJob, domain.CmdAdd},
		{domain.CmdProfile, domain.CmdAdd},
		{domain.CmdJob, domain.CmdEdit},
		{domain.CmdJob, domain.CmdRm},
		{domain.CmdProfile, domain.CmdEdit},
		{domain.CmdProfile, domain.CmdRm},
	}
	for _, args := range cases {
		t.Run(args[len(args)-1]+"/"+args[0], func(t *testing.T) {
			stateDir := setupTestProject(t)
			writeRunTOML(t, stateDir, abortableConfig())
			startFakeDaemon(t, &fakeDaemon{})
			backingOut(t)

			root := NewCmd()
			root.SilenceUsage, root.SilenceErrors = true, true
			root.SetArgs(args)
			err := root.Execute()

			sub, _, findErr := root.Find(args)
			if findErr != nil {
				t.Fatal(findErr)
			}
			exit := rules.ExitCode(rules.BackedOut(rules.BackedOutParams{Err: err, Cancelled: shared.Cancelled(sub)}))
			if exit != domain.ExitCodeCancelled {
				t.Errorf("err = %v, cancelled = %v; want exit %d", err, shared.Cancelled(sub), domain.ExitCodeCancelled)
			}
		})
	}
}
