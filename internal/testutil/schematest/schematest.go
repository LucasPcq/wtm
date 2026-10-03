// Package schematest compiles a bundled JSON Schema for a test to validate
// against.
package schematest

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/LucasPcq/wtm/internal/schemas"
)

func Compile(t testing.TB, schema schemas.Schema) *jsonschema.Schema {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema.Bytes()))
	if err != nil {
		t.Fatalf("parse %s: %v", schema.Filename(), err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schema.Filename(), doc); err != nil {
		t.Fatalf("add %s: %v", schema.Filename(), err)
	}
	compiled, err := compiler.Compile(schema.Filename())
	if err != nil {
		t.Fatalf("compile %s: %v", schema.Filename(), err)
	}
	return compiled
}

// Validate checks one JSON document, given as bytes, against a compiled schema.
func Validate(t testing.TB, schema *jsonschema.Schema, doc []byte) error {
	t.Helper()
	var value any
	if err := json.Unmarshal(doc, &value); err != nil {
		t.Fatalf("decode %s: %v", doc, err)
	}
	return schema.Validate(value)
}
