// Package cli is extract's command line: its flags, its dry run, its output and its JSON, over extract's use case.
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
		"The source worktree is the first thing chosen: pass its branch as [source],\n" +
		"or omit it to pick interactively from the worktrees that have changes. A source\n" +
		"is required when there is no terminal or with --output json.\n\n" +
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
	yesUsage      = "Skip all prompts; resolve every decision from flags and safe defaults (requires a source arg, --files and --to; --from is also required when --to already exists locally; errors if a selection is missing)"

	flagFF             = "ff"
	flagKeep           = "keep"
	exitConfigNotFound = 12
	exitConflict       = 15
)

var exits = map[string]int{extract.CodeConfigNotFound: exitConfigNotFound, extract.CodeConflict: exitConflict}

type Project struct {
	UseCase extract.UseCase
	Cwd     string
}

type extractCommand struct {
	io         *output.Streams
	open       func(ctx context.Context) (Project, error)
	files      []string
	to         string
	from       string
	isolation  string
	ff         bool
	keep       bool
	onConflict string
	remember   []string
	output     string
	dryRun     bool
	expect     string
	ask        bool
	quiet      bool
}

func New(streams *output.Streams, open func(ctx context.Context) (Project, error)) *cobra.Command {
	c := &extractCommand{io: streams, open: open}
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
	flags.BoolP("yes", "y", false, yesUsage)
	flags.StringVar(&c.output, "output", uicli.OutputText, uicli.OutputUsage)
	flags.BoolVarP(&c.dryRun, "dry-run", "n", false, uicli.DryRunUsage)
	flags.StringVar(&c.expect, "expect-fingerprint", "", uicli.ExpectUsage)
	_ = cmd.RegisterFlagCompletionFunc("isolation", cobra.FixedCompletions(extract.Names(project.Isolations()), cobra.ShellCompDirectiveNoFileComp))
	_ = cmd.RegisterFlagCompletionFunc("on-conflict", cobra.FixedCompletions(extract.Names(extract.OnConflicts()), cobra.ShellCompDirectiveNoFileComp))
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		invalid := domain.Invalid(nil)
		invalid.Cause = err
		return c.misused(cmd, invalid)
	})
	return cmd
}

func (c *extractCommand) run(cmd *cobra.Command, args []string) error {
	c.quiet, _ = cmd.Flags().GetBool(uicli.FlagQuiet)
	req := c.request(cmd, args)
	if c.output != uicli.OutputText && c.output != uicli.OutputJSON {
		refused := domain.InvalidMessage(`invalid --output "` + c.output + `": expected one of text, json`)
		c.output = uicli.OutputText
		return c.misused(cmd, refused)
	}
	ctx := cmd.Context()
	project, err := c.open(ctx)
	if err != nil {
		return c.fail(cmd, domain.AsError(domain.AsErrorParams{Err: err}), req)
	}
	_, stderr := c.streams()
	progress := output.NewProgress(ctx, output.ProgressParams{W: stderr, Human: c.human(), Live: !c.quiet && c.io.CanAnimate()})
	outcome, err := project.UseCase.Run(ctx, req, operation.Options{DryRun: c.dryRun, ExpectFingerprint: c.expect, Surface: uicli.SurfaceCLI, Report: progress.Report})
	progress.Close()
	return c.conclude(cmd, concluded{Outcome: outcome, Err: domain.AsError(domain.AsErrorParams{Err: err}), Request: req, Cwd: project.Cwd})
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

func (c *extractCommand) human() bool {
	return c.output == uicli.OutputText
}

func (c *extractCommand) streams() (io.Writer, io.Writer) {
	if c.quiet && c.human() {
		return io.Discard, io.Discard
	}
	return c.io.Out, c.io.Err
}

type concluded struct {
	Outcome extract.Outcome
	Err     *domain.Error
	Request extract.Request
	Cwd     string
}

func (c *extractCommand) conclude(cmd *cobra.Command, done concluded) error {
	if !c.human() {
		return c.envelope(envelopeParams{Outcome: done.Outcome, Err: done.Err})
	}
	stdout, _ := c.streams()
	outcome := done.Outcome
	switch {
	case done.Err != nil:
		return c.fail(cmd, done.Err, done.Request)
	case outcome.Result == nil && outcome.Plan.Nothing():
		output.Frame(stdout, func(w io.Writer) { drawNothing(w, outcome.Plan) })
		return nil
	case outcome.Result == nil:
		apply := extractLine{Command: cmd.CommandPath(), Request: done.Request}.String() + " --expect-fingerprint " + outcome.Fingerprint
		output.Frame(stdout, func(w io.Writer) { drawDryRun(w, outcome, apply) })
		return nil
	}
	output.Frame(stdout, func(w io.Writer) { drawOutcome(w, outcome, done.Cwd) })
	return c.failed(outcome.Failure())
}

func (c *extractCommand) fail(cmd *cobra.Command, failure *domain.Error, req extract.Request) error {
	var next []output.NextStep
	if failure.Code == operation.CodePlanStale {
		next = []output.NextStep{{Command: extractLine{Command: cmd.CommandPath(), Request: req}.String() + " --dry-run", Note: output.ReplanNote}}
	}
	return c.failWith(failure, next)
}

func (c *extractCommand) misused(cmd *cobra.Command, failure *domain.Error) error {
	return c.failWith(failure, []output.NextStep{uicli.HelpStep(cmd)})
}

func (c *extractCommand) failWith(failure *domain.Error, next []output.NextStep) error {
	if !c.human() {
		return c.envelope(envelopeParams{Err: failure})
	}
	_, stderr := c.streams()
	output.Frame(stderr, func(w io.Writer) {
		output.Failure(w, output.FailureParams{Err: failure, Next: next, Flags: extractFlags})
	})
	return c.failed(failure)
}

func (c *extractCommand) envelope(params envelopeParams) error {
	doc := envelopeOf(params)
	if err := output.Envelope(c.io.Out, doc); err != nil {
		return c.failed(domain.AsError(domain.AsErrorParams{Err: err}))
	}
	return c.failed(doc.Err)
}

func (c *extractCommand) failed(failure *domain.Error) error {
	if failure == nil {
		return nil
	}
	return &uicli.Failed{Err: failure, Quiet: c.quiet && c.human(), Flags: extractFlags, Exits: exits}
}
