package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"go/parser"
	"go/token"
	"maps"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configured returns the main worktree of a new repository configured for wtm, its worktrees under ../.trees.
func configured(t *testing.T) string {
	t.Helper()
	main := filepath.Join(gittest.TempDir(t), "main")
	gittest.Build(t, gittest.FakeParams{Main: main, Branch: "main"})
	stateDir := filepath.Join(main, ".git", "wtm")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "config.toml"), []byte("[worktrees]\nbase_path = \"../.trees\"\n"), 0o644))
	return main
}

// command returns the command of the catalogue called name.
func command(t *testing.T, name string) api.Command {
	t.Helper()
	found, ok := catalogue().Find(name)
	require.True(t, ok, name)
	return found
}

// registeredCommands reads main.go for the commands it registers: each command/<name>/cli it imports, which Go refuses unused.
func registeredCommands(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, parser.ImportsOnly)
	require.NoError(t, err)
	var names []string
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		require.NoError(t, err)
		name, ok := strings.CutPrefix(path, commandPrefix)
		if name, isCLI := strings.CutSuffix(name, "/cli"); ok && isCLI {
			names = append(names, name)
		}
	}
	return names
}

// runs reports whether root runs the command of path, its words: a command of that full path, not its parent taking the rest as arguments.
func runs(root *cobra.Command, path []string) bool {
	found, _, err := root.Find(path)
	return err == nil && found.CommandPath() == strings.Join(append([]string{root.Name()}, path...), " ")
}

// writeConfig appends extra to the config.toml of the repository whose main worktree is main.
func writeConfig(t *testing.T, main, extra string) {
	t.Helper()
	path := filepath.Join(main, ".git", "wtm", "config.toml")
	config, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, append(config, extra...), 0o644))
}

// keepsTheMutationContract drives m through the catalogue alone: its questions, their answers, a dry run, the same plan refused once stale with the replan that follows it up, that replan, then the apply held to its fingerprint.
func keepsTheMutationContract(t *testing.T, m mutation) {
	t.Helper()
	main, c, shape := drive(t, m.name)
	if m.setup != nil {
		m.setup(t, main)
	}

	empty, err := c.Questions(t.Context(), api.Call{Dir: main})
	require.NoError(t, err)
	// a later answer's problem may wait on the subject's, as extract's files wait on their source
	subject := m.answers[0].Path
	assert.NotEmpty(t, domain.ProblemsOf(empty.Problems, subject), "an empty request is not asked its %s", subject)

	asked, err := c.Questions(t.Context(), api.Call{Dir: main, Answers: m.answers})
	require.NoError(t, err)
	require.Empty(t, asked.Problems, "the answers leave a question open")
	shape.golden("questions", asked)
	answers := asked.Answers()

	preview := c.Execute(t.Context(), api.Call{Dir: main, Answers: answers, Options: domain.RunOptions{DryRun: true}})
	require.Equal(t, http.StatusOK, preview.Status, preview.Envelope.Err)
	require.NotEmpty(t, preview.Envelope.Fingerprint)
	assert.NotNil(t, preview.Envelope.Plan)
	assert.Empty(t, preview.Envelope.Items, "a dry run applies nothing")
	shape.golden("dry_run", preview.Envelope)

	m.stale(t, main)
	stale := c.Execute(t.Context(), api.Call{Dir: main, Answers: answers, Options: domain.RunOptions{ExpectFingerprint: preview.Envelope.Fingerprint}})
	assert.Equal(t, http.StatusPreconditionFailed, stale.Status)
	require.NotNil(t, stale.Envelope.Err)
	assert.Equal(t, domain.CodePlanStale, stale.Envelope.Err.Code)
	require.Len(t, stale.Envelope.FollowUps, 1, "a stale plan is followed up by its replan")
	replan := stale.Envelope.FollowUps[0]
	assert.Equal(t, m.name, replan.Command)
	assert.True(t, replan.DryRun)
	assert.Empty(t, stale.Envelope.Items, "a stale plan applies nothing")
	m.untouched(t, main)
	shape.golden("stale", stale.Envelope)

	replanned := c.Execute(t.Context(), api.Call{Dir: main, Answers: replan.Answers, Options: domain.RunOptions{DryRun: replan.DryRun}})
	require.Equal(t, http.StatusOK, replanned.Status, replanned.Envelope.Err)
	assert.NotEqual(t, preview.Envelope.Fingerprint, replanned.Envelope.Fingerprint)

	var events []json.RawMessage
	report := func(event domain.Event) {
		data, err := domain.MarshalEvent(event)
		require.NoError(t, err)
		events = append(events, data)
	}
	applied := c.Execute(t.Context(), api.Call{Dir: main, Answers: replan.Answers, Options: domain.RunOptions{ExpectFingerprint: replanned.Envelope.Fingerprint, Report: report}})
	require.Equal(t, http.StatusOK, applied.Status, applied.Envelope.Err)
	shape.golden("applied", applied.Envelope)
	for _, followUp := range applied.Envelope.FollowUps {
		argv, unspelled := spelled(followUp)
		assert.NotEmpty(t, argv, followUp.Code)
		assert.Empty(t, unspelled, "the command line drops answers of %s", followUp.Code)
	}
	shape.golden("events", events)
	require.NotEmpty(t, applied.Envelope.Items)
	for _, item := range applied.Envelope.Items {
		assert.Equal(t, domain.StatusDone, item.Status, item.Err)
	}
	m.applied(t, main)
}

