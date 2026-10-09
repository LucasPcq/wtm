package run

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	urlflow "github.com/LucasPcq/wtm/internal/flow/run/url"
	"github.com/LucasPcq/wtm/internal/output"
)

func newURLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         domain.CmdURL + " [worktree]",
		Annotations: map[string]string{domain.AnnotationMachineOutput: domain.AnnotationOn},
		Short:       "Print where a job is reachable in a worktree",
		Long:        "Write a job's URL on stdout and nothing else, for $(…). [worktree] defaults to the current one, and no picker ever opens here — an ambiguity is an error naming --job. The URL is the named URL the proxy serves (http://api.feat-x.myrepo.localhost); --raw prints the port URL instead (http://localhost:<port>), which every OS resolves and no proxy has to serve.",
		Example: `  wtm run url --job api

  curl "$(wtm run url feat/login --job api)/health"

  # The port URL, which needs no proxy
  wtm run url feat/login --job api --raw`,
		Args: cobra.MaximumNArgs(1),
		RunE: runURL,
	}
	shared.AddJobFlag(cmd, "Job whose URL to print (required when several jobs publish one)")
	cmd.Flags().Bool(domain.FlagRaw, false, "Print the port URL (http://localhost:<port>) instead of the named URL")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runURL(cmd *cobra.Command, args []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}

	raw, _ := cmd.Flags().GetBool(domain.FlagRaw)
	job, _ := cmd.Flags().GetString(domain.FlagJob)

	outcome, err := urlflow.Run(cmd.Context(), urlflow.Params{
		Context: ctx.FlowContext(),
		Request: urlflow.Request{
			Worktree: runctx.FirstArg(args),
			Cwd:      ctx.Dir,
			Job:      job,
			Raw:      raw,
			Config:   ctx.Run,
		},
	})
	if err != nil {
		return err
	}

	// The shape follows the format: a document lists what the worktree
	// publishes — narrowed by --job as the line is — where a line is one address
	// and therefore has to be unambiguous.
	if ctx.Format == domain.OutputJSON {
		return output.WriteJobURLsJSON(cmd.OutOrStdout(), outcome.Entries)
	}

	entry, err := outcome.One()
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), entry.URL)
	return nil
}

// outputJobURL writes one address as the document `run url` writes for a set of
// them: one shape for one question, whichever command was asked it.
func outputJobURL(cmd *cobra.Command, entry domain.JobURLEntry) error {
	return output.WriteJobURLsJSON(cmd.OutOrStdout(), []domain.JobURLEntry{entry})
}
