package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

var schema = create.Questions{}.Form()

// createFlags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard; --remember is the contract's, not a field's.
var createFlags = withRemember(schema.Flags())

// descriptions are the help of each field, by its path: the usage of its flag and the description of its wizard step.
var descriptions = schema.Usages()

func withRemember(flags map[string]string) output.Flags {
	flags["remember"] = "--remember"
	return flags
}
