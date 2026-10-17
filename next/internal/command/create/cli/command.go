// Package cli is create's command line: its flags, its form, its output and its JSON, over create's use case.
package cli

import (
	"cmp"
	"context"
	"io"
	"os"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
	"golang.org/x/term"
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

var exits = map[string]int{create.CodeConfigNotFound: exitConfigNotFound}

type Project struct {
	UseCase create.UseCase
	Cwd     string
}

type createCommand struct {
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

func New(open func(ctx context.Context) (Project, error)) *cobra.Command {
	c := &createCommand{open: open}
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
	flags.BoolVar(&c.dryRun, "dry-run", false, uicli.DryRunUsage)
	flags.StringVar(&c.expect, "expect-fingerprint", "", uicli.ExpectUsage)
	flags.BoolVar(&c.ask, "ask", false, uicli.AskUsage)
	_ = cmd.RegisterFlagCompletionFunc("env-from", cobra.FixedCompletions(create.Names(project.EnvStrategies()), cobra.ShellCompDirectiveNoFileComp))
	_ = cmd.RegisterFlagCompletionFunc("isolation", cobra.FixedCompletions(create.Names(project.Isolations()), cobra.ShellCompDirectiveNoFileComp))
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		invalid := domain.Invalid(nil)
		invalid.Cause = err
		return c.fail(cmd, refusal{Err: invalid})
	})
	return cmd
}

func (c *createCommand) run(cmd *cobra.Command, args []string) error {
	c.quiet, _ = cmd.Flags().GetBool(uicli.FlagQuiet)
	req := c.request(cmd, args)
	if c.output != uicli.OutputText && c.output != uicli.OutputJSON {
		refused := domain.InvalidMessage(`invalid --output "` + c.output + `": expected one of text, json`)
		c.output = uicli.OutputText
		return c.fail(cmd, refusal{Err: refused, Request: req})
	}
	ctx := cmd.Context()
	project, err := c.open(ctx)
	if err != nil {
		return c.fail(cmd, refusal{Err: domain.AsError(domain.AsErrorParams{Err: err}), Request: req})
	}
	_, stderr := c.streams(cmd)
	progress := output.NewProgress(ctx, output.ProgressParams{W: stderr, Human: c.human(), Live: output.IsTerminal(stderr)})
	ran, err := c.create(cmd, creating{UseCase: project.UseCase, Request: req, Report: progress.Report})
	progress.Close()
	return c.conclude(cmd, concluded{Outcome: ran.Outcome, Err: domain.AsError(domain.AsErrorParams{Err: err}), Request: ran.Request, Cwd: project.Cwd})
}

type creating struct {
	UseCase create.UseCase
	Request create.Request
	Report  domain.Report
}

func (c *createCommand) create(cmd *cobra.Command, params creating) (ran, error) {
	ctx := cmd.Context()
	if c.interactive(cmd) {
		return runForm(ctx, formParams{App: params.UseCase, Request: params.Request, DryRun: c.dryRun, Report: params.Report, In: cmd.InOrStdin(), Out: cmd.ErrOrStderr()})
	}
	outcome, err := params.UseCase.Run(ctx, params.Request, operation.Options{DryRun: c.dryRun, ExpectFingerprint: c.expect, Surface: uicli.SurfaceCLI, Report: params.Report})
	return ran{Outcome: outcome, Request: params.Request}, err
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

func (c *createCommand) interactive(cmd *cobra.Command) bool {
	stdin, ok := cmd.InOrStdin().(*os.File)
	return ok && term.IsTerminal(int(stdin.Fd())) && c.expect == "" && c.human() && !c.yes
}

func (c *createCommand) streams(cmd *cobra.Command) (io.Writer, io.Writer) {
	if c.quiet && c.human() {
		return io.Discard, io.Discard
	}
	return cmd.OutOrStdout(), cmd.ErrOrStderr()
}

type concluded struct {
	Outcome create.Outcome
	Err     *domain.Error
	Request create.Request
	Cwd     string
}

func (c *createCommand) conclude(cmd *cobra.Command, done concluded) error {
	if !c.human() {
		return c.envelope(cmd, envelopeParams{Outcome: done.Outcome, Err: done.Err})
	}
	stdout, _ := c.streams(cmd)
	line := createLine{Command: cmd.CommandPath(), Request: done.Request}
	switch {
	case done.Err != nil:
		return c.fail(cmd, refusal{Err: done.Err, Request: done.Request})
	case done.Outcome.Result == nil:
		apply := line.String() + " --expect-fingerprint " + done.Outcome.Fingerprint
		output.Frame(stdout, func(w io.Writer) { drawDryRun(w, dryRunParams{Outcome: done.Outcome, Apply: apply}) })
		return nil
	}
	followUps := itemFollowUps(done.Outcome.Result.Items)
	output.Frame(stdout, func(w io.Writer) {
		drawOutcome(w, outcomeParams{Outcome: done.Outcome, FollowUps: followUps, Cwd: done.Cwd})
	})
	return c.failed(cmp.Or(done.Err, done.Outcome.Failure()))
}

type refusal struct {
	Err     *domain.Error
	Request create.Request
}

func (c *createCommand) fail(cmd *cobra.Command, refused refusal) error {
	failure := refused.Err
	if !c.human() {
		return c.envelope(cmd, envelopeParams{Err: refused.Err})
	}
	_, stderr := c.streams(cmd)
	var next []output.NextStep
	if failure.Code == operation.CodePlanStale {
		next = []output.NextStep{{Command: createLine{Command: cmd.CommandPath(), Request: refused.Request}.String() + " --dry-run", Note: output.ReplanNote}}
	}
	output.Frame(stderr, func(w io.Writer) {
		output.Failure(w, output.FailureParams{Err: failure, Next: next, Flags: createFlags})
	})
	return c.failed(failure)
}

func (c *createCommand) envelope(cmd *cobra.Command, params envelopeParams) error {
	doc := envelopeOf(params)
	if err := output.Envelope(cmd.OutOrStdout(), doc); err != nil {
		return c.failed(domain.AsError(domain.AsErrorParams{Err: err}))
	}
	return c.failed(doc.Err)
}

func (c *createCommand) failed(failure *domain.Error) error {
	if failure == nil {
		return nil
	}
	return &uicli.Failed{Err: failure, Quiet: c.quiet && c.human(), Flags: createFlags, Exits: exits}
}

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
