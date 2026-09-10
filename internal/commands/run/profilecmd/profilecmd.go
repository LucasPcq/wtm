// Package profilecmd implements `wtm run profile add|rm|edit|list` — CRUD on
// profiles in run.toml. The runners here read flags and pick the two seams;
// what they ask and what they write lives in internal/flow/run/profile.
package profilecmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	profileflow "github.com/LucasPcq/wtm/internal/flow/run/profile"
	"github.com/LucasPcq/wtm/internal/output"
)

// NewCmd creates the wtm run profile command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdProfile,
		Short: "Add, remove, or edit profiles in run.toml",
		Long:  "Manage profiles declared in <git-common-dir>/wtm/run.toml.",
	}

	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newRmCmd())
	cmd.AddCommand(newEditCmd())
	cmd.AddCommand(newListCmd())

	return cmd
}

type presenter struct {
	shared.CLIPresenter
}

func (p presenter) Changed(outcome profileflow.Outcome) error {
	out := p.Cmd.OutOrStdout()
	if p.Format == domain.OutputJSON {
		return output.WriteProfileResultJSON(out, output.ProfileActionResult{
			Name:   outcome.Name,
			Status: outcome.Status,
		})
	}

	output.Frame(out, func(w io.Writer) {
		switch outcome.Status {
		case domain.JobActionUpdated:
			output.Update(w, fmt.Sprintf(domain.RunProfileUpdatedFmt, outcome.Name))
		case domain.JobActionRemoved:
			output.Success(w, fmt.Sprintf(domain.RunProfileRemovedFmt, outcome.Name))
		default:
			output.Success(w, fmt.Sprintf(domain.RunProfileAddedFmt, outcome.Name))
		}
	})
	return nil
}
