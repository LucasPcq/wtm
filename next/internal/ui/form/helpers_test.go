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

// boundLines is the fake command that runs a line, its flags bound on its cobra command, which parsed args.
func boundLines(t *testing.T, args ...string) (*form.Command[request, lines, outcome], *cobra.Command) {
	t.Helper()
	command, cmd := linesCommand(), &cobra.Command{Use: "run"}
	command.Bind(cmd, &uicli.Run{})
	require.NoError(t, cmd.ParseFlags(args))
	return command, cmd
}

// concluded is how the fake command concludes ran and err, with the given --output, its stdout and the failure it returns.
func concluded(t *testing.T, format string, ran form.Ran[request, outcome], err error) (string, error) {
	t.Helper()
	streams, _, stdout, _ := output.Test()
	command, cmd := fakeCommand(), &cobra.Command{Use: "fake"}
	command.Draw = func(w io.Writer, applied outcome, view output.View) {
		_, _ = io.WriteString(w, "drew "+strings.Join(applied.Plan["branches"], ", ")+" from "+view.Cwd+"\n")
	}
	run := &uicli.Run{Command: "fake", Streams: streams, Env: uicli.Env{Cwd: "/repo"}, CommandLine: formtest.CommandLine}
	command.Bind(cmd, run)
	run.Output = format

	failure := command.Conclude(cmd, ran, err)
	return stdout.String(), failure
}

// refused is how a text run of ran ending on err is refused, quiet or not: what it prints on stderr, and the next steps the quiet line keeps.
func refused(t *testing.T, quiet bool, ran form.Ran[request, outcome], err error) (string, []output.NextStep) {
	t.Helper()
	streams, _, _, stderr := output.Test()
	command, cmd := fakeCommand(), &cobra.Command{Use: "fake"}
	run := &uicli.Run{Command: "fake", Streams: streams, Output: uicli.OutputText, Quiet: quiet, CommandLine: formtest.CommandLine}
	command.Bind(cmd, run)

	var failed *uicli.Failed
	require.ErrorAs(t, command.Conclude(cmd, ran, err), &failed)
	return stderr.String(), failed.Next
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

// launchedSubject launches q bound to run with args, as the command line would, its positional argument the subject.
func launchedSubject(t *testing.T, q *form.Query[request, subjects, read], run *uicli.Run, args ...string) error {
	t.Helper()
	cmd := &cobra.Command{Use: "fake"}
	q.Bind(cmd, run)
	cmd.Flags().Bool(uicli.FlagQuiet, false, "")
	require.NoError(t, cmd.ParseFlags(args))
	cmd.SetContext(t.Context())
	return q.Launch(cmd, cmd.Flags().Args())
}

// quietlyFailed is the failure a quiet text run of the fake command prints alone once it applied, failing at subject on message.
func quietlyFailed(t *testing.T, subject, message string) string {
	t.Helper()
	streams, _, _, _ := output.Test()
	command, cmd := fakeCommand(), &cobra.Command{Use: "fake"}
	command.Draw = func(io.Writer, outcome, output.View) {}
	run := &uicli.Run{Command: "fake", Streams: streams, Output: uicli.OutputText, Quiet: true, CommandLine: formtest.CommandLine}
	command.Bind(cmd, run)
	applied := outcome{Fingerprint: "print", Result: &domain.Result[struct{}]{Items: []domain.Unit[struct{}]{
		{Subject: "feat/a", Status: domain.StatusDone},
		{Subject: subject, Status: domain.StatusFailed, Err: &domain.Error{Kind: domain.KindInternal, Code: "fake.failed", Message: message}},
	}}}

	var failed *uicli.Failed
	require.ErrorAs(t, command.Conclude(cmd, form.Ran[request, outcome]{Outcome: applied}, nil), &failed)
	return failed.Err.Message
}

// trailLine is the line of screen that starts with prefix.
func trailLine(t *testing.T, screen, prefix string) string {
	t.Helper()
	for line := range strings.Lines(screen) {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSuffix(line, "\n")
		}
	}
	require.Failf(t, "no trail line", "no line starts with %q in:\n%s", prefix, screen)
	return ""
}
