package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/web"
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
}

func TestEachPublishedEnumListsEveryConstantOfItsType(t *testing.T) {
	published := map[reflect.Type]any{reflect.TypeFor[domain.Phase](): domain.Phases()}
	for typ, e := range enums {
		published[typ] = e.values
	}

	for typ, values := range published {
		t.Run(typ.Name(), func(t *testing.T) {
			assert.ElementsMatch(t, declaredOf(t, typ), stringsOf(values), "a constant of %s the enum leaves out", typ)
		})
	}
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

// The envelopes a command's cli prints reach shapes the catalogue's goldens do not: a merged pull request, a failed hook, a refusal.
func TestEachEnvelopeGoldenOfACommandsCLIMatchesTheSchema(t *testing.T) {
	defs := apiDefs(t)
	goldens, err := filepath.Glob(filepath.Join("..", "..", "internal", "command", "*", "cli", "testdata", "envelope_*.golden"))
	require.NoError(t, err)
	require.NotEmpty(t, goldens)

	for _, golden := range goldens {
		name := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(golden))))
		t.Run(name+"_"+strings.TrimSuffix(strings.TrimPrefix(filepath.Base(golden), "envelope_"), ".golden"), func(t *testing.T) {
			assert.NoError(t, validates(t, defs(name+".envelope"), readFile(t, golden)))
		})
	}
}

// create and extract followed an apply up with the worktree to enter, saying nothing of it.
func TestEveryFollowUpOfAGoldenSaysWhatItDoes(t *testing.T) {
	goldens := append(globbed(t, filepath.Join("testdata", "catalogue_*.golden")), globbed(t, filepath.Join("..", "..", "internal", "command", "*", "cli", "testdata", "envelope_*.golden"))...)

	for _, golden := range goldens {
		var document any
		require.NoError(t, json.Unmarshal(readFile(t, golden), &document))
		for _, followUp := range followUpsIn(document) {
			assert.NotEmpty(t, followUp["message"], "%s: follow-up %v", golden, followUp["code"])
		}
	}
}

func TestAFieldOfEachKindMatchesTheSchema(t *testing.T) {
	form := domain.Form{Fields: fieldKinds()}
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

func TestTheAnswersOfEachFormMatchItsRequest(t *testing.T) {
	defs := apiDefs(t)

	forms := map[string]domain.Form{}
	for _, command := range catalogue() {
		forms[command.Name] = command.Form
	}
	for _, stream := range streams() {
		forms[stream.Name] = stream.Form
	}

	for name, form := range forms {
		t.Run(name, func(t *testing.T) {
			document, err := json.Marshal(form.Answers())
			require.NoError(t, err)
			assert.NoError(t, validates(t, defs(name+".request"), document))
		})
	}
}

func TestEachResponseOfTheWebServerMatchesTheSchema(t *testing.T) {
	defs := apiDefs(t)
	main := configured(t)
	ownConfigDir(t)
	server, _ := webServing(t, version)
	repo := registered(t, main)

	for name, c := range map[string]struct {
		def      string
		response func() *http.Response
	}{
		"version":  {def: "web.Version", response: func() *http.Response { return get(t, server, "/api/version", false) }},
		"repos":    {def: "web.Repos", response: func() *http.Response { return get(t, server, "/api/repos", false) }},
		"commands": {def: "web.Commands", response: func() *http.Response { return get(t, server, "/api/repos/"+repo+"/commands", false) }},
		"command_line": {def: "web.CommandLine", response: func() *http.Response {
			return post(t, server, "/api/command-line", `{"command":"list","code":"x","message":"List them"}`)
		}},
		"unspellable": {def: "web.CommandLine", response: func() *http.Response {
			return post(t, server, "/api/command-line", `{"code":"x","message":"Nothing to run"}`)
		}},
		"no_token":     {def: "web.Refusal", response: func() *http.Response { return get(t, server, "/api/version", true) }},
		"repo_unknown": {def: "web.Refusal", response: func() *http.Response { return post(t, server, "/api/repos/nope/commands/list", `{}`) }},
		"wrong_method": {def: "web.Refusal", response: func() *http.Response { return post(t, server, "/api/version", ``) }},
		"not_a_call":   {def: "web.Refusal", response: func() *http.Response { return post(t, server, "/api/repos/"+repo+"/commands/list", `[`) }},
	} {
		t.Run(name, func(t *testing.T) {
			body, err := io.ReadAll(c.response().Body)
			require.NoError(t, err)
			assert.NoError(t, validates(t, defs(c.def), body), string(body))
		})
	}

	busy, err := json.Marshal(web.Busy{Operations: []string{"op-1"}})
	require.NoError(t, err)
	assert.NoError(t, validates(t, defs("web.Busy"), busy))
	assert.NoError(t, validates(t, defs("web.Call"), []byte(`{"answers":[{"path":"branches","values":["feat/a"]}],"dry_run":true,"expect_fingerprint":"f"}`)))
	assert.NoError(t, validates(t, defs("web.Call"), []byte(`{}`)))
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
		"field_of_no_kind":   {def: "domain.Form", document: `{"fields":[{"kind":"slider","path":"x"}],"problems":[]}`},
		"another_command":    {def: "create.envelope", document: `{"v":2,"command":"extract","ok":true,"follow_ups":[],"warnings":[]}`},
		"event_of_no_fields": {def: "domain.Event", document: `{"type":"unit.started"}`},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, validates(t, defs(c.def), []byte(c.document)))
		})
	}
}
