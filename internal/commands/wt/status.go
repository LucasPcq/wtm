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
	"github.com/LucasPcq/wtm/internal/flow"
	statusflow "github.com/LucasPcq/wtm/internal/flow/status"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStatus + " [worktree]",
		Short: "Show a worktree's whole state: isolation, env files, jobs and what to fix",
		Long: "Read one worktree's state in one document: its branch and path, how it runs\n" +
			"(isolation, addressing, port offset), which declared .env files it lacks, every\n" +
			"job run.toml declares with its state (the states `wtm events` reports) and its\n" +
			"address, and each problem found with the exact command that fixes it.\n\n" +
			"Without [worktree], a terminal opens the worktree picker on the current one;\n" +
			"anywhere else (no terminal, --output json, --quiet, --yes) it reads the current one and\n" +
			"asks nothing. It changes nothing, never starts the run daemon (with none running\n" +
			"it reads the job index and checks the processes itself) and needs no --yes.\n" +
			"No .env value appears in its output.\n" +
			"It exits 0 whatever it finds: read `problems`.",
		Example: `  wtm status

  wtm status feat/login --output json

  # Every worktree as a table, then each problem with its fix
  wtm status --all

  # The commands that fix what it found
  wtm status --output json | jq -r '.problems[].fix'`,
		Args: statusArgs,
		RunE: runStatus,
	}
	cmd.Flags().Bool(domain.FlagAll, false, "Read every worktree of the repository (JSON: an array of the same documents)")
	shared.AddYesFlag(cmd, "Skip the worktree picker: read the current worktree (never required, JSON included)")
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
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	interactive := shared.Interactive(shared.UnattendedParams{TTY: runctx.IsTTY(), Format: format, Yes: yes}) && shared.Animate(cmd, true)
	params := statusflow.Params{
		Context:   shared.FlowContext(cfg),
		Request:   statusflow.Request{Worktree: runctx.FirstArg(args), Cwd: dir},
		Prompter:  statusPrompter(interactive),
		Presenter: shared.NewPresenter(cmd, format),
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

// statusPrompter is the run module's picker in a fully interactive run, and
// Unattended — the current worktree, no question — everywhere else. A var so a
// test can stand in for the terminal.
var statusPrompter = func(interactive bool) flow.Prompter {
	return shared.FlowPrompter(shared.FlowPrompterParams{Interactive: interactive, Stderr: true})
}

func reportStatus(cmd *cobra.Command, report statusReport) error {
	outcome, err := statusflow.Run(report.Params)
	if err != nil {
		return err
	}
	// The abort notice marked the command: returning nil lets the root end on
	// the cancelled code, which an error here would turn into a plain failure.
	if outcome.Aborted {
		return nil
	}

	out := cmd.OutOrStdout()
	if report.Format == domain.OutputJSON {
		return output.WriteStatusJSON(out, outcome.Document)
	}
	output.Frame(out, func(w io.Writer) {
		output.FormatStatus(w, output.FormatStatusParams{
			Document:   outcome.Document,
			ProjectDir: report.ProjectDir,
			Hyperlinks: output.IsTerminal(out),
		})
	})
	return nil
}

func reportStatusAll(cmd *cobra.Command, report statusReport) error {
	docs, err := statusflow.RunAll(report.Params)
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
