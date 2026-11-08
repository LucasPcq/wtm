// Package cli is run ps' command line: its flag, the jobs of the machine drawn group by group, and its JSON, over run ps' use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/runps"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// Use is how the command line spells run ps.
const Use = "run ps"

const (
	psShort = "List the jobs running on this machine"
	psLong  = "List the jobs wtm's daemon runs, in every repository of the machine (--repo\n" +
		"narrows to one): a group per worktree with the profile its jobs were started\n" +
		"under, what crashed first, then what runs, then what ended or never started; a\n" +
		"runner with the services it starts, and each repository's shared services with\n" +
		"the worktrees holding them. It works from any directory, changes nothing and\n" +
		"never starts the daemon."
	psExample = `  wtm run ps

  # Only this repository's jobs, as JSON
  wtm run ps --repo . --output json`
)

// query is run ps' form: a read.
type query = form.Query[runps.Request, runps.Questions, runps.Outcome]

// psFlags spells the failures: run ps' only field is a flag.
var psFlags = (&query{Words: words}).Flags()

// New returns the run ps command: its flag becomes a runps.Request, read and drawn on streams, or the v2 JSON envelope with --output json.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (runps.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	run := &uicli.Run{Command: runps.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: psFlags}
	q := &query{
		Words: words,
		Open: func(ctx context.Context, dir string) (form.Reader[runps.Request, runps.Questions, runps.Outcome], error) {
			return open(ctx, dir)
		},
		Draw: drawPs,
	}
	cmd := &cobra.Command{Use: "ps", Short: psShort, Long: psLong, Example: psExample, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return q.Launch(cmd, args)
	}}
	q.Bind(cmd, run)

	return cmd
}
