package form_test

import (
	"io"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

var size = tea.WindowSizeMsg{Width: 80, Height: 30}

func open(t *testing.T, script form.Script) *formtest.Driver {
	t.Helper()
	return start(form.New(t.Context(), script))
}

func dryRun(script form.Script) form.Script {
	script.DryRun = true
	return script
}

func start(w *form.Wizard) *formtest.Driver {
	return formtest.Start(w, size)
}

// asking opens the wizard of the fake command on the step at, every step before it given.
func asking(t *testing.T, a *app, given request) (*formtest.Driver, *form.Session[request, questions, outcome]) {
	t.Helper()
	s := fakeCommand().Session(t.Context(), a, given, domain.RunOptions{})
	return start(form.New(t.Context(), s.Script())), s
}

func bound(t *testing.T, args ...string) (*form.Command[request, questions, outcome], *cobra.Command) {
	t.Helper()
	command, cmd := fakeCommand(), &cobra.Command{Use: "fake"}
	command.Bind(cmd, &uicli.Run{})
	require.NoError(t, cmd.ParseFlags(args))
	return command, cmd
}

// concluded is how the fake command concludes ran and err, with the given --output, its stdout and the failure it returns.
func concluded(t *testing.T, format string, ran form.Ran[request, outcome], err error) (string, error) {
	t.Helper()
	streams, _, stdout, _ := output.Test()
	command, cmd := fakeCommand(), &cobra.Command{Use: "fake"}
	command.Draw = func(w io.Writer, applied outcome, env uicli.Env) {
		_, _ = io.WriteString(w, "drew "+strings.Join(applied.Plan["branches"], ", ")+" from "+env.Cwd+"\n")
	}
	run := &uicli.Run{Command: "fake", Streams: streams, Env: uicli.Env{Cwd: "/repo", CommandLine: formtest.CommandLine}}
	command.Bind(cmd, run)
	run.Output = format

	failure := command.Conclude(cmd, ran, err)
	return stdout.String(), failure
}

// queryCommand is a cobra command q is bound to, printing with run.
func queryCommand(q *form.Query[request, questions, read], run *uicli.Run) *cobra.Command {
	cmd := &cobra.Command{Use: "fake"}
	q.Bind(cmd, run)
	return cmd
}

// launched launches q bound to run with args, as the command line would.
func launched(t *testing.T, q *form.Query[request, questions, read], run *uicli.Run, args ...string) error {
	t.Helper()
	cmd := queryCommand(q, run)
	cmd.Flags().Bool(uicli.FlagQuiet, false, "")
	require.NoError(t, cmd.ParseFlags(args))
	cmd.SetContext(t.Context())
	return q.Launch(cmd, nil)
}
