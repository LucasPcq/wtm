// Package cli is env show's command line: its argument and flag, the worktree picker, the table of its .env files, and its JSON, over env show's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/envshow"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// Use is how the command line spells env show.
const Use = "env show"

const (
	showShort = "Show what a worktree's .env keys follow"
	showLong  = "Read one worktree's .env files app by app, file by file, key by key: where each file\n" +
		"comes from, what each key follows (a job's port or address, a value of run.toml, a value\n" +
		"wtm owns, or nothing), its value in the main checkout and here, and what wtm writes into\n" +
		"it. The keys to check come first: one that follows nothing although it holds a port or an\n" +
		"address, and one out of step with what wtm writes, which `wtm env` brings back.\n\n" +
		"Only the values wtm writes are shown; every other value, secrets included, is withheld\n" +
		"unless --show-values. With no worktree named, a terminal asks which one, the one you\n" +
		"stand in first; anywhere else it is that one. It changes nothing."
	showExample = `  # Pick a worktree, the cursor on the one you are in
  wtm env show

  wtm env show feat/login --output json`
)

// query is env show's form: a read.
type query = form.Query[envshow.Request, envshow.Questions, envshow.Outcome]

// showFlags spells the failures: the worktree is positional, the switch a flag.
var showFlags = (&query{Words: words}).Flags()

// New returns the env show command: its argument and flag become an envshow.Request, the worktree picked first when none is named and the user can see and answer the picker; it draws what it read on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (envshow.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	run := &uicli.Run{Command: envshow.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: showFlags}
	q := &query{
		Words: words,
		Open: func(ctx context.Context, dir string) (form.Reader[envshow.Request, envshow.Questions, envshow.Outcome], error) {
			return open(ctx, dir)
		},
		Draw: drawShow,
	}
	cmd := &cobra.Command{Use: "show [worktree]", Short: showShort, Long: showLong, Example: showExample, Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return q.Launch(cmd, args)
	}}
	q.Bind(cmd, run)

	return cmd
}
