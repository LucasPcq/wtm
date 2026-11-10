// Package cli is config show's command line: config.toml drawn as a document, its JSON, over config show's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/configshow"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	showShort = "Show the project's settings: each value, its default and what it does"
	showLong  = "Read the project's config.toml as a document: where new worktrees go and the\n" +
		"branch they start from, where their .env files come from and which, the commands\n" +
		"run when one is created or removed, and the answers create remembers. Each setting\n" +
		"says its value, whether it is the default, what it does and the values it takes.\n\n" +
		"A value wtm can't use is a warning on its setting; a file that doesn't parse is\n" +
		"refused, with where it stopped. It changes nothing: `wtm config set` changes a\n" +
		"setting, `wtm config edit` opens the file in your editor."
	showExample = `  wtm config show

  wtm config show --output json`
)

// query is config show's form: a read.
type query = form.Query[configshow.Request, configshow.Questions, configshow.Outcome]

// showFlags spells the failures: config show has no field.
var showFlags = (&query{Words: words}).Flags()

// New returns config show's command, a subcommand of config: it reads config.toml and draws it on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (configshow.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	run := &uicli.Run{Command: configshow.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: showFlags}
	q := &query{
		Words: words,
		Open: func(ctx context.Context, dir string) (form.Reader[configshow.Request, configshow.Questions, configshow.Outcome], error) {
			return open(ctx, dir)
		},
		Draw: drawShow,
	}
	cmd := &cobra.Command{Use: "show", Short: showShort, Long: showLong, Example: showExample, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return q.Launch(cmd, args)
	}}
	q.Bind(cmd, run)

	return cmd
}
