package wt

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	statusflow "github.com/LucasPcq/wtm/internal/flow/status"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStatus + " [worktree]",
		Short: "Show a worktree's whole state: isolation, env files, jobs and what to fix",
		Long: "Read one worktree's state in one document: its branch and path, how it runs\n" +
			"(isolation, addressing, port offset), which declared .env files it lacks, every\n" +
			"job run.toml declares with its state (the states `wtm events` reports) and its\n" +
			"address, and each problem found with the exact command that fixes it.\n\n" +
			"[worktree] defaults to the current one. It changes nothing, asks nothing, never\n" +
			"starts the run daemon (with none running it reads the job index and checks the\n" +
			"processes itself) and needs no --yes. No .env value appears in its output.\n" +
			"It exits 0 whatever it finds: read `problems`.",
		Example: `  wtm status

  wtm status feat/login --output json

  # Every worktree, one line each, problems expanded
  wtm status --all

  # The commands that fix what it found
  wtm status --output json | jq -r '.problems[].fix'`,
		Args: statusArgs,
		RunE: runStatus,
	}
	cmd.Flags().Bool(domain.FlagAll, false, "Read every worktree of the repository (JSON: an array of the same documents)")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	all, _ := cmd.Flags().GetBool(domain.FlagAll)
	params := statusflow.Params{
		Context: shared.FlowContext(cfg),
		Request: statusflow.Request{Worktree: runctx.FirstArg(args), Cwd: dir},
	}
	if all {
		return reportStatusAll(cmd, statusReport{Params: params, Format: format, ProjectDir: cfg.ProjectDir})
	}
	return reportStatus(cmd, statusReport{Params: params, Format: format, ProjectDir: cfg.ProjectDir})
}

func statusArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
		return err
	}
	all, _ := cmd.Flags().GetBool(domain.FlagAll)
	if all && len(args) > 0 {
		return rules.Usage(errors.New(domain.StatusAllWithWorktreeMsg))
	}
	return nil
}

type statusReport struct {
	Params     statusflow.Params
	Format     string
	ProjectDir string
}

func reportStatus(cmd *cobra.Command, report statusReport) error {
	var doc domain.StatusDocument
	err := components.RunLoading(components.LoadingParams{
		Message: domain.StatusLoading,
		Animate: shared.Animate(cmd, rules.IsHumanFormat(report.Format)),
		Work: func() error {
			var runErr error
			doc, runErr = statusflow.Run(report.Params)
			return runErr
		},
	})
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if report.Format == domain.OutputJSON {
		return output.WriteStatusJSON(out, doc)
	}
	output.Frame(out, func(w io.Writer) {
		output.FormatStatus(w, output.FormatStatusParams{
			Document:   doc,
			ProjectDir: report.ProjectDir,
			Hyperlinks: output.IsTerminal(out),
		})
	})
	return nil
}

func reportStatusAll(cmd *cobra.Command, report statusReport) error {
	var docs []domain.StatusDocument
	err := components.RunLoading(components.LoadingParams{
		Message: domain.StatusLoading,
		Animate: shared.Animate(cmd, rules.IsHumanFormat(report.Format)),
		Work: func() error {
			var runErr error
			docs, runErr = statusflow.RunAll(report.Params)
			return runErr
		},
	})
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if report.Format == domain.OutputJSON {
		return output.WriteStatusAllJSON(out, docs)
	}
	output.Frame(out, func(w io.Writer) {
		output.FormatStatusAll(w, docs)
	})
	return nil
}
