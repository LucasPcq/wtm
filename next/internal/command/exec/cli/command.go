// Package cli is exec's command line: its flags, its form, its output and its JSON, over exec's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	execShort = "Run a shell command in several worktrees at once"
	execLong  = "Run one /bin/sh line in each worktree named, or in every one with --all, from the worktree's\n" +
		"root, several at once (--jobs, one per CPU by default). Everything after -- is the command.\n" +
		"Without worktrees or a command, the wizard asks for them; your current worktree is picked first.\n" +
		"\n" +
		"Each command gets the environment of wtm-next, without the variables wtm sets in a worktree\n" +
		"nor the ports run.toml declares; in an isolated worktree whose jobs run docker compose, it gets\n" +
		"the worktree's own, as its hooks do. Its stdin is closed. Its output goes to a log under wtm's\n" +
		"state, overwritten by the next run; a worktree that fails shows its last lines and its log.\n" +
		"\n" +
		"A failure does not stop the others: the run ends with the line that runs the command again\n" +
		"where it did not pass. The exit code is 1 when any failed, whatever each command's own code."
	execExample = `  # Pick the worktrees, then type the command
  wtm exec

  wtm exec feat/login feat/signup -- pnpm test

  # Every worktree, two at a time, keeping each one's whole output
  wtm exec --all --jobs 2 --print -- git status --short

  # For a script or an agent, with the v2 JSON envelope
  wtm exec --all --yes --output json -- make lint`

	yesUsage = "Run unattended: no question, no confirmation (worktrees or --all, and the command after --, required)"
)

type execCommand struct {
	io   uicli.Run
	form *execForm
}

// New returns the exec command: its flags and the words after -- become an exec.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (exec.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &execCommand{io: uicli.Run{Command: exec.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: execFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: exec.Name + " [worktree...] [-- <command>]", Short: execShort, Long: execLong, Example: execExample, Args: cobra.ArbitraryArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *execCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names exec, from exec's schema, as wtm-next runs it; none when its answers are not exec's.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+exec.Name, followUp)
}
