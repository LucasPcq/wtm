package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

var schema = extract.Questions{}.Form()

// extractFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard, and descriptions are the usage of each, by path: both read from extract's form.
var extractFlags, descriptions = spell(schema)

func spell(form domain.Form) (output.Flags, map[string]string) {
	return form.Flags()
}
