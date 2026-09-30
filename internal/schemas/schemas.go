// Package schemas embeds the JSON Schema files used to validate and provide
// IDE autocomplete for wtm's TOML config files. Schemas are shipped with the
// binary so they always match the current version's expected structure.
package schemas

import "embed"

//go:embed run.schema.json project.schema.json global.schema.json
var fs embed.FS

// Schema identifies one of the JSON Schema files bundled with wtm.
type Schema string

const (
	// Run is the schema for .wtm/run.toml (jobs + profiles).
	Run Schema = "run.schema.json"

	// Project is the schema for .wtm/config.toml (per-project settings).
	Project Schema = "project.schema.json"

	// Global is the schema for the global wtm config (per-user defaults).
	Global Schema = "global.schema.json"
)

// Filename returns the basename of the schema file (e.g. "run.schema.json").
func (s Schema) Filename() string { return string(s) }

// Bytes returns the raw JSON content for the schema.
func (s Schema) Bytes() []byte {
	data, err := fs.ReadFile(string(s))
	if err != nil {
		// Embedded files are checked at compile time; a missing file means a
		// programmer error in the embed directive, not user input.
		panic("schemas: missing embedded schema " + string(s) + ": " + err.Error())
	}
	return data
}
