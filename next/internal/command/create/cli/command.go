// Package cli is create's command line: its flags, its form, its output and its JSON, over create's use case.
package cli

import (
	"cmp"
	"context"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
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

	rememberUsage = "The answers to always use in this repository: env_from, isolation, fast_forward (repeatable)"
	yesUsage      = "Run unattended: no question, no confirmation; what is not given takes its default"

	flagFF             = "ff"
	exitConfigNotFound = 12
)

// exits are the exit codes of create's own codes, ahead of those of their kind.
var exits = map[string]int{create.CodeConfigNotFound: exitConfigNotFound}

// A Project is what the command runs against, found by New's open once the flags are valid: the use case wired to the repository, and the directory the command runs in, from which paths are shown.
type Project struct {
	UseCase create.UseCase
	Cwd     string
}

type createCommand struct {
	io          *output.Streams
	open        func(ctx context.Context) (Project, error)
	from        string
	envFrom     string
	isolation   string
	ff          bool
	ifNotExists bool
	remember    []string
	yes         bool
	output      string
	dryRun      bool
	expect      string
	ask         bool
	quiet       bool
}

// New returns the create command: its flags become a create.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json.
func New(streams *output.Streams, open func(ctx context.Context) (Project, error)) *cobra.Command {
	c := &createCommand{io: streams, open: open}
	cmd := &cobra.Command{Use: "create [branch...]", Short: createShort, Long: createLong, Example: createExample, Args: cobra.ArbitraryArgs, RunE: c.run}

	flags := cmd.Flags()
	flags.StringVar(&c.from, "from", "", descriptions["from"])
	flags.StringVar(&c.envFrom, "env-from", "", descriptions["env_from"])
	flags.StringVar(&c.isolation, "isolation", "", descriptions["isolation"])
	flags.BoolVar(&c.ff, flagFF, false, descriptions["fast_forward"])
	flags.BoolVar(&c.ifNotExists, "if-not-exists", false, descriptions["if_not_exists"])
	flags.StringArrayVar(&c.remember, "remember", nil, rememberUsage)
	flags.BoolVarP(&c.yes, "yes", "y", false, yesUsage)
	flags.StringVar(&c.output, "output", uicli.OutputText, uicli.OutputUsage)
	flags.BoolVarP(&c.dryRun, "dry-run", "n", false, uicli.DryRunUsage)
	flags.StringVar(&c.expect, "expect-fingerprint", "", uicli.ExpectUsage)
	flags.BoolVar(&c.ask, "ask", false, uicli.AskUsage)

	_ = cmd.RegisterFlagCompletionFunc("env-from", cobra.FixedCompletions(create.Names(project.EnvStrategies()), cobra.ShellCompDirectiveNoFileComp))
	_ = cmd.RegisterFlagCompletionFunc("isolation", cobra.FixedCompletions(create.Names(project.Isolations()), cobra.ShellCompDirectiveNoFileComp))

	// A flag cobra cannot parse is refused as any invalid request is: framed, or in the envelope.
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		invalid := domain.Invalid(nil)
		invalid.Cause = err
		return c.misused(cmd, invalid)
	})

	return cmd
}

