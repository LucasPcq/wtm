// Package statefile decides what a state file's schema version allows.
package statefile

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain/statefile"
)

type Schema struct {
	Path string
	// Found is zero when the file carries no schema.
	Found     int
	Supported int
}

// Newer reads a file without a schema as at 1.
func Newer(schema Schema) bool {
	return max(schema.Found, 1) > schema.Supported
}

// Written is zero, so omitted, while the schema is still 1.
func Written(supported int) int {
	if supported <= 1 {
		return 0
	}
	return supported
}

// NewerError unwraps to statefile.ErrSchemaNewer, which sets the exit code.
type NewerError struct {
	Schema
}

func (e *NewerError) Error() string {
	return fmt.Sprintf(statefile.SchemaNewerFmt, e.Path, e.Found, e.Supported)
}

func (*NewerError) Unwrap() error { return statefile.ErrSchemaNewer }

func Refusal(schema Schema) error {
	if !Newer(schema) {
		return nil
	}
	return &NewerError{Schema: schema}
}
