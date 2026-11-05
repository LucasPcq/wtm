package main

import (
	"encoding/json"
	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
	published := map[reflect.Type]any{reflect.TypeFor[domain.Phase](): domain.Phases(), reflect.TypeFor[domain.Surface](): domain.Surfaces()}
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
			// the envelope names its command, which a folder spells without its dash (fast-forward's is fastforward)
			var envelope struct{ Command string }
			require.NoError(t, json.Unmarshal(readFile(t, golden), &envelope))
			assert.NoError(t, validates(t, defs(envelope.Command+".envelope"), readFile(t, golden)))
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

func TestEachLineOfAJobsStreamMatchesTheSchema(t *testing.T) {
	defs := apiDefs(t)
	at := time.Date(2026, 11, 3, 20, 17, 21, 0, time.UTC)

	for name, v := range map[string]any{
		"line":    runlogs.Entry{Worktree: "feat/a", Job: "api", At: at, Text: "ready", N: 3},
		"undated": runlogs.Entry{Worktree: "feat/a", Job: "api", Text: "no stamp"},
		"absent":  runlogs.Absent{Worktree: "feat/a"},
		"never":   runlogs.Absent{Worktree: "feat/a", Job: "never"},
		"ended":   runlogs.Ended{Worktree: "feat/a", Job: "api", State: runlogs.JobCrashed, ExitCode: new(1)},
		"warning": domain.Warning{Code: "run-logs.daemon_skew", Message: "of another build", Params: domain.Params{"skew": "build"}, FollowUp: &domain.FollowUp{Command: "run daemon restart", Code: "run-logs.restart_daemon", Message: "restart it"}},
		"failure": &domain.Error{Kind: domain.KindInternal, Code: "run-logs.broken", Message: "the daemon went away"},
	} {
		t.Run(name, func(t *testing.T) {
			line, err := runlogs.JSONOf(v)
			require.NoError(t, err)
			assert.NoError(t, validates(t, defs("web.JobLine"), line), string(line))
		})
	}
}

func TestTheLinesAndTheLogOfAJobAreServedByTheWebServer(t *testing.T) {
	main := configured(t)
	ownConfigDir(t)
	writeRun(t, main, seedJob)
	logs := filepath.Join(main, ".git", "wtm", "logs", "main")
	require.NoError(t, os.MkdirAll(logs, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(logs, "seed.log"), []byte("2026-11-03T20:17:21Z  seeded\n2026-11-03T20:17:22Z  done\n"), 0o600))
	server, _ := webServing(t, version)
	repo := registered(t, main)

	lines := get(t, server, "/api/repos/"+repo+"/jobs/seed/logs?worktree=main&follow=false", false)
	file := get(t, server, "/api/repos/"+repo+"/jobs/seed/log?worktree=main", false)

	require.Equal(t, http.StatusOK, lines.StatusCode)
	body, err := io.ReadAll(lines.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `data: {"type":"line","worktree":"main","job":"seed","at":"2026-11-03T20:17:21Z","text":"seeded"}`)
	require.Equal(t, http.StatusOK, file.StatusCode)
	saved, err := io.ReadAll(file.Body)
	require.NoError(t, err)
	assert.Equal(t, "2026-11-03T20:17:21Z  seeded\n2026-11-03T20:17:22Z  done\n", string(saved))
}
