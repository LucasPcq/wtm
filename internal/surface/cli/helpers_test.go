package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
	"github.com/LucasPcq/wtm/internal/testutil/dispatchtest"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

type world struct {
	Repo   *repo
	Bench  *dispatchtest.Bench
	Engine *Engine
	detail Option
}

type worldParams struct {
	Form    Form
	Confirm Confirm
	Detail  render.Detail
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
	TTY  bool
	Ctx  context.Context
}

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

// Decoded loosely, so a test reads the keys the way an agent does.
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

func (w *world) binding(t *testing.T, options ...Option) (*cobra.Command, *binding) {
	t.Helper()
	cmd := w.Engine.Bind(commandName, Doc{Use: commandName}, options...)
	return cmd, w.Engine.bindings[commandName]
}

func stripGlyph(line string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), domain.GlyphFailure))
}

func fieldsOf(t *testing.T, err kernel.Error) []kernel.FieldError {
	t.Helper()
	invalid, ok := err.(*kernel.InvalidError)
	require.True(t, ok, "got %T", err)
	return invalid.Fields
}

func applyingForm(asked *[]string) Form {
	return func(ctx context.Context, params FormParams) (dispatch.Reply, error) {
		*asked = append(*asked, string(params.Request))
		return params.Entry.Call(ctx, dispatch.Call{Request: params.Request, Options: params.Options, Progress: params.Progress})
	}
}

func answering(offered *[]string, answers ...bool) Confirm {
	return func(params ConfirmParams) (bool, error) {
		*offered = append(*offered, params.Command)
		if len(*offered) > len(answers) {
			return false, nil
		}
		return answers[len(*offered)-1], nil
	}
}

func quietTestCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return nil }}
	cmd.Flags().Bool(domain.FlagQuiet, false, "")
	cmd.Flags().String(domain.FlagOutput, domain.OutputText, "")
	if err := cmd.Flags().Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

func silenced(cmd *cobra.Command) bool {
	return cmd.OutOrStdout() == io.Discard && cmd.ErrOrStderr() == io.Discard
}

const shieldHelperEnv = "WTM_TEST_SHIELD_HELPER"

type interruptHelper struct {
	cmd   *exec.Cmd
	lines *bufio.Scanner
}

func startHelper(t *testing.T, test string) interruptHelper {
	t.Helper()
	helper := exec.Command(os.Args[0], "-test.run=^"+test+"$")
	helper.Env = append(os.Environ(), shieldHelperEnv+"=1")
	stdout, err := helper.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = helper.Process.Kill() })
	return interruptHelper{cmd: helper, lines: bufio.NewScanner(stdout)}
}

func (h interruptHelper) expect(t *testing.T, want string) string {
	t.Helper()
	if !h.lines.Scan() {
		t.Fatalf("helper ended before saying %q", want)
	}
	if want != "" && h.lines.Text() != want {
		t.Fatalf("helper said %q, want %q", h.lines.Text(), want)
	}
	return h.lines.Text()
}

func (h interruptHelper) diesOnSIGINT(t *testing.T) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- h.cmd.Wait() }()
	select {
	case err := <-done:
		var exitErr *exec.ExitError
		status, ok := syscall.WaitStatus(0), false
		if errors.As(err, &exitErr) {
			status, ok = exitErr.Sys().(syscall.WaitStatus)
		}
		if !ok || !status.Signaled() || status.Signal() != syscall.SIGINT {
			t.Fatalf("helper ended with %v, want SIGINT", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the second interrupt did not end the run")
	}
}

type exclusivePair struct {
	cmd  *cobra.Command
	args []string
}

func exclusivePairs(cmd *cobra.Command) []exclusivePair {
	var pairs []exclusivePair
	seen := map[string]bool{}
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		for _, group := range f.Annotations[domain.AnnotationMutuallyExclusive] {
			if seen[group] {
				continue
			}
			seen[group] = true
			names := strings.Fields(group)
			for i, first := range names {
				for _, second := range names[i+1:] {
					path := strings.Fields(cmd.CommandPath())[1:]
					args := append(path, flagArg(cmd, first), flagArg(cmd, second))
					pairs = append(pairs, exclusivePair{cmd: cmd, args: args})
				}
			}
		}
	})
	for _, sub := range cmd.Commands() {
		pairs = append(pairs, exclusivePairs(sub)...)
	}
	return pairs
}

func flagArg(cmd *cobra.Command, name string) string {
	if cmd.Flags().Lookup(name).Value.Type() == "bool" {
		return "--" + name
	}
	return "--" + name + "=x"
}

func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
}

func walk(cmd *cobra.Command, visit func(*cobra.Command)) {
	visit(cmd)
	for _, sub := range cmd.Commands() {
		walk(sub, visit)
	}
}

const interruptHelperEnv = "WTM_TEST_INTERRUPT_HELPER"
