// Package cli is list's command line: its flags, its table, its JSON, and the picker that ends on what to do next with a worktree, over list's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	listShort = "List all worktrees"
	listLong  = "List the worktrees of the repository: whether each holds uncommitted changes,\n" +
		"is locked or mid-rebase, its commits past the base branch, where its branch stands\n" +
		"against origin, the jobs up in it, and with --with-prs its newest pull request,\n" +
		"open, merged or closed.\n" +
		"It changes nothing.\n\n" +
		"On a terminal it opens a picker: choose a worktree, then what to do with it, and the\n" +
		"command that does it is printed (r refreshes from origin). Without a terminal, or\n" +
		"with --output json, it prints the list."
	listExample = `  # Pick a worktree, then what to do with it
  wtm list

  # Each worktree with its pull request, as JSON
  wtm list --with-prs --output json`
)

// query is list's form: a read.
type query = form.Query[list.Request, list.Questions, list.Outcome]

type listCommand struct {
	io    uicli.Run
	query *query
	app   list.UseCase // the use case opened, for the picker to read again
}

// listFlags spells the failures: list's only field is a flag.
var listFlags = (&query{}).Flags()

// New returns the list command: its flags become a list.Request, read, then picked from when the user can see and answer it; it draws the list on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (list.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &listCommand{io: uicli.Run{Command: list.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: listFlags}}
	c.query = &query{Open: c.open(open), Draw: drawList, Pick: c.pick}
	cmd := &cobra.Command{Use: list.Name, Short: listShort, Long: listLong, Example: listExample, Args: cobra.NoArgs, RunE: c.run}
	c.query.Bind(cmd, &c.io)

	return cmd
}

func (c *listCommand) run(cmd *cobra.Command, args []string) error {
	return c.query.Launch(cmd, args)
}

// open opens the use case with open, kept for the picker.
func (c *listCommand) open(open func(ctx context.Context, dir string) (list.UseCase, error)) func(ctx context.Context, dir string) (form.Reader[list.Request, list.Questions, list.Outcome], error) {
	return func(ctx context.Context, dir string) (form.Reader[list.Request, list.Questions, list.Outcome], error) {
		app, err := open(ctx, dir)
		c.app = app
		return app, err
	}
}
