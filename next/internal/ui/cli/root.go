// Package cli is the root of wtm-next's command line: version, --quiet, help, signals and the exit code of the commands registered on it.
package cli

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/spf13/cobra"
)

const (
	FlagQuiet  = "quiet"
	quietUsage = "Silence human output; errors and the exit code are unaffected, and --output json still emits its document"
)

type Params struct {
	Version       string
	Streams       *output.Streams
	Args          []string
	Commands      []*cobra.Command
	KillCancelled func()
	AwaitShields  func()
}

func Main(params Params) int {
	ctx, stop := interruptible(params)
	defer stop()
	root := NewRoot(params.Version, params.Streams, params.Commands)
	root.SetArgs(params.Args)
	code := Execute(ctx, root)
	params.KillCancelled()
	params.AwaitShields()
	return code
}

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

func Execute(ctx context.Context, root *cobra.Command) int {
	failing, err := root.ExecuteContextC(ctx)
	if err == nil {
		return ExitOK
	}
	stderr := root.ErrOrStderr()
	if failure, ok := errors.AsType[*Failed](err); ok {
		if failure.Quiet {
			output.Frame(stderr, func(w io.Writer) {
				output.QuietFailure(w, output.FailureParams{Err: failure.Err, Flags: failure.Flags})
			})
		}
		return exitCode(failure, ctx.Err() != nil)
	}
	usage := domain.InvalidMessage(err.Error())
	output.Frame(stderr, func(w io.Writer) {
		output.Failure(w, output.FailureParams{Err: usage, Next: []output.NextStep{HelpStep(failing)}})
	})
	return ExitUsage
}

func HelpStep(cmd *cobra.Command) output.NextStep {
	return output.NextStep{Command: cmd.CommandPath() + " --help"}
}

type Failed struct {
	Err   *domain.Error
	Quiet bool
	Flags output.Flags
	Exits map[string]int
}

func (f *Failed) Error() string { return f.Err.Error() }
