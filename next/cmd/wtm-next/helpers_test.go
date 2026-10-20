package main

import (
	"context"
	"encoding/json"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
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
