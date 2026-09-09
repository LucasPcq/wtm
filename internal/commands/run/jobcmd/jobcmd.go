// Package jobcmd implements `wtm run job add|rm|edit|list` — CRUD on jobs in
// run.toml. The runners here read flags and pick the two seams; what they ask
// and what they write lives in internal/flow/run/job.
package jobcmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	jobflow "github.com/LucasPcq/wtm/internal/flow/run/job"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

// NewCmd creates the wtm run job command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdJob,
		Short: "Add, remove, or edit jobs in run.toml",
		Long:  "Manage jobs declared in <git-common-dir>/wtm/run.toml.",
	}

	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newRmCmd())
	cmd.AddCommand(newEditCmd())
	cmd.AddCommand(newListCmd())

	return cmd
}

// presenter turns what the flow changed into the two shapes this surface has:
// a document, or the lines a reader sees.
type presenter struct {
	shared.CLIPresenter
}

func (p presenter) Changed(outcome jobflow.Outcome) error {
	out := p.Cmd.OutOrStdout()
	if p.Format == domain.OutputJSON {
		return output.WriteJobResultJSON(out, domain.JobActionResult{
			Name:   outcome.Name,
			Status: outcome.Status,
		})
	}

	output.Frame(out, func(w io.Writer) {
		switch outcome.Status {
		case domain.JobActionUpdated:
			output.Update(w, fmt.Sprintf(domain.RunJobUpdatedFmt, outcome.Name))
		case domain.JobActionRemoved:
			output.Success(w, fmt.Sprintf(domain.RunJobRemovedFmt, outcome.Name))
		default:
			output.Success(w, fmt.Sprintf(domain.RunJobAddedFmt, outcome.Name))
		}
		// What the removal dragged along, each named so the reader can put it back.
		for _, line := range removalLines(outcome.Effect) {
			output.Message(w, line)
		}
	})
	return nil
}

func removalLines(effect rules.RemoveJobEffect) []string {
	var lines []string
	for _, part := range []struct {
		format string
		names  []string
	}{
		{domain.JobRemovedProfilesFmt, effect.Profiles},
		{domain.JobRemovedEmptiedFmt, effect.EmptiedProfiles},
		{domain.JobRemovedEnvPortsFmt, effect.EnvPorts},
		{domain.JobRemovedRunnersFmt, effect.Runners},
	} {
		if len(part.names) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf(part.format, strings.Join(part.names, domain.RunURLListSep)))
	}
	return lines
}
