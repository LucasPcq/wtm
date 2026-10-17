// Package cli is create's command line: its flags, its form, its output and its JSON, over create's use case.
package cli

import (
	"cmp"
	"context"
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	createShort = "Create one or more worktrees"
	createLong  = "Create one or more git worktrees with env provisioning, metadata, and hooks.\n" +
		"Several branches are created one after the other from the same source; a failure\n" +
		"does not stop the others, and the run ends with what was created and what failed.\n" +
		"A branch that already exists locally is checked out as-is, keeping its commits;\n" +
		"its parent can't be inferred, so --from then names the branch recorded for `wtm sync`.\n" +
		"A branch only origin has is created from it, tracking it.\n" +
		"When run.toml declares jobs, a branch whose derived name a live worktree or another\n" +
		"branch of the run already carries (feat.x next to feat/x) is refused.\n" +
		"Without arguments, the form asks for the branches: tab adds another, enter continues."
	createExample = `  # Answer the form: branches, source, env strategy, isolation
  wtm create

  # Three worktrees from the base branch, no prompts
  wtm create feat/login feat/billing fix/header --yes

  # A stacked branch on top of feat/login
  wtm create feat/login-ui --from feat/login --yes

  # See the plan first, then apply exactly that plan
  wtm create feat/login --dry-run
  wtm create feat/login --expect-fingerprint <hash>

  # For a script or an agent: idempotent, with the v2 JSON envelope
  wtm create feat/login --if-not-exists --output json`

	yesUsage = "Run unattended: no question, no confirmation; what is not given takes its default"
)

type createCommand struct {
	io   uicli.Run
	form *createForm
}

// New returns the create command: its flags become a create.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (create.UseCase, error)) *cobra.Command {
	// exits are the exit codes of create's own codes, ahead of those of their kind.
	exits := map[string]int{create.CodeConfigNotFound: uicli.ExitConfigNotFound}
	c := &createCommand{io: uicli.Run{Command: commandName, Streams: streams, Env: env, Flags: createFlags, Exits: exits}, form: newForm(open)}
	cmd := &cobra.Command{Use: "create [branch...]", Short: createShort, Long: createLong, Example: createExample, Args: cobra.ArbitraryArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *createCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, func(ran form.Ran[create.Request, create.Outcome], err error) error {
		return c.conclude(cmd, concluded{Outcome: ran.Outcome, Err: domain.AsError(domain.AsErrorParams{Err: err}), Request: ran.Request, Cwd: c.io.Env.Cwd})
	})
}

// concluded is how a run ended, for conclude: its outcome, its refusal, the request that ran and where.
type concluded struct {
	Outcome create.Outcome
	Err     *domain.Error
	Request create.Request
	Cwd     string
}

// conclude prints how the run ended — the JSON envelope, a refusal, the dry run and the line that applies it, or the outcome — and returns the failure that sets the exit code.
func (c *createCommand) conclude(cmd *cobra.Command, done concluded) error {
	line := createLine{Command: cmd.CommandPath(), Request: done.Request}
	if !c.io.Human() {
		doc := done.Outcome.Envelope(done.Err)
		doc.FollowUps = append(doc.FollowUps, uicli.Replan(doc.Err, line.Argv())...)
		return c.io.Envelope(doc)
	}

	stdout, _ := c.io.Writers()
	switch {
	case done.Err != nil:
		return c.io.Refuse(done.Err, line.Argv())
	case done.Outcome.Result == nil:
		apply := line.String() + " --expect-fingerprint " + done.Outcome.Fingerprint
		output.Frame(stdout, func(w io.Writer) {
			output.DryRun(w, output.DryRunParams{Command: commandName, Rows: planRowsOf(done.Outcome.Plan), Warnings: done.Outcome.Warnings, Fingerprint: done.Outcome.Fingerprint, Apply: apply})
		})
		return nil
	}

	followUps := itemFollowUps(done.Outcome.Result.Items)
	output.Frame(stdout, func(w io.Writer) {
		drawOutcome(w, done.Outcome, followUps, done.Cwd)
	})

	return c.io.Failed(cmp.Or(done.Err, done.Outcome.Failure()))
}

// itemFollowUps are the commands the failed branches suggest, to repair what they left behind.
func itemFollowUps(items []create.Item) []output.NextStep {
	var steps []output.NextStep
	for _, item := range items {
		steps = append(steps, output.FollowUpSteps(item.Err)...)
	}

	return steps
}
