package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type InvalidFlagValueParams struct {
	Flag    string
	Value   string
	Allowed []string
}

// InvalidFlagValue is one wording for a flag value that does not parse, whatever
// the flag: "invalid --mode value "x": use add or refresh". It carries
// domain.ErrUsage, so it exits 2 like any other command line refused before the
// command ran.
func InvalidFlagValue(params InvalidFlagValueParams) error {
	return flagValueError{text: fmt.Sprintf(domain.FlagValueInvalidFmt, params.Flag, params.Value, alternatives(params.Allowed))}
}

func alternatives(values []string) string {
	if len(values) < 2 {
		return strings.Join(values, "")
	}
	return strings.Join(values[:len(values)-1], domain.FlagValueListSep) + domain.FlagValueLastSep + values[len(values)-1]
}

type flagValueError struct{ text string }

func (e flagValueError) Error() string { return e.text }
func (e flagValueError) Unwrap() error { return domain.ErrUsage }