func (c *createCommand) run(cmd *cobra.Command, args []string) error {
	c.quiet, _ = cmd.Flags().GetBool(uicli.FlagQuiet)
	req := c.request(cmd, args)

	if c.output != uicli.OutputText && c.output != uicli.OutputJSON {
		refused := domain.InvalidMessage(`invalid --output "` + c.output + `": expected one of text, json`)
		// The refusal of an unknown format is printed as text, the one format sure to be understood.
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
	ran, err := c.create(cmd, project.UseCase, req, progress.Report)
	progress.Close()

	return c.conclude(cmd, concluded{Outcome: ran.Outcome, Err: domain.AsError(domain.AsErrorParams{Err: err}), Request: ran.Request, Cwd: project.Cwd})
}

// create runs the request in the wizard when the user can answer it, else unattended; it returns the request that ran, the wizard's answers included.
func (c *createCommand) create(cmd *cobra.Command, useCase create.UseCase, req create.Request, report domain.Report) (ran, error) {
	ctx := cmd.Context()

	if c.interactive() {
		return runForm(ctx, formParams{App: useCase, Request: req, DryRun: c.dryRun, Report: report, In: c.io.In, Out: c.io.Err})
	}

	outcome, err := useCase.Run(ctx, req, operation.Options{DryRun: c.dryRun, ExpectFingerprint: c.expect, Surface: uicli.SurfaceCLI, Report: report})
	return ran{Outcome: outcome, Request: req}, err
}

func (c *createCommand) request(cmd *cobra.Command, args []string) create.Request {
	req := create.Request{
		From:        c.from,
		EnvFrom:     project.EnvStrategy(c.envFrom),
		Isolation:   project.Isolation(c.isolation),
		IfNotExists: c.ifNotExists,
		Remember:    c.remember,
		Ask:         c.ask,
	}

	for _, arg := range args {
		req.Branches = append(req.Branches, strings.TrimSpace(arg))
	}

	if cmd.Flags().Changed(flagFF) {
		req.FastForward = new(c.ff)
	}

	return req
}

func (c *createCommand) human() bool {
	return c.output == uicli.OutputText
}

// interactive reports whether the wizard runs: the user can see and answer it, the output is for a human, and neither --yes nor a previewed plan to apply asks for an unattended run.
func (c *createCommand) interactive() bool {
	return c.io.CanPrompt() && c.expect == "" && c.human() && !c.yes
}

// streams are stdout and stderr, both silenced by --quiet in human output.
func (c *createCommand) streams() (io.Writer, io.Writer) {
	if c.quiet && c.human() {
		return io.Discard, io.Discard
	}

	return c.io.Out, c.io.Err
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
	if !c.human() {
		return c.envelope(envelopeParams{Outcome: done.Outcome, Err: done.Err})
	}

	stdout, _ := c.streams()
	line := createLine{Command: cmd.CommandPath(), Request: done.Request}
	switch {
	case done.Err != nil:
		return c.fail(cmd, done.Err, done.Request)
	case done.Outcome.Result == nil:
		apply := line.String() + " --expect-fingerprint " + done.Outcome.Fingerprint
		output.Frame(stdout, func(w io.Writer) { drawDryRun(w, done.Outcome, apply) })
		return nil
	}

	followUps := itemFollowUps(done.Outcome.Result.Items)
	output.Frame(stdout, func(w io.Writer) {
		drawOutcome(w, done.Outcome, followUps, done.Cwd)
	})

	return c.failed(cmp.Or(done.Err, done.Outcome.Failure()))
}

// fail prints a refusal; a stale plan is followed by the line that previews it again.
func (c *createCommand) fail(cmd *cobra.Command, failure *domain.Error, req create.Request) error {
	var next []output.NextStep
	if failure.Code == operation.CodePlanStale {
		next = []output.NextStep{{Command: createLine{Command: cmd.CommandPath(), Request: req}.String() + " --dry-run", Note: output.ReplanNote}}
	}

	return c.failWith(failure, next)
}

// misused prints a misuse of the command line, followed by the way to its help.
func (c *createCommand) misused(cmd *cobra.Command, failure *domain.Error) error {
	return c.failWith(failure, []output.NextStep{uicli.HelpStep(cmd)})
}

func (c *createCommand) failWith(failure *domain.Error, next []output.NextStep) error {
	if !c.human() {
		return c.envelope(envelopeParams{Err: failure})
	}

	_, stderr := c.streams()
	output.Frame(stderr, func(w io.Writer) {
		output.Failure(w, output.FailureParams{Err: failure, Next: next, Flags: createFlags})
	})

	return c.failed(failure)
}

// envelope writes the v2 JSON envelope on stdout and returns its failure; an envelope that cannot be written is the failure.
func (c *createCommand) envelope(params envelopeParams) error {
	doc := envelopeOf(params)
	if err := output.Envelope(c.io.Out, doc); err != nil {
		return c.failed(domain.AsError(domain.AsErrorParams{Err: err}))
	}

	return c.failed(doc.Err)
}

// failed is the error the root reads the exit code and the --quiet line from, nil for no failure.
func (c *createCommand) failed(failure *domain.Error) error {
	if failure == nil {
		return nil
	}

	return &uicli.Failed{Err: failure, Quiet: c.quiet && c.human(), Flags: createFlags, Exits: exits}
}

// itemFollowUps are the commands the failed branches suggest, to repair what they left behind.
func itemFollowUps(items []create.Item) []output.NextStep {
	var steps []output.NextStep
	for _, item := range items {
		if item.Err == nil || item.Err.FollowUp == nil {
			continue
		}
		steps = append(steps, output.NextStep{Command: output.FollowUpLine(item.Err.FollowUp), Note: item.Err.FollowUp.Message})
	}

	return steps
}
