// Package statefile names the schema version of each file wtm keeps and the refusal of a newer one.
package statefile

import "errors"

const (
	ProjectConfigSchema   = 1
	GlobalConfigSchema    = 1
	RunConfigSchema       = 1
	MetaSchema            = 1
	PendingRemovalsSchema = 1
)

var ErrSchemaNewer = errors.New("state file written by a newer wtm")

const (
	SchemaNewerFmt    = "%s is at schema %d, newer than the %d this wtm writes: upgrade wtm to change it"
	SchemaNewerTitle  = "Written by a newer wtm"
	SchemaNewerWhyFmt = "%s is at schema %d, newer than the %d this wtm knows"
	SchemaNewerCost   = "this wtm reads it as far as it understands it, and refuses to write it"
	SchemaNewerFix    = "upgrade wtm before changing it"
)
