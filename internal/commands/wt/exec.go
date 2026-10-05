package wt

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	execflow "github.com/LucasPcq/wtm/internal/flow/exec"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/shellcmd"
)

func newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdExec + " [worktree...] [-- <command>]",
		Short: "Run one command in several worktrees, in parallel",
		Long: "Run a shell line in each selected worktree, in parallel, and report which passed.\n" +
			"Everything after -- is run with /bin/sh -c from the worktree's root, with that\n" +
			"worktree's environment: the variables describing the worktree you stand in are\n" +
			"removed, and the target's run variables (compose project, shifted ports) are added\n" +
			"when it has them — the same ones its hooks get. stdin is closed. Each worktree's whole\n" +
			"output is kept in a log under the state directory; failures show its tail.\n" +
			"Pass worktree names (branches), --all, or nothing to pick interactively; without --\n" +
			"the wizard asks for the command too, and a run that cannot ask refuses. The run\n" +
			"exits 1 when any command failed; each worktree's own exit code is in the report.",
		Example: `  # Pick the worktrees and type the command in the wizard
  wtm exec

  # Run the tests on two branches
  wtm exec feat/login feat/signup -- pnpm test

  # Reinstall everywhere after a lockfile bump, two at a time
  wtm exec --all --jobs 2 -- pnpm install

  # Read each worktree's last commit
  wtm exec --all --yes --print -- git log -1 --oneline

  # For an agent
  wtm exec --all --yes --output json -- pnpm typecheck`,
		Args: cobra.ArbitraryArgs,
		RunE: runExec,
	}

	cmd.Flags().Bool(domain.FlagAll, false, "Run in every worktree, the main checkout included")
	cmd.Flags().Int(domain.FlagJobs, 0, "How many commands run at once (0: one per CPU)")
	cmd.Flags().Bool(domain.FlagPrint, false, "Also show the full output of every worktree, successes included")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts (requires worktree names or --all, and the command after --)")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runExec(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool(domain.FlagAll)
	jobs, _ := cmd.Flags().GetInt(domain.FlagJobs)
	printAll, _ := cmd.Flags().GetBool(domain.FlagPrint)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	split, err := rules.SplitExecArgs(rules.SplitExecArgsParams{Args: args, Dash: cmd.ArgsLenAtDash()})
	if err != nil {
		return fmt.Errorf("%w: %w", domain.ErrUsage, err)
	}
	names := split.Names
	if all && len(names) > 0 {
		return fmt.Errorf("%w: %w", domain.ErrUsage, domain.ErrExecAllWithNames)
	}
	if jobs < 0 {
		return fmt.Errorf("%w: --%s cannot be negative", domain.ErrUsage, domain.FlagJobs)
	}
	if format == domain.OutputJSON && !yes {
		return domain.ErrJSONNeedsYes
	}
	if err := checkExecLine(split.Command); err != nil {
		return err
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	config, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes
	if !interactive && !yes && len(names) == 0 && !all {
		return errors.New(domain.ExecNeedsTerminal)
	}

	workers := rules.ExecJobs(rules.ExecJobsParams{Requested: jobs, CPUs: runtime.NumCPU()})
	_, err = execflow.Run(cmd.Context(), execflow.Params{
		Context:   shared.FlowContext(config),
		Request:   execflow.Request{Branches: names, All: all, Command: split.Command, Jobs: workers, Print: printAll, Dir: dir},
		Prompter:  shared.FlowPrompter(shared.FlowPrompterParams{Interactive: interactive, Stderr: true}),
		Presenter: &execPresenter{CLIPresenter: shared.NewPresenter(cmd, format), print: printAll},
	})
	return err
}

// checkExecLine only checks a command given after --: one the wizard asks for
// is checked by its step.
func checkExecLine(line string) error {
	if line == "" {
		return nil
	}
	if err := shellcmd.CheckSyntax(line); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrUsage, err)
	}
	return nil
}
