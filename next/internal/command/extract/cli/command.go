// Package cli is extract's command line: its flags, its form, its dry run, its output and its JSON, over extract's use case.
package cli

import (
	"context"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	extractShort = "Move uncommitted changes to another worktree"
	extractLong  = "Move a subset of a worktree's uncommitted changes to another worktree\n" +
		"(new or existing) to split an oversized PR or isolate unrelated work.\n\n" +
		"On a terminal the wizard asks what the flags leave open: the source worktree,\n" +
		"the files, the target (an existing worktree, or a new one: its branch, source\n" +
		"branch and isolation), move or copy, and what to do when the files conflict\n" +
		"with the target; its recap previews the plan, then applies exactly that plan.\n" +
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

	rememberUsage = "The answers to always use in this repository: isolation, fast_forward (repeatable)"
	yesUsage      = "Run unattended: no question, no confirmation; what is not given takes its default (a source, --files and --to are required, and --from when --to already exists locally)"

	flagFF       = "ff"
	flagKeep     = "keep"
	exitConflict = 15
)

// A Project is what the command runs against, found by New's open once the flags are valid: the use case wired to the repository, and the directory the command runs in, from which paths are shown.
type Project struct {
	UseCase extract.UseCase
	Cwd     string
}

type extractCommand struct {
	io         uicli.Run
	open       func(ctx context.Context) (Project, error)
	files      []string
	to         string
	from       string
	isolation  string
	ff         bool
	keep       bool
	onConflict string
	remember   []string
	dryRun     bool
	expect     string
	ask        bool
	yes        bool
}

// New returns the extract command: its flags become an extract.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json.
func New(streams *output.Streams, open func(ctx context.Context) (Project, error)) *cobra.Command {
	// exits are the exit codes of extract's own codes, ahead of those of their kind.
	exits := map[string]int{extract.CodeConfigNotFound: uicli.ExitConfigNotFound, extract.CodeConflict: exitConflict}
	c := &extractCommand{io: uicli.Run{Command: commandName, Streams: streams, Flags: extractFlags, Exits: exits}, open: open}
	cmd := &cobra.Command{Use: "extract [source]", Short: extractShort, Long: extractLong, Example: extractExample, Args: cobra.MaximumNArgs(1), RunE: c.run}

	flags := cmd.Flags()
	flags.StringSliceVar(&c.files, "files", nil, descriptions["files"])
	flags.StringVar(&c.to, "to", "", descriptions["to"])
	flags.StringVar(&c.isolation, "isolation", "", descriptions["isolation"])
	flags.StringVar(&c.from, "from", "", descriptions["from"])
	flags.BoolVar(&c.ff, flagFF, false, descriptions["fast_forward"])
	flags.BoolVar(&c.keep, flagKeep, false, descriptions["keep"])
	flags.StringVar(&c.onConflict, "on-conflict", "", descriptions["on_conflict"])
	flags.StringArrayVar(&c.remember, "remember", nil, rememberUsage)
	flags.BoolVar(&c.ask, "ask", false, uicli.AskUsage)
	flags.BoolVarP(&c.yes, "yes", "y", false, yesUsage)
	flags.StringVar(&c.io.Output, "output", uicli.OutputText, uicli.OutputUsage)
	flags.BoolVarP(&c.dryRun, "dry-run", "n", false, uicli.DryRunUsage)
	flags.StringVar(&c.expect, "expect-fingerprint", "", uicli.ExpectUsage)

	_ = cmd.RegisterFlagCompletionFunc("isolation", cobra.FixedCompletions(domain.Names(project.Isolations()), cobra.ShellCompDirectiveNoFileComp))
	_ = cmd.RegisterFlagCompletionFunc("on-conflict", cobra.FixedCompletions(domain.Names(extract.OnConflicts()), cobra.ShellCompDirectiveNoFileComp))

	return cmd
}

