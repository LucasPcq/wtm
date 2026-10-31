// Package cli is checkout's command line: its flags, its form, its output and its JSON, over checkout's use case.
package cli

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	checkoutShort = "Create a worktree from an existing pull request"
	checkoutLong  = "Create a worktree from a pull request's branch, with env provisioning, metadata, and hooks.\n" +
		"A local branch of the pull request's name is checked out as-is, keeping commits you never\n" +
		"pushed; it is fast-forwarded when it is behind origin if you accept, or with --ff.\n" +
		"Otherwise the branch is created from origin's, tracking it.\n" +
		"Its parent, the branch `wtm sync` rebases it onto, is the branch the pull request merges into\n" +
		"unless --from names another. A pull request from a fork is refused: `gh pr checkout` does it.\n" +
		"Needs gh, GitHub's CLI, signed in. Without a number, the wizard lists the open pull requests."
	checkoutExample = `  # Pick among the open pull requests
  wtm checkout

  # Only the ones waiting for your review
  wtm checkout --review

  wtm checkout 42

  # See the plan first, then apply exactly that plan
  wtm checkout 42 --dry-run
  wtm checkout 42 --expect-fingerprint <hash>

  # No prompts, with the v2 JSON envelope
  wtm checkout 42 --yes --output json`

	yesUsage = "Run unattended: no question, no confirmation; what is not given takes its default (pull request number required; the parent defaults to the branch the pull request merges into)"
)

type checkoutCommand struct {
	io   uicli.Run
	form *checkoutForm
}

// New returns the checkout command: its flags become a checkout.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (checkout.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &checkoutCommand{io: uicli.Run{Command: checkout.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: checkoutFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: checkout.Name + " [number]", Short: checkoutShort, Long: checkoutLong, Example: checkoutExample, Args: cobra.MatchAll(cobra.MaximumNArgs(1), prNumber), RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

// prNumber refuses an argument that is no pull request number before anything is read, since the form takes a flag's value as its field's.
func prNumber(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}

	_, err := checkout.Request{}.Answer(checkout.PathPR, args)
	if refused, ok := errors.AsType[*domain.Error](err); ok && len(refused.Fields) > 0 {
		return errors.New(refused.Fields[0].Message(checkoutFlags[checkout.PathPR]))
	}

	return err
}

func (c *checkoutCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}
