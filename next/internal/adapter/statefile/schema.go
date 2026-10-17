package statefile

import "github.com/LucasPcq/wtm/next/internal/domain"

// A Schema is the schema version Found in the state file at Path, against the
// newest version this wtm writes, Supported. A file without one is version 1.
type Schema struct {
	Path      string
	Found     int
	Supported int
}

// Refusal returns a *domain.SchemaNewer when the file's schema is newer than
// the one supported, nil otherwise.
func Refusal(schema Schema) error {
	if max(schema.Found, 1) <= schema.Supported {
		return nil
	}
	return &domain.SchemaNewer{Path: schema.Path, Found: schema.Found, Supported: schema.Supported}
}