// keepsTheReadContract drives the read called name through the catalogue alone: its questions, then the read of their answers, with no operation, plan nor fingerprint.
func keepsTheReadContract(t *testing.T, name string) {
	t.Helper()
	main, c, shape := drive(t, name)

	asked, err := c.Questions(t.Context(), api.Call{Dir: main})
	require.NoError(t, err)
	shape.golden("questions", asked)
	read := c.Execute(t.Context(), api.Call{Dir: main, Answers: asked.Answers()})
	shape.golden("read", read.Envelope)

	require.Equal(t, http.StatusOK, read.Status, read.Envelope.Err)
	require.NotEmpty(t, read.Envelope.Items)
	assert.Empty(t, read.Envelope.Op)
	assert.Empty(t, read.Envelope.Fingerprint)
	assert.Nil(t, read.Envelope.Plan)
	assert.NoDirExists(t, filepath.Join(main, ".git", "wtm", "ops"), "a read runs no operation")
}

// drive is the command of the catalogue called name, the main worktree of a new repository configured for wtm it runs on, with a home and a global config of its own, and the shapes it gives, its schema checked first.
func drive(t *testing.T, name string) (string, api.Command, shapes) {
	t.Helper()
	main := configured(t)
	ownConfigDir(t)
	t.Setenv(stateDirEnv, "")
	c := command(t, name)
	shape := shapes{t: t, command: name, root: filepath.Dir(main)}
	shape.golden("form", c.Form)
	return main, c, shape
}

// shapes compares each JSON document a command of the catalogue gives a surface with its golden, testdata/catalogue_<command>_<part>.golden: the published shapes a surface's types are generated from.
type shapes struct {
	t       *testing.T
	command string
	root    string // the directory the test's repository lives in
}

// varying are the values a run gives anew each time: hex ids (operations, fingerprints, commits, short commits) and times.
var varying = regexp.MustCompile(`"[0-9a-f]{7,}"|"\d{4}-\d\d-\d\dT[^"]+"`)

// golden compares v, as JSON, with the golden of part, its repository's directory and the values a run gives anew written as placeholders.
func (s shapes) golden(part string, v any) {
	s.t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	require.NoError(s.t, err)
	text := strings.ReplaceAll(string(data), s.root, "<root>")
	text = varying.ReplaceAllStringFunc(text, func(value string) string {
		if strings.Contains(value, "T") {
			return `"<time>"`
		}
		return `"<id>"`
	})
	formtest.Golden(s.t, "catalogue_"+s.command+"_"+part, text+"\n")
}

// servingBus runs the event bus's daemon on a new socket until the test ends, and returns its socket, in a short directory: a unix socket path is limited to about 100 bytes.
func servingBus(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "wtm-next")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "wtm.sock")

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- bus.Serve(ctx, bus.ServeParams{SocketPath: socket, Idle: time.Minute}) }()
	t.Cleanup(func() {
		cancel()
		<-served
	})
	require.Eventually(t, func() bool {
		conn, err := net.Dial("unix", socket)
		if err == nil {
			_ = conn.Close()
		}
		return err == nil
	}, 2*time.Second, 10*time.Millisecond)
	return socket
}

// watchedLines streams the event stream of stream from dir through the surface's watch until it sent count lines, and returns them with the response.
func watchedLines(t *testing.T, stream api.Stream, call api.Call, count int) ([]string, api.Response) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var lines []string
	response := stream.Watch(ctx, call, func(line json.RawMessage) error {
		lines = append(lines, string(line))
		if len(lines) == count {
			cancel()
		}
		return nil
	})
	return lines, response
}

