package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
	"github.com/LucasPcq/wtm/internal/testutil/dispatchtest"
)

// The few helpers the tests share. Everything a test file holds is a test.

// world is the example served on a command line of its own: the repository,
// the bench dispatch runs on, the engine binding them.
type world struct {
	Repo   *repo
	Bench  *dispatchtest.Bench
	Engine *Engine
	detail Option
}

type worldParams struct {
	Form    Form
	Confirm Confirm
	// Detail is the example command's own rendering; none declares the
	// generic one.
	Detail render.Detail
}

func newWorld(t testing.TB, params worldParams) *world {
	t.Helper()
	r := &repo{}
	b := dispatchtest.New(t, dispatchtest.BenchParams{})
	engine := NewEngine(EngineParams{
		Registry: func(d *dispatch.Dispatcher) dispatch.Registry {
			return dispatch.Registry{
				dispatch.Register(dispatch.RegisterParams[request, facts, plan, string]{Dispatcher: d, Command: makeCommand(r), Messages: makeMessages, ExitCodes: makeExitCodes}),
				dispatch.RegisterQuery(dispatch.RegisterQueryParams[struct{}, inventory]{Query: inventoryQuery(r)}),
			}
		},
		Dispatcher: func(context.Context) (*dispatch.Dispatcher, error) { return b.Dispatcher, nil },
		Form:       params.Form,
		Confirm:    params.Confirm,
	})
	return &world{Repo: r, Bench: b, Engine: engine, detail: detailOption(params.Detail)}
}

func detailOption(detail render.Detail) Option {
	if detail == nil {
		return GenericDetail()
	}
	return Detail(detail)
}

// root is the tree the commands of the example hang from, with the hooks and
// the flags of the real one.
func (w *world) root() *cobra.Command {
	root := &cobra.Command{
		Use:                domain.AppName,
		PersistentPreRunE:  rootCmd.PersistentPreRunE,
		PersistentPostRunE: rootCmd.PersistentPostRunE,
		SilenceErrors:      true,
		SilenceUsage:       true,
	}
	root.PersistentFlags().BoolP(domain.FlagQuiet, "q", false, "")
	root.AddCommand(
		w.Engine.Bind(commandName, Doc{Use: commandName + " [branch...]", Short: "Make worktrees"},
			Positional("branches"), Flag(FlagParams{Path: "fast_forward", Name: "ff"}), w.detail),
		w.Engine.Bind(queryName, Doc{Use: queryName, Short: "List the worktrees"}, GenericDetail()),
	)
	markUsageErrors(root)
	return root
}

type ran struct {
	Stdout string
	Stderr string
	Code   int
}

type runParams struct {
	Args []string
	// TTY says stdin is a terminal.
	TTY bool
	Ctx context.Context
}

// run executes the command line as the binary does, down to its exit code.
func (w *world) run(t *testing.T, params runParams) ran {
	t.Helper()
	stdinWas := shared.StdinIsTerminal
	shared.StdinIsTerminal = func() bool { return params.TTY }
	t.Cleanup(func() {
		shared.StdinIsTerminal = stdinWas
		humanOutputSilenced = false
	})
	ctx := params.Ctx
	if ctx == nil {
		ctx = t.Context()
	}
	root := w.root()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(params.Args)
	code := execute(ctx, executeParams{Root: root, Stderr: &stderr})
	return ran{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}

func args(words ...string) runParams {
	return runParams{Args: words}
}

// envelope is a run's JSON document, decoded loosely so a test reads keys the
// way an agent does.
type envelope struct {
	V           int               `json:"v"`
	Command     string            `json:"command"`
	OK          bool              `json:"ok"`
	Op          string            `json:"op"`
	Items       []json.RawMessage `json:"items"`
	Plan        json.RawMessage   `json:"plan"`
	Fingerprint string            `json:"fingerprint"`
	View        json.RawMessage   `json:"view"`
	FollowUps   []json.RawMessage `json:"follow_ups"`
	Warnings    []json.RawMessage `json:"warnings"`
	Error       *struct {
		Kind    string `json:"kind"`
		Code    string `json:"code"`
		Message string `json:"message"`
		Fields  []struct {
			Path    string `json:"path"`
			Flag    string `json:"flag"`
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"fields"`
	} `json:"error"`
}

func decode(t *testing.T, stdout string) envelope {
	t.Helper()
	var doc envelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc), stdout)
	return doc
}

// binding is the example command bound alone, to read what Bind generated.
func (w *world) binding(t *testing.T, options ...Option) (*cobra.Command, *binding) {
	t.Helper()
	cmd := w.Engine.Bind(commandName, Doc{Use: commandName}, options...)
	return cmd, w.Engine.bindings[commandName]
}

// stripGlyph reads a failure line without the glyph that opens it.
func stripGlyph(line string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), domain.GlyphFailure))
}

// fieldsOf is what an invalid request names.
func fieldsOf(t *testing.T, err kernel.Error) []kernel.FieldError {
	t.Helper()
	invalid, ok := err.(*kernel.InvalidError)
	require.True(t, ok, "got %T", err)
	return invalid.Fields
}

// applyingForm is a form that asks nothing: it runs the request it is given,
// keeping each one.
func applyingForm(asked *[]string) Form {
	return func(ctx context.Context, params FormParams) (dispatch.Reply, error) {
		*asked = append(*asked, string(params.Request))
		return params.Entry.Call(ctx, dispatch.Call{Request: params.Request, Options: params.Options, Progress: params.Progress})
	}
}

// answering confirms the follow-ups with answers in turn, then declines.
func answering(offered *[]string, answers ...bool) Confirm {
	return func(params ConfirmParams) (bool, error) {
		*offered = append(*offered, params.Command)
		if len(*offered) > len(answers) {
			return false, nil
		}
		return answers[len(*offered)-1], nil
	}
}
