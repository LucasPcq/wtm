// Package cli is status's command line: its flags, its worktree picker, the state of one worktree or the table of them all, and its JSON, over status's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	statusShort = "Show a worktree's whole state and what to fix"
	statusLong  = "Read one worktree's state in one document: where its branch stands against the\n" +
		"base branch and origin, its uncommitted work and last commits, how it runs\n" +
		"(isolation, addressing, port offset, every job run.toml declares with its state\n" +
		"and address), which declared .env files it lacks and how their keys stand, and each\n" +
		"problem found with the command that fixes it. With --with-prs, its newest pull\n" +
		"request, open, merged or closed, an open one's checks and review; a merged one\n" +
		"is a problem, its fix to clean the worktree.\n\n" +
		"Without [worktree], a terminal opens the worktree picker on the one you are in;\n" +
		"anywhere else (no terminal, --output json, --quiet) it reads the one you are in.\n" +
		"It changes nothing, never starts the run daemon and prints no .env value.\n" +
		"It exits 0 whatever it finds: read `problems`."
	statusExample = `  # Pick a worktree, the cursor on the one you are in
  wtm status

  wtm status feat/login --output json

  # Every worktree as a table, then each problem with its fix
  wtm status --all`
)

// query is status's form: a read.
type query = form.Query[status.Request, status.Questions, status.Outcome]

// statusFlags spells the failures: the worktree is positional, the others are flags.
var statusFlags = (&query{}).Flags()

// New returns the status command: its argument and flags become a status.Request, the worktree picked first when none is named and the user can see and answer the picker; it draws what it read on streams, or the v2 JSON envelope with --output json, the fixes spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (status.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	run := &uicli.Run{Command: status.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: statusFlags}
	q := &query{Open: standing(open, env.Cwd), Draw: drawStatus}
	cmd := &cobra.Command{Use: status.Name + " [worktree]", Short: statusShort, Long: statusLong, Example: statusExample, Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return q.Launch(cmd, args)
	}}
	q.Bind(cmd, run)

	return cmd
}

// standing opens the use case with open, its requests read from cwd, where the command line stands.
func standing(open func(ctx context.Context, dir string) (status.UseCase, error), cwd string) func(ctx context.Context, dir string) (form.Reader[status.Request, status.Questions, status.Outcome], error) {
	return func(ctx context.Context, dir string) (form.Reader[status.Request, status.Questions, status.Outcome], error) {
		app, err := open(ctx, dir)
		return here{app: app, cwd: cwd}, err
	}
}

// here is the use case asked from the directory cwd: the worktree it reads when none is named.
type here struct {
	app status.UseCase
	cwd string
}

func (h here) Questions(ctx context.Context, req status.Request) (status.Questions, error) {
	req.Cwd = h.cwd
	return h.app.Questions(ctx, req)
}

func (h here) Run(ctx context.Context, req status.Request, opts domain.RunOptions) (status.Outcome, error) {
	req.Cwd = h.cwd
	return h.app.Run(ctx, req, opts)
}