// withTS is line with its time replaced by x.
func withTS(t *testing.T, line string) string {
	t.Helper()
	var fields map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &fields))
	fields["ts"] = "x"
	data, err := json.Marshal(fields)
	require.NoError(t, err)
	return string(data)
}

// ownConfigDir gives the test a home and a config directory of its own: on Linux, os.UserConfigDir reads $XDG_CONFIG_HOME before $HOME.
func ownConfigDir(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

// The schema the catalogue publishes, which api embeds, and the event stream's, which it refers to.
const (
	apiSchemaID     = "https://wtm.sh/schemas/api.v2.schema.json"
	eventsSchemaID  = "https://wtm.sh/schemas/events.v2.json"
	apiSchemaFile   = "../../internal/api/api.v2.schema.json"
	eventSchemaFile = "../../internal/adapter/events/events.v2.json"
)

// generatedSchema is api.v2.schema.json as the Go types of the catalogue give it: the fragments written by hand beside a MarshalJSON (domain/*.schema.json) as they are, domain.Field as the union of its kinds, the events as the event stream's schema has them, then each command's request, from its form, and its envelope, with its plan and its detail reflected, and each stream's request and the envelope it ends on.
func generatedSchema(t *testing.T) []byte {
	t.Helper()
	s := schemaWriter{t: t, defs: map[string]any{}, types: map[string]reflect.Type{}}
	for _, fragment := range []string{"envelope.schema.json", "form.schema.json"} {
		s.fragment(filepath.Join("..", "..", "internal", "domain", fragment))
	}
	s.fieldUnion()
	s.defs["domain.Event"] = map[string]any{"$ref": "events.v2.json#/$defs/event"}
	s.defs["domain.Line"] = map[string]any{"$ref": "events.v2.json"}
	// the types the fragments name
	for _, named := range []reflect.Type{reflect.TypeFor[domain.Warning](), reflect.TypeFor[domain.FollowUp](), reflect.TypeFor[domain.Section]()} {
		s.of(named)
	}
	for _, command := range catalogue() {
		s.command(command)
	}
	for _, stream := range streams() {
		s.stream(stream)
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	require.NoError(t, encoder.Encode(map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"$id":         apiSchemaID,
		"title":       "wtm api v2",
		"description": "What the catalogue of wtm's commands sends a surface other than the command line: the v2 envelope, the form of a request and its fields, the events of an operation, and each command's request, plan, detail and envelope under $defs. Generated from the Go types by make schema: never edit it by hand.",
		"$defs":       s.defs,
	}))
	return buf.Bytes()
}

// schemaWriter gathers the $defs of the published schema: each named struct once, under its package and its name.
type schemaWriter struct {
	t     *testing.T
	defs  map[string]any
	types map[string]reflect.Type
}

func (s *schemaWriter) fragment(path string) {
	s.t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(s.t, err)
	var doc struct {
		Defs map[string]any `json:"$defs"`
	}
	require.NoError(s.t, json.Unmarshal(data, &doc))
	maps.Copy(s.defs, doc.Defs)
}

// fieldUnion defines domain.Field as one of its kinds, each discriminated by the kind Form.MarshalJSON writes it with.
func (s *schemaWriter) fieldUnion() {
	s.t.Helper()
	var variants []any
	for _, field := range fieldKinds() {
		data, err := json.Marshal(domain.Form{Fields: []domain.Field{field}})
		require.NoError(s.t, err)
		var form struct {
			Fields []map[string]json.RawMessage `json:"fields"`
		}
		require.NoError(s.t, json.Unmarshal(data, &form))
		require.Len(s.t, form.Fields, 1)
		written := form.Fields[0]

		kind, name := reflect.TypeOf(field), defName(reflect.TypeOf(field))
		properties, required := map[string]any{"kind": map[string]any{"const": json.RawMessage(written["kind"])}}, []string{"kind"}
		s.fields(kind, properties, &required)
		// MarshalJSON writes a list it leaves empty as [], never null
		for key, value := range written {
			nullable, _ := properties[key].(map[string]any)
			if alternatives, ok := nullable["anyOf"].([]any); ok && strings.HasPrefix(string(value), "[") {
				properties[key] = alternatives[0]
			}
		}
		s.types[name] = kind
		s.defs[name] = map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}
		variants = append(variants, ref(name))
	}
	s.defs["domain.Field"] = map[string]any{"description": "One field of a form, of the kind its kind names.", "oneOf": variants}
}

