package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

var schema = extract.Questions{}.Form()

// extractFlags spells each field path of the request as the argument or flag that sets it, for the failures; --remember is the contract's, not a field's.
var extractFlags = withRemember(schema.Flags())

// descriptions are the help of each field, by its path: the usage of its flag.
var descriptions = schema.Usages()

func withRemember(flags map[string]string) output.Flags {
	flags["remember"] = "--remember"
	return flags
}
