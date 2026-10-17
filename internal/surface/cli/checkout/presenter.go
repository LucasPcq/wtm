package checkout

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	checkoutflow "github.com/LucasPcq/wtm/internal/flow/checkout"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

type checkoutPresenter struct {
	shared.CLIPresenter
}

func (p checkoutPresenter) CheckedOut(outcome checkoutflow.Outcome) error {
	pr, result := outcome.PR, outcome.Result
	if p.Format == domain.OutputJSON {
		return render.WritePRCheckoutJSON(p.Cmd.OutOrStdout(), render.PRCheckoutJSON{
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
	render.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		render.FormatPRCheckoutResult(w, render.PRCheckoutResultParams{
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
