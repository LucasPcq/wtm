// Package cli is config edit's command line: config.toml opened in the user's editor, then checked, over config edit's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/configedit"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// Use is how the command line spells config edit after the program's name.
const Use = "config edit"

const (
	editShort = "Open the project's config.toml in your editor, then check it"
	editLong  = "Open config.toml in $VISUAL, else $EDITOR, else vi, and check it once the editor\n" +
		"is closed. $EDITOR may carry arguments (code --wait).\n\n" +
		"A file that no longer parses, or holds a setting wtm can't use, is kept as edited:\n" +
		"each problem is named and the edit offered again. The commands that read it refuse\n" +
		"until it is fixed."
	editExample = `  wtm config edit

  EDITOR="code --wait" wtm config edit`
)

// query is config edit's form: it asks nothing.
type query = form.Query[configedit.Request, configedit.Questions, configedit.Outcome]

// editFlags spells the failures: config edit has no field.
var editFlags = (&query{Words: words}).Flags()

// New returns config edit's command, a subcommand of config: it opens config.toml in the editor, then draws what the editor left on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (configedit.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	run := &uicli.Run{Command: configedit.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: editFlags, Exits: map[string]int{configedit.CodeInvalid: uicli.ExitError}}
	q := &query{
		Words: words,
		Open: func(ctx context.Context, dir string) (form.Reader[configedit.Request, configedit.Questions, configedit.Outcome], error) {
			return open(ctx, dir)
		},
		Draw: drawEdit,
	}
	cmd := &cobra.Command{Use: "edit", Short: editShort, Long: editLong, Example: editExample, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return q.Launch(cmd, args)
	}}
	q.Bind(cmd, run)

	return cmd
}

// Line is the command line of a follow-up that runs config edit: the program and `config edit`.
func Line(followUp domain.FollowUp) []string {
	return (&query{Words: words}).Spell(uicli.Program+" "+Use, followUp)
}