func (c *extractCommand) run(cmd *cobra.Command, args []string) error {
	if err := c.io.Begin(cmd); err != nil {
		return err
	}

	req := c.request(cmd, args)

	ctx := cmd.Context()
	project, err := c.open(ctx)
	if err != nil {
		return c.fail(cmd, domain.AsError(domain.AsErrorParams{Err: err}), req)
	}

	progress := c.io.Progress(ctx)
	ran, err := c.extract(cmd, project.UseCase, req, progress.Report)
	progress.Close()

	return c.conclude(cmd, concluded{Outcome: ran.Outcome, Err: domain.AsError(domain.AsErrorParams{Err: err}), Request: ran.Request, Cwd: project.Cwd})
}

// extract runs the request in the wizard when the user can answer it, else unattended; it returns the request that ran, the wizard's answers included.
func (c *extractCommand) extract(cmd *cobra.Command, useCase extract.UseCase, req extract.Request, report domain.Report) (ran, error) {
	ctx := cmd.Context()

	if c.interactive() {
		return runForm(ctx, formParams{App: useCase, Request: req, DryRun: c.dryRun, Report: report, In: c.io.Streams.In, Out: c.io.Streams.Err})
	}

	outcome, err := useCase.Run(ctx, req, operation.Options{DryRun: c.dryRun, ExpectFingerprint: c.expect, Surface: uicli.SurfaceCLI, Report: report})
	return ran{Outcome: outcome, Request: req}, err
}

// interactive reports whether the wizard runs: the user can see and answer it, the output is for a human, and neither --yes nor a previewed plan to apply asks for an unattended run.
func (c *extractCommand) interactive() bool {
	return c.io.Streams.CanPrompt() && c.expect == "" && c.io.Human() && !c.yes
}

func (c *extractCommand) request(cmd *cobra.Command, args []string) extract.Request {
	req := extract.Request{
		Files:      c.files,
		To:         c.to,
		From:       c.from,
		Isolation:  project.Isolation(c.isolation),
		OnConflict: extract.OnConflict(c.onConflict),
		Remember:   c.remember,
		Ask:        c.ask,
	}

	if len(args) > 0 {
		req.Source = strings.TrimSpace(args[0])
	}

	if cmd.Flags().Changed(flagFF) {
		req.FastForward = new(c.ff)
	}
	if cmd.Flags().Changed(flagKeep) {
		req.Keep = new(c.keep)
	}

	return req
}

// concluded is how a run ended, for conclude: its outcome, its refusal, the request that ran and where.
type concluded struct {
	Outcome extract.Outcome
	Err     *domain.Error
	Request extract.Request
	Cwd     string
}

// conclude prints how the run ended — the JSON envelope, a refusal, nothing to extract, the dry run and the line that applies it, or the outcome — and returns the failure that sets the exit code.
func (c *extractCommand) conclude(cmd *cobra.Command, done concluded) error {
	if !c.io.Human() {
		return c.io.Envelope(envelopeOf(envelopeParams{Outcome: done.Outcome, Err: done.Err}))
	}

	stdout, _ := c.io.Writers()
	outcome := done.Outcome
	switch {
	case done.Err != nil:
		return c.fail(cmd, done.Err, done.Request)
	case outcome.Result == nil && outcome.Plan.Nothing():
		output.Frame(stdout, func(w io.Writer) { drawNothing(w, outcome.Plan) })
		return nil
	case outcome.Result == nil:
		apply := extractLine{Command: cmd.CommandPath(), Request: done.Request}.String() + " --expect-fingerprint " + outcome.Fingerprint
		output.Frame(stdout, func(w io.Writer) {
			output.DryRun(w, output.DryRunParams{Command: commandName, Rows: planRowsOf(outcome.Plan), Warnings: outcome.Warnings, Fingerprint: outcome.Fingerprint, Apply: apply})
		})
		return nil
	}

	output.Frame(stdout, func(w io.Writer) { drawOutcome(w, outcome, done.Cwd) })
	return c.io.Failed(outcome.Failure())
}

// fail prints a refusal; a stale plan is followed by the line that previews it again.
func (c *extractCommand) fail(cmd *cobra.Command, failure *domain.Error, req extract.Request) error {
	var next []output.NextStep
	if failure.Code == operation.CodePlanStale {
		next = []output.NextStep{{Command: extractLine{Command: cmd.CommandPath(), Request: req}.String() + " --dry-run", Note: output.ReplanNote}}
	}

	return c.io.FailWith(failure, next)
}
