// Package cli is the root of wtm-next's command line: version, --quiet, help, signals and the exit code of the commands registered on it.
package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/spf13/cobra"
)

// FlagQuiet is the name of the persistent flag that silences human output; a command reads it to decide what it prints.
const (
	FlagQuiet  = "quiet"
	quietUsage = "Silence human output; errors and the exit code are unaffected, and --output json still emits its document"
)

// Params is what [Main] runs the command line with.
type Params struct {
	Version  string
	Streams  *output.Streams
	Args     []string // the arguments after the program's name
	Commands []*cobra.Command
	// KillCancelled kills the child processes an interrupted operation is still stopping.
	KillCancelled func()
	// AwaitShields blocks until every shielded section, which an interruption lets finish, has finished.
	AwaitShields func()
}

// Main runs the command line once with signals handled and returns the process's exit code.
// It returns only once the children are killed and the shielded sections are finished, so the process may exit right after.
func Main(params Params) int {
	ctx, stop := interruptible(params)
	defer stop()

	code := Execute(ctx, NewRoot(params.Version, params.Streams, params.Commands), params.Args)

	params.KillCancelled()
	params.AwaitShields()
	return code
}

// NewRoot returns the root command over streams, with --quiet, the framed help and commands registered on it.
// It prints no error nor usage of its own: [Execute] renders them.
func NewRoot(version string, streams *output.Streams, commands []*cobra.Command) *cobra.Command {
	root := &cobra.Command{
		Use:           "wtm-next",
		Short:         "Git worktree manager, rebuilt as slices",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetIn(streams.In)
	root.SetOut(streams.Out)
	root.SetErr(streams.Err)
	root.PersistentFlags().BoolP(FlagQuiet, "q", false, quietUsage)

	help := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		out := cmd.OutOrStdout()
		var text bytes.Buffer
		cmd.SetOut(&text)
		help(cmd, args)
		cmd.SetOut(out)
		output.Help(out, text.String())
	})

	root.AddCommand(commands...)
	return root
}

// Execute runs root with args under ctx and returns the exit code of its outcome.
// A [*Failed] has been rendered by its command unless it is quiet, in which case Execute prints its one line; any other error is a usage error, in the envelope when args ask for JSON, else printed with the failing command's help as the next step.
func Execute(ctx context.Context, root *cobra.Command, args []string) int {
	root.SetArgs(args)
	failing, err := root.ExecuteContextC(ctx)
	if err == nil {
		return ExitOK
	}

	stderr := root.ErrOrStderr()
	if failure, ok := errors.AsType[*Failed](err); ok {
		if failure.Quiet {
			output.Frame(stderr, func(w io.Writer) {
				output.QuietFailure(w, output.FailureParams{Err: failure.Err, Next: failure.Next, Flags: failure.Flags})
			})
		}
		return exitCode(failure, ctx.Err() != nil)
	}

	usage := domain.Invalid(nil)
	usage.Cause = err
	if asksJSON(args) {
		if err := output.Envelope(root.OutOrStdout(), domain.Envelope{Command: failing.Name(), Err: usage}, nil); err != nil {
			return ExitError
		}
		return ExitUsage
	}

	output.Frame(stderr, func(w io.Writer) {
		output.Failure(w, output.FailureParams{Err: usage, Next: []output.NextStep{HelpStep(failing)}})
	})
	return ExitUsage
}

// asksJSON reports whether args ask for the JSON envelope, read without cobra since a usage error may stop its parsing before --output: the last --output before "--" says exactly json.
func asksJSON(args []string) bool {
	format := ""
	for i, arg := range args {
		if arg == "--" {
			break
		}
		if value, ok := strings.CutPrefix(arg, "--output="); ok {
			format = value
		}
		if arg == "--output" && i+1 < len(args) {
			format = args[i+1]
		}
	}

	return format == OutputJSON
}

// HelpStep returns the next step that shows cmd's help.
func HelpStep(cmd *cobra.Command) output.NextStep {
	return output.NextStep{Command: cmd.CommandPath() + " --help"}
}

// Failed is the error a command returns once its failure is settled, carrying what the root needs to print it and pick its exit code.
type Failed struct {
	Err *domain.Error
	// Quiet reports whether the command printed nothing under --quiet, leaving the root to print the failure's one line.
	Quiet bool
	// Next is the way out of Err, its follow-up spelled, which --quiet still prints.
	Next []output.NextStep
	// Flags spells each field path of Err as the flag that sets it.
	Flags output.Flags
	// Exits maps the command's own error codes to their exit codes, ahead of those every command shares.
	Exits map[string]int
}

// Error returns the message of the failure.
func (f *Failed) Error() string { return f.Err.Error() }
