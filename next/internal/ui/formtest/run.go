package formtest

import (
	"context"

	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/spf13/cobra"
)

// Printed is what a command line printed on each stream and the code it exited with.
type Printed struct {
	Stdout string
	Stderr string
	Code   int
}

// Run runs args on the root of version 1.2.3 holding the command that command builds over test streams, off a terminal unless command sets the streams up otherwise before it returns.
func Run(ctx context.Context, command func(streams *output.Streams) *cobra.Command, args ...string) Printed {
	streams, _, stdout, stderr := output.Test()
	root := uicli.NewRoot("1.2.3", streams, []*cobra.Command{command(streams)})
	code := uicli.Execute(ctx, root, args)
	return Printed{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}
