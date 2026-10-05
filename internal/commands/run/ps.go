package run

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
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
		Example: `  wtm run ps

  wtm run ps --output json`,
		RunE: runPs,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runPs(cmd *cobra.Command, _ []string) error {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON {
		jobs := rules.JobsByWorktree(shared.LoadJobs(cmd.Context()).Jobs)
		return output.WriteRunningJobsJSON(cmd.OutOrStdout(), runningJobs(cmd.Context(), runningJobsParams{Jobs: jobs, Held: runjobs.Held(cmd.Context(), jobs)}))
	}

	var listing runjobs.Listing
	var held domain.HeldAddresses
	loadErr := components.RunLoading(components.LoadingParams{
		Message: domain.RunLoadingJobs,
		Animate: shared.Animate(cmd, true),
		Work: func() error {
			listing = shared.LoadJobs(cmd.Context())
			held = runjobs.Held(cmd.Context(), listing.Jobs)
			return nil
		},
	})
	if loadErr != nil {
		return loadErr
	}

	out := cmd.OutOrStdout()
	jobs := rules.JobsByWorktree(listing.Jobs)
	output.Frame(out, func(w io.Writer) {
		fmt.Fprint(w, output.FormatRunningJobs(output.FormatRunningJobsParams{
			Jobs:       jobs,
			Now:        time.Now(),
			Branches:   branchesOf(cmd.Context(), jobs),
			Projects:   projectsOf(cmd.Context(), jobs),
			Held:       held,
			Hyperlinks: output.IsTerminal(out),
		}))
		if listing.Diverged() {
			output.Blank(w)
			output.Warning(w, fmt.Sprintf(domain.RunDaemonDivergedFmt, rules.DaemonVersionLabel(listing.DaemonVersion), domain.Version))
		}
	})
	return nil
}

// branchesOf names each work dir once: a table of eight jobs in two worktrees
// asks git twice, not eight times.
func branchesOf(ctx context.Context, jobs []domain.JobInfo) map[string]string {
	branches := map[string]string{}
	for _, job := range jobs {
		if _, seen := branches[job.WorkDir]; seen {
			continue
		}
		branches[job.WorkDir] = target.BranchOf(ctx, job.WorkDir)
	}
	return branches
}

// projectsOf names each work dir's repository, nil when they all belong to one:
// the daemon is machine-wide, and "main" alone is ambiguous across two repos.
func projectsOf(ctx context.Context, jobs []domain.JobInfo) map[string]string {
	projects := map[string]string{}
	for _, job := range jobs {
		if _, seen := projects[job.WorkDir]; seen {
			continue
		}
		projects[job.WorkDir] = target.ProjectOf(ctx, job.WorkDir)
	}
	if rules.DistinctValues(projects) < 2 {
		return nil
	}
	return projects
}

// runningJobs is the document `run ps --output json` writes: every row names
// its worktree by branch and path, and its project even when there is one.
type runningJobsParams struct {
	Jobs []domain.JobInfo
	Held domain.HeldAddresses
}

func runningJobs(ctx context.Context, params runningJobsParams) []domain.RunningJob {
	jobs := params.Jobs
	branches := branchesOf(ctx, jobs)
	projects := map[string]string{}
	rows := make([]domain.RunningJob, 0, len(jobs))
	for _, job := range jobs {
		project, seen := projects[job.WorkDir]
		if !seen {
			project = target.ProjectOf(ctx, job.WorkDir)
			projects[job.WorkDir] = project
		}
		rows = append(rows, domain.RunningJob{
			Name:      job.Name,
			Kind:      job.Kind,
			Status:    job.Status,
			PID:       job.PID,
			Branch:    branches[job.WorkDir],
			Path:      job.WorkDir,
			Project:   project,
			StartedAt: job.StartedAt,
			URL:       job.URL,
			ExitCode:  job.ExitCode,
			Held:      params.Held[job.WorkDir][job.Name],
		})
	}
	return rows
}
