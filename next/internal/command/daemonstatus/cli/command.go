// Package cli is daemon status's command line: the daemon drawn as text, its JSON, over daemon status's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstatus"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	statusShort = "Show whether wtm's daemon runs, its build and the jobs it holds"
	statusLong  = "Read wtm's daemon, the one process of this machine that runs the jobs of every\n" +
		"repository, serves their named addresses and carries the event stream: whether\n" +
		"it runs and its pid, its build against this wtm's, the jobs it holds by repository\n" +
		"and worktree, the port of its proxy, its socket and its index of jobs.\n\n" +
		"With none running, the stacks and the holds on shared services its index keeps,\n" +
		"which run on without it. It changes nothing and never starts the daemon."
	statusExample = `  wtm daemon status

  wtm daemon status --output json`
)

// query is daemon status's form: a read.
type query = form.Query[daemonstatus.Request, daemonstatus.Questions, daemonstatus.Outcome]

// statusFlags spells the failures: daemon status has no field.
var statusFlags = (&query{Words: words}).Flags()

// New returns daemon status's command, a subcommand of daemon: it reads the daemon and draws it on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (daemonstatus.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	run := &uicli.Run{Command: daemonstatus.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: statusFlags}
	q := &query{
		Words: words,
		Open: func(ctx context.Context, dir string) (form.Reader[daemonstatus.Request, daemonstatus.Questions, daemonstatus.Outcome], error) {
			return open(ctx, dir)
		},
		Draw: drawStatus,
	}
	cmd := &cobra.Command{Use: "status", Short: statusShort, Long: statusLong, Example: statusExample, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return q.Launch(cmd, args)
	}}
	q.Bind(cmd, run)

	return cmd
}
