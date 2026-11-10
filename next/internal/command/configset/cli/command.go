// Package cli is config set's command line: its form, its output and its JSON, over config set's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/configset"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// Use is how the command line spells config set after the program's name.
const Use = "config set"

const (
	setShort = "Change the project's settings in config.toml"
	setLong  = "Change one or more of the project's settings: the folder new worktrees go in,\n" +
		"the branch they start from, where their .env files come from and which, the\n" +
		"commands run when one is created or removed, and the answers create remembers.\n\n" +
		"A setting given replaces config.toml's; the others are kept, and none is asked.\n" +
		"With no flag, a terminal asks every setting. A list given replaces\n" +
		"the whole list; given empty, it is none. The file is written whole, and refused\n" +
		"when it changed on disk since the preview. Its outcome ends on the command that\n" +
		"puts back what it changed."
	setExample = `  wtm config set --base-path ../wt

  wtm config set --env-strategy main --env-file apps/api/.env --env-file apps/web/.env

  # Forget every remembered answer
  wtm config set --remembered= --yes`

	yesUsage = "Write without asking"
)

type setCommand struct {
	io   uicli.Run
	form *setForm
}

// New returns config set's command, a subcommand of config: it previews the change, asks to confirm it when the user can see and answer it, then draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (configset.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &setCommand{io: uicli.Run{Command: configset.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: setFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: "set", Short: setShort, Long: setLong, Example: setExample, Args: cobra.NoArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *setCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line is the command line of a follow-up that runs config set: the program, `config set`, then the answers as its flags.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+Use, followUp)
}
