package cli

import (
	"cmp"
	"context"
	"io"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/command/initialize"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// initForm is init over the ui/form engine, which asks every setting by its kind.
type initForm = form.Command[initialize.Request, initialize.Questions, initialize.Outcome]

// initFlags spells each field path of the request as the flag that sets it, for the failures and the wizard.
var initFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (initialize.UseCase, error)) *initForm {
	return &initForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[initialize.Request, initialize.Questions, initialize.Outcome](open),
		Preview:    func(previewed initialize.Outcome) form.Preview { return form.PreviewOf(worded(previewed)) },
		Draw:       func(w io.Writer, applied initialize.Outcome, view output.View) { drawOutcome(w, worded(applied), view) },
		ApplyLabel: applyLabel,
	}
}

// applyLabel names the write the plan previewed makes: a first set-up, or the settings set up again.
func applyLabel(previewed initialize.Outcome) string {
	if previewed.Plan.Created {
		return "Yes, set it up"
	}

	return "Yes, save the settings"
}

// worded is the outcome with its recap in the words of the settings the wizard asks with: each row's label, and the strategy as its option reads in the trail.
func worded(outcome initialize.Outcome) initialize.Outcome {
	rows := slices.Clone(outcome.Plan.Recap)
	for i, row := range rows {
		said, ok := form.Settings{}.Words(row.Path)
		if !ok {
			continue
		}
		rows[i].Label = said.Label
		if row.Path != domain.PathEnvStrategy {
			continue
		}
		// a value no option names (a hand-edited config.toml's) reads as it is
		short := func(value string) string { return cmp.Or(said.Options[value].Short, value) }
		values := slices.Clone(row.Values)
		for j, value := range values {
			values[j].Text = short(value.Value)
			if was := value.Params["was"]; value.Note == initialize.NoteChanged {
				values[j].Text += " (was " + short(was) + ")"
			}
		}
		rows[i].Values = values
	}
	outcome.Plan.Recap = rows
	return outcome
}
