package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThePublishedSchemaIsTheOneTheGoTypesGive(t *testing.T) {
	generated := generatedSchema(t)
	if updating() {
		// the binary embeds the schema it was built with: the next run checks the one written
		require.NoError(t, os.WriteFile(apiSchemaFile, generated, 0o644))
		return
	}

	assert.Equal(t, string(readFile(t, apiSchemaFile)), string(generated), "api.v2.schema.json is stale: run make schema")
	assert.Equal(t, string(readFile(t, apiSchemaFile)), string(api.Schema()), "api embeds another schema than the one on disk: rebuild")
}

func TestEachGoldenOfTheCatalogueMatchesTheSchema(t *testing.T) {
	defs := apiDefs(t)
	goldens, err := filepath.Glob(filepath.Join("testdata", "catalogue_*.golden"))
	require.NoError(t, err)
	require.NotEmpty(t, goldens)

	for _, golden := range goldens {
		part := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(golden), "catalogue_"), ".golden")
		name, shape, _ := strings.Cut(part, "_")
		t.Run(part, func(t *testing.T) {
			document := readFile(t, golden)
			switch shape {
			case "form", "questions":
				assert.NoError(t, validates(t, defs("domain.Form"), document))
			case "events":
				var events []json.RawMessage
				require.NoError(t, json.Unmarshal(document, &events))
				for _, event := range events {
					assert.NoError(t, validates(t, defs("domain.Event"), event))
				}
			default:
				assert.NoError(t, validates(t, defs(name+".envelope"), document))
			}
		})
	}
}

func TestAFieldOfEachKindMatchesTheSchema(t *testing.T) {
	form := domain.Form{Fields: fieldKinds(), Sections: []domain.Section{{Key: "where", Title: "Where", Summary: "next to main"}}}
	document, err := json.Marshal(form)
	require.NoError(t, err)

	assert.NoError(t, validates(t, apiDefs(t)("domain.Form"), document))
}

func TestAnEventOfEachTypeMatchesTheSchema(t *testing.T) {
	defs := apiDefs(t)

	for _, event := range eventsOfEachType() {
		t.Run(domain.EventType(event), func(t *testing.T) {
			document, err := domain.MarshalEvent(event)
			require.NoError(t, err)
			assert.NoError(t, validates(t, defs("domain.Event"), document))
		})
	}
}

func TestTheAnswersOfEachCommandsFormMatchItsRequest(t *testing.T) {
	defs := apiDefs(t)

	for _, command := range catalogue() {
		t.Run(command.Name, func(t *testing.T) {
			document, err := json.Marshal(command.Form.Answers())
			require.NoError(t, err)
			assert.NoError(t, validates(t, defs(command.Name+".request"), document))
		})
	}
}

func TestTheSchemaRefusesWhatNoCommandSends(t *testing.T) {
	defs := apiDefs(t)

	for name, c := range map[string]struct {
		def      string
		document string
	}{
		"unknown_path":       {def: "create.request", document: `[{"path":"brnach","values":["x"]}]`},
		"two_values_of_one":  {def: "create.request", document: `[{"path":"from","values":["main","dev"]}]`},
		"plan_on_a_read":     {def: "list.envelope", document: `{"v":2,"command":"list","ok":true,"plan":{},"follow_ups":[],"warnings":[]}`},
		"field_not_in_plan":  {def: "create.envelope", document: `{"v":2,"command":"create","ok":true,"plan":{"branches":[],"env_from":"main","creates":0,"surprise":1},"follow_ups":[],"warnings":[]}`},
		"failure_no_error":   {def: "create.envelope", document: `{"v":2,"command":"create","ok":false,"follow_ups":[],"warnings":[]}`},
		"field_of_no_kind":   {def: "domain.Form", document: `{"fields":[{"kind":"slider","path":"x"}],"sections":[],"problems":[]}`},
		"another_command":    {def: "create.envelope", document: `{"v":2,"command":"extract","ok":true,"follow_ups":[],"warnings":[]}`},
		"event_of_no_fields": {def: "domain.Event", document: `{"type":"unit.started"}`},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, validates(t, defs(c.def), []byte(c.document)))
		})
	}
}