// command defines the request of c, the answer of each field of its form by its path, and its envelope, with its plan (none for a read) and its detail.
func (s *schemaWriter) command(c api.Command) {
	s.t.Helper()
	plan := any(false)
	if c.Plan != nil {
		plan = s.of(c.Plan)
	}
	detail := map[string]any{"anyOf": []any{s.of(c.Detail), map[string]any{"type": "object", "maxProperties": 0}}}

	s.defs[c.Name+".request"] = requestSchema(c.Form)
	s.defs[c.Name+".envelope"] = map[string]any{
		"allOf": []any{ref("domain.Envelope")},
		"properties": map[string]any{
			"command": map[string]any{"const": c.Name},
			"plan":    plan,
			"items":   map[string]any{"items": map[string]any{"properties": map[string]any{"detail": detail}}},
		},
	}
}

// stream defines the request of st, the answer of each field of its form by its path, and the envelope it ends on, with no plan nor item: its lines are domain.Line.
func (s *schemaWriter) stream(st api.Stream) {
	s.defs[st.Name+".request"] = requestSchema(st.Form)
	s.defs[st.Name+".envelope"] = map[string]any{
		"allOf":      []any{ref("domain.Envelope")},
		"properties": map[string]any{"command": map[string]any{"const": st.Name}, "plan": false, "items": false},
	}
}

// requestSchema is the request a surface sends for form: the answer of each of its fields by its path, one value for a field that takes one, then the fields to remember and whether to ask again when a field is rememberable.
func requestSchema(form domain.Form) map[string]any {
	var answers []any
	seen := map[string]bool{}
	for _, field := range form.Fields {
		path := field.Spec().Path
		if !seen[path] {
			seen[path] = true
			answers = append(answers, answerSchema(path, valuesSchema(field)))
		}
	}
	if rememberable := form.Rememberable(); len(rememberable) > 0 {
		answers = append(answers,
			answerSchema(domain.PathRemember, map[string]any{"type": "array", "items": map[string]any{"enum": rememberable}}),
			answerSchema(domain.PathAsk, oneValue(map[string]any{"enum": []string{"true", "false"}})))
	}

	return map[string]any{"type": "array", "items": map[string]any{"oneOf": answers}}
}

func answerSchema(path string, values map[string]any) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"path", "values"},
		"properties":           map[string]any{"path": map[string]any{"const": path}, "values": values},
	}
}

func valuesSchema(field domain.Field) map[string]any {
	switch field.(type) {
	case domain.SelectField, domain.TextField:
		return oneValue(map[string]any{"type": "string"})
	case domain.ConfirmField:
		return oneValue(map[string]any{"enum": []string{"true", "false"}})
	case domain.MultiSelectField, domain.TextListField:
	}
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
}

func oneValue(value map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": value, "minItems": 1, "maxItems": 1}
}

var marshaler = reflect.TypeFor[json.Marshaler]()

// of is the schema of a value of type t as encoding/json writes it: a reference to its definition for a named struct, defined on first sight, or to the fragment written by hand beside its MarshalJSON.
func (s *schemaWriter) of(t reflect.Type) map[string]any {
	s.t.Helper()
	if t.Kind() == reflect.Pointer {
		return s.of(t.Elem())
	}
	if t == reflect.TypeFor[time.Time]() {
		return map[string]any{"type": "string", "format": "date-time"}
	}
	if t.Implements(marshaler) || reflect.PointerTo(t).Implements(marshaler) {
		name := defName(t)
		if _, written := s.defs[name]; !written {
			s.t.Fatalf("%s writes its own JSON: its schema is a fragment written by hand beside its MarshalJSON, and there is none", t)
		}
		return ref(name)
	}

	switch t.Kind() {
	case reflect.Interface:
		if t == reflect.TypeFor[domain.Field]() {
			return ref("domain.Field")
		}
		require.Equal(s.t, reflect.TypeFor[any](), t, "an interface the schema cannot describe")
		return map[string]any{}
	case reflect.Struct:
		return s.named(t)
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": s.of(t.Elem())}
	case reflect.Map:
		require.Equal(s.t, reflect.String, t.Key().Kind(), "%s: JSON keys are strings", t)
		return map[string]any{"type": "object", "additionalProperties": s.of(t.Elem())}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]any{"type": "integer"}
	default:
		s.t.Fatalf("%s: a kind the schema cannot describe", t)
		return nil
	}
}

