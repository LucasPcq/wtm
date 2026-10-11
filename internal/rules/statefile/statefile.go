// Package statefile decides what a state file's schema version allows.
package statefile

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain/statefile"
)

type SchemaParams struct {
	// Found is the schema read from the file, zero when it carries none.
	Found     int
	Supported int
}

// Newer reports a file written by a newer wtm. A file without a schema is at 1.
func Newer(params SchemaParams) bool {
	return max(params.Found, 1) > params.Supported
}

// Written is the schema key to write for a file at supported: zero, so omitted,
// while the schema is still 1.
func Written(supported int) int {
	if supported <= 1 {
		return 0
	}
	return supported
}

type RefusalParams struct {
	Path   string
	Schema SchemaParams
}

// Refusal is the error of a write refused on a newer schema, nil when the
// write may go ahead.
func Refusal(params RefusalParams) error {
	if !Newer(params.Schema) {
		return nil
	}
	return fmt.Errorf(statefile.SchemaNewerFmt, params.Path, params.Schema.Found, params.Schema.Supported, statefile.ErrSchemaNewer)
}

// NewerLines say what a newer file costs, in the order a reader needs them.
func NewerLines(params RefusalParams) []string {
	return []string{
		fmt.Sprintf(statefile.SchemaNewerWhyFmt, params.Path, params.Schema.Found, params.Schema.Supported),
		statefile.SchemaNewerCost,
		statefile.SchemaNewerFix,
	}
}
