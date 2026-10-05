package run

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	openflow "github.com/LucasPcq/wtm/internal/flow/run/open"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/integration"
)

// openInBrowser is a variable so a test can watch what a run would have opened
// without a browser window appearing.
var openInBrowser = integration.OpenURL

func newOpenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdOpen + " [worktree]",
		Short: "Open a job's URL in the browser",
		Long:  "Hand a job's URL to the desktop's own opener. [worktree] defaults to the current one, and is picked interactively when there is a terminal. A worktree publishing one URL opens it; when several jobs publish one, --job names it, and is required outside a fully interactive run — a picker never runs under a pipe, under --yes or in --output json mode.",
		Example: `  wtm run open

  wtm run open feat/login --job web

  # The port URL instead of the named one
  wtm run open feat/login --job web --raw`,
		Args: cobra.MaximumNArgs(1),
		RunE: runOpen,
	}
	shared.AddJobFlag(cmd, "Job whose URL to open (required when several jobs publish one, outside a fully interactive run)")
	cmd.Flags().Bool(domain.FlagRaw, false, "Open the port URL (http://localhost:<port>) instead of the named URL")
	shared.AddYesFlag(cmd, "Skip the pickers; --job is then required when several jobs publish a URL")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runOpen(cmd *cobra.Command, args []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}

	raw, _ := cmd.Flags().GetBool(domain.FlagRaw)
	job, _ := cmd.Flags().GetString(domain.FlagJob)

	outcome, err := openflow.Run(cmd.Context(), openflow.Params{
		Context: ctx.FlowContext(),
		Request: openflow.Request{
			Worktree: runctx.FirstArg(args),
			Cwd:      ctx.Dir,
			Job:      job,
			Raw:      raw,
			Config:   ctx.Run,
		},
		Prompter:  ctx.Prompter(ctx.Interactive),
		Presenter: openPresenter{CLIPresenter: ctx.CLI(cmd)},
		Open:      openInBrowser,
	})
	if err != nil {
		return err
	}
	if outcome.Aborted {
		return shared.EndAborted(cmd)
	}
	// The shape follows the format, as it does for `run url`: a caller asking
	// for a document gets the address that was opened, not an empty stdout.
	if ctx.Format == domain.OutputJSON {
		return outputJobURL(cmd, outcome.Entry)
	}
	return nil
}

// openPresenter keeps the addressing warning out of a machine run: a caller
// asking for JSON asked for a document, not for prose on stderr.
type openPresenter struct {
	shared.CLIPresenter
}

func (p openPresenter) Status(notice flow.Notice) {
	if !rules.IsHumanFormat(p.Format) {
		return
	}
	p.CLIPresenter.Status(notice)
}