func (s *schemaWriter) named(t reflect.Type) map[string]any {
	s.t.Helper()
	if t.Name() == "" {
		return s.object(t)
	}

	name := defName(t)
	if known, seen := s.types[name]; seen {
		require.Equal(s.t, known, t, "two types are defined as %s", name)
		return ref(name)
	}
	s.types[name] = t
	s.defs[name] = s.object(t)
	return ref(name)
}

// object is a struct as encoding/json writes it, closed: each field by its name, an embedded struct's promoted, a field without omitempty required, and null when its nil is.
func (s *schemaWriter) object(t reflect.Type) map[string]any {
	s.t.Helper()
	properties, required := map[string]any{}, []string{}
	s.fields(t, properties, &required)

	object := map[string]any{"type": "object", "additionalProperties": false, "properties": properties}
	if len(required) > 0 {
		object["required"] = required
	}
	return object
}

func (s *schemaWriter) fields(t reflect.Type, properties map[string]any, required *[]string) {
	s.t.Helper()
	for field := range t.Fields() {
		tag := field.Tag.Get("json")
		tagName, options, _ := strings.Cut(tag, ",")
		name := cmp.Or(tagName, field.Name)
		switch {
		case tag == "-":
			continue
		case field.Anonymous && tagName == "" && field.Type.Kind() == reflect.Struct:
			s.fields(field.Type, properties, required)
			continue
		case field.Anonymous && tagName == "" && field.Type.Kind() == reflect.Pointer && field.Type.Elem().Kind() == reflect.Struct:
			// encoding/json writes none of a nil embedded pointer's fields: each is optional
			s.fields(field.Type.Elem(), properties, new([]string))
			continue
		case !field.IsExported():
			continue
		}

		schema := s.of(field.Type)
		optional := slices.ContainsFunc(strings.Split(options, ","), func(option string) bool { return option == "omitempty" || option == "omitzero" })
		if !optional {
			*required = append(*required, name)
			if nilable(field.Type) {
				schema = map[string]any{"anyOf": []any{schema, map[string]any{"type": "null"}}}
			}
		}
		properties[name] = schema
	}
}

// nilable reports whether encoding/json writes a nil value of t as null.
func nilable(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Interface:
		return true
	default:
		return false
	}
}

// defName names t's definition by its package and its name: create.Plan, domain.Warning.
func defName(t reflect.Type) string {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return path.Base(t.PkgPath()) + "." + t.Name()
}

func ref(name string) map[string]any {
	return map[string]any{"$ref": "#/$defs/" + name}
}

// apiDefs compiles the definitions of the published schema, as api embeds it, by their name.
func apiDefs(t *testing.T) func(name string) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	for id, data := range map[string][]byte{apiSchemaID: api.Schema(), eventsSchemaID: readFile(t, eventSchemaFile)} {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		require.NoError(t, err)
		require.NoError(t, compiler.AddResource(id, doc))
	}

	return func(name string) *jsonschema.Schema {
		t.Helper()
		schema, err := compiler.Compile(apiSchemaID + "#/$defs/" + name)
		require.NoError(t, err, name)
		return schema
	}
}

// validates returns the error of document against schema, nil when it is valid.
func validates(t *testing.T, schema *jsonschema.Schema, document []byte) error {
	t.Helper()
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(document))
	require.NoError(t, err)
	return schema.Validate(value)
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

// updating reports whether the test runs with -update, which rewrites what it compares with.
func updating() bool {
	return flag.Lookup("update").Value.String() == "true"
}

// globbed are the files pattern matches, at least one.
func globbed(t *testing.T, pattern string) []string {
	t.Helper()
	files, err := filepath.Glob(pattern)
	require.NoError(t, err)
	require.NotEmpty(t, files, pattern)
	return files
}

// followUpsIn are the follow-ups a JSON document holds at any depth: each value of a follow_up or of a fix (a problem's), each entry of a follow_ups or of actions (a worktree's).
func followUpsIn(document any) []map[string]any {
	var found []map[string]any
	switch value := document.(type) {
	case map[string]any:
		for key, nested := range value {
			if followUp, ok := nested.(map[string]any); ok && (key == "follow_up" || key == "fix") {
				found = append(found, followUp)
			}
			if list, ok := nested.([]any); ok && (key == "follow_ups" || key == "actions") {
				for _, entry := range list {
					if followUp, ok := entry.(map[string]any); ok {
						found = append(found, followUp)
					}
				}
			}
			found = append(found, followUpsIn(nested)...)
		}
	case []any:
		for _, nested := range value {
			found = append(found, followUpsIn(nested)...)
		}
	}
	return found
}
