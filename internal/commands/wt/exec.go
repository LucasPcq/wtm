package wt

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
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
		Use:   domain.CmdExec + " [worktree...] -- <command>",
		Short: "Run one command in several worktrees, in parallel",
		Long: "Run a shell line in each selected worktree, in parallel, and report which passed.\n" +
			"Everything after -- is run with /bin/sh -c from the worktree's root, with that\n" +
			"worktree's environment: the variables describing the worktree you stand in are\n" +
			"removed, and the target's run variables (compose project, shifted ports) are added\n" +
			"when it has them — the same ones its hooks get. stdin is closed. Each worktree's whole\n" +
			"output is kept in a log under the state directory; failures show its tail.\n" +
			"Pass worktree names (branches), --all, or nothing to pick interactively. The run\n" +
			"exits 1 when any command failed; each worktree's own exit code is in the report.",
		Example: `  # Run the tests on two branches
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
	cmd.Flags().Int(domain.FlagJobs, runtime.NumCPU(), "How many commands run at once")
	cmd.Flags().Bool(domain.FlagPrint, false, "Also show the full output of every worktree, successes included")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts (requires worktree names or --all)")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runExec(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool(domain.FlagAll)
	jobs, _ := cmd.Flags().GetInt(domain.FlagJobs)
	printAll, _ := cmd.Flags().GetBool(domain.FlagPrint)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	dash := cmd.ArgsLenAtDash()
	if dash < 0 {
		return fmt.Errorf("%w: %w", domain.ErrUsage, domain.ErrExecNoCommand)
	}
	names := args[:dash]
	line, err := rules.ExecCommandLine(args[dash:])
	if err != nil {
		return fmt.Errorf("%w: %w", domain.ErrUsage, err)
	}
	if all && len(names) > 0 {
		return fmt.Errorf("%w: %w", domain.ErrUsage, domain.ErrExecAllWithNames)
	}
	if jobs < 1 {
		return fmt.Errorf("%w: --%s must be at least 1", domain.ErrUsage, domain.FlagJobs)
	}
	if format == domain.OutputJSON && !yes {
		return fmt.Errorf("--output json requires --%s (prompts cannot run in JSON mode)", domain.FlagYes)
	}
	if err := shellcmd.CheckSyntax(line); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrUsage, err)
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
	if !interactive && !yes {
		return errors.New(domain.ExecNeedsTerminal)
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()

	_, err = execflow.Run(execflow.Params{
		Ctx:       ctx,
		Context:   shared.FlowContext(config),
		Request:   execflow.Request{Branches: names, All: all, Command: line, Jobs: jobs, Print: printAll, Dir: dir},
		Prompter:  shared.FlowPrompter(shared.FlowPrompterParams{Interactive: interactive, Stderr: true}),
		Presenter: &execPresenter{CLIPresenter: shared.NewPresenter(cmd, format), print: printAll},
	})
	return err
}
