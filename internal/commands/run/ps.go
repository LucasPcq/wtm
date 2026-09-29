package run

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// newPsCmd creates the wtm run ps subcommand.
func newPsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdPs,
		Short: "List currently running jobs",
		Long: "Show the jobs managed by the background daemon (name, kind, status, address, uptime, worktree).\n" +
			"It lists every repository the daemon knows, so it works from anywhere — inside a\n" +
			"run-initialized repository or not.\n" +
			"To act on those jobs, open the run view with `wtm run logs`, which covers as many\n" +
			"worktrees as you select.",
		RunE: runPs,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runPs(cmd *cobra.Command, _ []string) error {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON {
		jobs, err := shared.LoadJobs()
		if err != nil {
			return err
		}
		return output.WriteRunningJobsJSON(cmd.OutOrStdout(), jobs)
	}

	var jobs []domain.JobInfo
	loadErr := components.RunLoading(components.LoadingParams{
		Message: "Loading jobs…",
		Animate: shared.Animate(cmd, true),
		Work: func() error {
			var e error
			jobs, e = shared.LoadJobs()
			return e
		},
	})
	if loadErr != nil {
		return loadErr
	}

	out := cmd.OutOrStdout()
	output.Frame(out, func(w io.Writer) {
		fmt.Fprint(w, output.FormatRunningJobs(output.FormatRunningJobsParams{Jobs: jobs, Now: time.Now(), Branches: branchesOf(jobs), Hyperlinks: output.IsTerminal(out)}))
	})
	return nil
}

// branchesOf names each work dir once: a table of eight jobs in two worktrees
// asks git twice, not eight times.
func branchesOf(jobs []domain.JobInfo) map[string]string {
	branches := map[string]string{}
	for _, job := range jobs {
		if _, seen := branches[job.WorkDir]; seen {
			continue
		}
		branches[job.WorkDir] = target.BranchOf(job.WorkDir)
	}
	return branches
}
