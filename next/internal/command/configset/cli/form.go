package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/configset"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// setForm is config set over the ui/form engine.
type setForm = form.Command[configset.Request, configset.Questions, configset.Outcome]

// setFlags spells each field path of the request as its flag.
var setFlags = newForm(nil).Flags()

func newForm(open func(ctx context.Context, dir string) (configset.UseCase, error)) *setForm {
	return &setForm{
		YesUsage:   yesUsage,
		Words:      words,
		Open:       form.Opener[configset.Request, configset.Questions, configset.Outcome](open),
		Preview:    form.PreviewOf[configset.Plan, configset.Detail],
		Draw:       drawOutcome,
		ApplyLabel: func(configset.Outcome) string { return "Yes, write config.toml" },
		GivenOnly:  true,
	}
}
