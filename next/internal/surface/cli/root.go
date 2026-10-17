// Package cli is the command line of wtm-next: the cobra root, its commands and their exit codes.
package cli

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/surface/output"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/spf13/cobra"
)

const (
	flagQuiet  = "quiet"
	quietUsage = "Silence human output; errors and the exit code are unaffected, and --output json still emits its document"
)

type Params struct {
	Version       string
	Args          []string
	Open          func(ctx context.Context) (Project, error)
	KillCancelled func()
	AwaitShields  func()
}

type Project struct {
	App *app.App
	Cwd string
}

func Main(params Params) int {
	ctx, stop := interruptible(params)
	defer stop()
	root := NewRoot(params)
	root.SetArgs(params.Args)
	code := Execute(ctx, root)
	params.KillCancelled()
	params.AwaitShields()
	return code
}

func NewRoot(params Params) *cobra.Command {
	root := &cobra.Command{
		Use:           "wtm-next",
		Short:         "Git worktree manager, rebuilt as slices",
		Version:       params.Version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.PersistentFlags().BoolP(flagQuiet, "q", false, quietUsage)
	help := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		out := cmd.OutOrStdout()
		var text bytes.Buffer
		cmd.SetOut(&text)
		help(cmd, args)
		cmd.SetOut(out)
		output.Help(out, text.String())
	})
	root.AddCommand(newCreate(params.Open))
	return root
}

func Execute(ctx context.Context, root *cobra.Command) int {
	_, err := root.ExecuteContextC(ctx)
	if err == nil {
		return exitOK
	}
	stderr := root.ErrOrStderr()
	if failure, ok := errors.AsType[*failed](err); ok {
		if failure.Quiet {
			output.Frame(stderr, func(w io.Writer) { output.QuietFailure(w, failure.Err) })
		}
		return exitCode(exitParams{Failure: failure.Err, Signalled: ctx.Err() != nil})
	}
	usage := &wtm.Error{Kind: wtm.KindInvalid, Code: wtm.CodeInvalidRequest, Message: err.Error()}
	output.Frame(stderr, func(w io.Writer) { output.Failure(w, output.FailureParams{Err: usage}) })
	return exitUsage
}

type failed struct {
	Err   *wtm.Error
	Quiet bool
}

func (f *failed) Error() string { return f.Err.Error() }
