// Package statefile decides what a state file's schema version allows.
package statefile

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain/statefile"
)

// Schema is the version a state file carries against the one this wtm writes.
type Schema struct {
	Path string
	// Found is read from the file, zero when it carries none.
	Found     int
	Supported int
}

// Newer reports a file written by a newer wtm. A file without a schema is at 1.
func Newer(schema Schema) bool {
	return max(schema.Found, 1) > schema.Supported
}

// Written is the schema key to write for a file at supported: zero, so omitted,
// while the schema is still 1.
func Written(supported int) int {
	if supported <= 1 {
		return 0
	}
	return supported
}

// NewerError is a write refused because a newer wtm wrote the file. It carries
// the schema as data, for an interface to present, and unwraps to
// statefile.ErrSchemaNewer, which sets the exit code.
type NewerError struct {
	Schema
}

func (e *NewerError) Error() string {
	return fmt.Sprintf(statefile.SchemaNewerFmt, e.Path, e.Found, e.Supported)
}

func (*NewerError) Unwrap() error { return statefile.ErrSchemaNewer }

// Refusal is the error of a write over schema, nil when the write may go ahead.
func Refusal(schema Schema) error {
	if !Newer(schema) {
		return nil
	}
	return &NewerError{Schema: schema}
}
