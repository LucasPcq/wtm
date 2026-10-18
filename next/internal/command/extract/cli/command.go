// Package cli is extract's command line: its flags, its form, its dry run, its output and its JSON, over extract's use case.
package cli

import (
	"context"
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	extractShort = "Move uncommitted changes to another worktree"
	extractLong  = "Move a subset of a worktree's uncommitted changes to another worktree\n" +
		"(new or existing) to split an oversized PR or isolate unrelated work.\n\n" +
		"On a terminal the wizard asks what the flags leave open: the source worktree,\n" +
		"the files, the target (an existing worktree, or a new one: its branch, source\n" +
		"branch, env strategy and isolation), move or copy, and what to do when the\n" +
		"files conflict with the target; its recap previews the plan, then applies\n" +
		"exactly that plan.\n" +
		"With --yes, --output json or no terminal nothing is asked: a source, --files\n" +
		"and --to are then required.\n\n" +
		"A --to branch that already exists locally is checked out as-is, keeping its\n" +
		"commits. Its parent can't be inferred, so --from then names the branch recorded\n" +
		"for `wtm sync` — asked in the wizard, required without it.\n\n" +
		"Untracked files are listed one by one, including inside brand-new directories,\n" +
		"so you can take part of a new folder; gitignored files are never listed.\n\n" +
		"Conflicts are found before anything is created or moved. It aborts by default,\n" +
		"leaving both worktrees untouched; --on-conflict resolve writes conflict markers\n" +
		"in the target so you can resolve them like a rebase. A file that merely already\n" +
		"exists in the target with other content counts as a conflict too.\n" +
		"--dry-run shows the plan, its conflicts and a fingerprint."
	extractExample = `  # Pick the source, the files and the target
  wtm extract

  # Move a directory's changes to a new branch
  wtm extract feat/login --files apps/api --to feat/login-api --yes

  # Copy one file instead, onto a branch stacked on the source
  wtm extract feat/login --files apps/web/login.ts --to feat/login-web --from feat/login --keep --yes`

	yesUsage = "Run unattended: no question, no confirmation; what is not given takes its default (a source, --files and --to are required, and --from when --to already exists locally)"

	exitConflict = 15
)

type extractCommand struct {
	io   uicli.Run
	form *extractForm
}

// New returns the extract command: its flags become an extract.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (extract.UseCase, error)) *cobra.Command {
	// exits are the exit codes of extract's own codes, ahead of those of their kind.
	exits := map[string]int{extract.CodeConfigNotFound: uicli.ExitConfigNotFound, extract.CodeConflict: exitConflict}
	c := &extractCommand{io: uicli.Run{Command: commandName, Streams: streams, Env: env, Flags: extractFlags, Exits: exits}, form: newForm(open)}
	cmd := &cobra.Command{Use: extract.Name + " [source]", Short: extractShort, Long: extractLong, Example: extractExample, Args: cobra.MaximumNArgs(1), RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *extractCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.conclude)
}

// conclude prints how the run ended — the JSON envelope, a refusal, nothing to extract, the dry run and the line that applies it, or the outcome — and returns the failure that sets the exit code.
func (c *extractCommand) conclude(cmd *cobra.Command, ran form.Ran[extract.Request, extract.Outcome], err error) error {
	failure := domain.AsError(domain.AsErrorParams{Err: err})
	argv := c.form.Argv(cmd.CommandPath(), ran.Request)
	outcome := ran.Outcome
	if !c.io.Human() {
		doc := ran.Outcome.Envelope(commandName, failure)
		doc.FollowUps = append(doc.FollowUps, uicli.Replan(doc.Err, argv)...)
		return c.io.Envelope(doc)
	}

	stdout, _ := c.io.Writers()
	switch {
	case failure != nil:
		return c.io.Refuse(failure, argv)
	case outcome.Result == nil && outcome.Plan.Nothing():
		output.Frame(stdout, func(w io.Writer) { drawNothing(w, outcome.Plan) })
		return nil
	case outcome.Result == nil:
		apply := output.Line(argv) + " --expect-fingerprint " + outcome.Fingerprint
		output.Frame(stdout, func(w io.Writer) {
			output.DryRun(w, output.DryRunParams{Command: commandName, Rows: planRowsOf(outcome.Plan), Warnings: outcome.Warnings, Fingerprint: outcome.Fingerprint, Apply: apply})
		})
		return nil
	}

	output.Frame(stdout, func(w io.Writer) { drawOutcome(w, outcome, c.io.Env.Cwd) })
	return c.io.Failed(outcome.Failure())
}
