// Package cli is run list's command line: its worktree, its table, its JSON, and the picker that ends on what to do with a profile or a job, over run list's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/runlist"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// Use is how the command line spells run list.
const Use = "run list"

const (
	listShort = "List the jobs and profiles of run.toml"
	listLong  = "List the profiles and the jobs run.toml declares, as a worktree runs them: each\n" +
		"job's state there, the ports it binds there, its folder and its command. It\n" +
		"changes nothing and never starts wtm's daemon.\n\n" +
		"On a terminal it opens a picker: choose a profile or a job, then what to do with\n" +
		"it, and the command that does it is printed. With no worktree named, a terminal\n" +
		"asks which one, the one you stand in first; anywhere else it is that one."
	yesUsage    = "Ask nothing: list the worktree you stand in, with no picker"
	listExample = `  # Pick a profile or a job, then what to do with it
  wtm run list

  # feat/a's jobs and profiles, as JSON
  wtm run list feat/a --output json`
)

// query is run list's form: a read.
type query = form.Query[runlist.Request, runlist.Questions, runlist.Outcome]

// listFlags spells the failures.
var listFlags = (&query{Words: words}).Flags()

// New returns the run list command: its worktree becomes a runlist.Request, read, then picked from when the user can see and answer it; it draws the profiles and jobs on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (runlist.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	run := &uicli.Run{Command: runlist.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: listFlags}
	q := &query{
		Words: words,
		Open: func(ctx context.Context, dir string) (form.Reader[runlist.Request, runlist.Questions, runlist.Outcome], error) {
			return open(ctx, dir)
		},
		Draw:     drawList,
		YesUsage: yesUsage,
	}
	// --yes asks nothing: no worktree, and no picker.
	q.Pick = func(ctx context.Context, read runlist.Outcome, req runlist.Request, run *uicli.Run) (runlist.Outcome, error) {
		if q.Yes() {
			return read, nil
		}
		return pick(ctx, read, req, run)
	}
	cmd := &cobra.Command{Use: "list [worktree]", Short: listShort, Long: listLong, Example: listExample, Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return q.Launch(cmd, args)
	}}
	q.Bind(cmd, run)

	return cmd
}
