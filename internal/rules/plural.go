package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type PluralParams struct {
	Count int
	One   string
	Many  string
}

// Plural is a count with its noun agreed: "1 file", "2 files".
func Plural(params PluralParams) string {
	if params.Count == 1 {
		return fmt.Sprintf(domain.PluralFmt, params.Count, params.One)
	}
	return fmt.Sprintf(domain.PluralFmt, params.Count, params.Many)
}
