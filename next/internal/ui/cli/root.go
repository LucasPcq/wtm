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
	Args          []string
	Commands      []*cobra.Command
	KillCancelled func()
	AwaitShields  func()
}

func Main(params Params) int {
	ctx, stop := interruptible(params)
	defer stop()
	root := NewRoot(params.Version, params.Commands)
	root.SetArgs(params.Args)
	code := Execute(ctx, root)
	params.KillCancelled()
	params.AwaitShields()
	return code
}

func NewRoot(version string, commands []*cobra.Command) *cobra.Command {
	root := &cobra.Command{
		Use:           "wtm-next",
		Short:         "Git worktree manager, rebuilt as slices",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
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
	_, err := root.ExecuteContextC(ctx)
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
	output.Frame(stderr, func(w io.Writer) { output.Failure(w, output.FailureParams{Err: usage}) })
	return ExitUsage
}

type Failed struct {
	Err   *domain.Error
	Quiet bool
	Flags output.Flags
	Exits map[string]int
}

func (f *Failed) Error() string { return f.Err.Error() }
