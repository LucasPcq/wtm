package checkout

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	checkoutflow "github.com/LucasPcq/wtm/internal/flow/checkout"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

type checkoutPresenter struct {
	shared.CLIPresenter
}

func (p checkoutPresenter) CheckedOut(outcome checkoutflow.Outcome) error {
	pr, result := outcome.PR, outcome.Result
	if p.Format == domain.OutputJSON {
		return output.WritePRCheckoutJSON(p.Cmd.OutOrStdout(), output.PRCheckoutJSON{
			Number:         pr.Number,
			Branch:         pr.Branch,
			Path:           result.Path,
			Author:         pr.Author,
			URL:            pr.URL,
			Draft:          pr.Draft,
			ExistingBranch: result.ExistingBranch,
			OriginState:    result.OriginState,
			Isolation:      result.Isolation,
			EnvPorts:       result.EnvPorts,
			Warnings:       result.Warnings,
			Origins:        result.Origins,
		})
	}

	reusedNote := shared.ReusedBranchNoteResult{}
	if result.ExistingBranch {
		reusedNote = shared.ReusedBranchNote(shared.ReusedBranchNoteParams{
			Branch: outcome.Target.Branch,
			Ahead:  outcome.Target.AheadBehind.Ahead,
			Behind: outcome.Target.AheadBehind.Behind,
		})
	}
	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		output.FormatPRCheckoutResult(w, output.PRCheckoutResultParams{
			Number:            pr.Number,
			Branch:            pr.Branch,
			EnvNote:           rules.EnvPortSettlementNote(result.EnvPorts),
			Path:              result.Path,
			ReusedNote:        reusedNote.Text,
			ReusedNoteWarning: reusedNote.Warning,
			GoCommand:         fmt.Sprintf(domain.GoCommandFmt, pr.Branch),
		})
	})
	return nil
}
