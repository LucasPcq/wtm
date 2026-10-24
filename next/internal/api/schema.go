package api

import (
	_ "embed"
	"slices"
)

//go:embed api.v2.schema.json
var schema []byte

// Schema returns the published JSON Schema of the catalogue, api.v2.schema.json: the v2 envelope, the form and its fields, the events, each command's request, plan, detail and envelope, and each stream's request and envelope, generated from their Go types (make schema). Each call returns a copy of its own.
func Schema() []byte {
	return slices.Clone(schema)
}
