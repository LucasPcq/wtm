// Package statefile names the schema version of each file wtm keeps and what a
// file written by a newer wtm costs.
package statefile

import "errors"

// The schema each file is written at. A file without a schema key is at 1, so
// the key is written only once a file's schema reaches 2: until then a file
// this wtm writes stays readable by the releases before it.
const (
	ProjectConfigSchema   = 1
	GlobalConfigSchema    = 1
	RunConfigSchema       = 1
	MetaSchema            = 1
	PendingRemovalsSchema = 1
)

// ErrSchemaNewer is a write refused because the file was written by a newer
// wtm: this one reads it as far as it understands it, and leaves it alone.
var ErrSchemaNewer = errors.New("upgrade wtm to change it")

const (
	SchemaNewerFmt    = "%s is at schema %d, newer than the %d this wtm writes: %w"
	SchemaNewerTitle  = "Written by a newer wtm"
	SchemaNewerWhyFmt = "%s is at schema %d, newer than the %d this wtm knows"
	SchemaNewerCost   = "this wtm reads it as far as it understands it, and refuses to write it"
	SchemaNewerFix    = "upgrade wtm before changing it"
)
