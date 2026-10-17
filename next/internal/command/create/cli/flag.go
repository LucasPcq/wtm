package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

var schema = create.Questions{}.Form()

// createFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard, and descriptions are the usage of each, by path: both read from create's form.
var createFlags, descriptions = spell(schema)

func spell(form domain.Form) (output.Flags, map[string]string) {
	return form.Flags()
}
