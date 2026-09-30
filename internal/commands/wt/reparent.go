package wt

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	reparentflow "github.com/LucasPcq/wtm/internal/flow/reparent"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newReparentCmd creates the wtm reparent subcommand.
func newReparentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdReparent + " [branch...]",
		Short: "Change the parent one or more worktrees are rebased onto",
		Long: "Change the recorded parent (source branch) of one or more worktrees. Only the\n" +
			"metadata is updated — the rebase happens on the next `wtm sync`. Pass the worktrees\n" +
			"and --to <parent>, or run with no arguments to multi-select interactively. The new\n" +
			"parent must exist as a local or origin remote-tracking branch (origin/x), and the\n" +
			"resulting parent chain must stay acyclic.",
		Example: `  # Pick the worktrees and their new parent
  wtm reparent

  # feat/login was merged: stack its child on main, then rebase it
  wtm reparent feat/login-ui --to main --yes
  wtm sync feat/login-ui`,
		Args: cobra.ArbitraryArgs,
		RunE: runReparent,
	}

	cmd.Flags().String(domain.FlagTo, "", "New parent branch to rebase onto")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (needs at least one worktree and --to)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runReparent(cmd *cobra.Command, args []string) error {
	to, _ := cmd.Flags().GetString(domain.FlagTo)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON && !yes {
		return fmt.Errorf("--output json requires --%s (the confirmation cannot run in JSON mode)", domain.FlagYes)
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	config, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	// The one place --yes is read: which Prompter gets installed.
	interactive := rules.IsHumanFormat(format) && !yes && term.IsTerminal(int(os.Stdin.Fd()))

	_, err = reparentflow.Run(reparentflow.Params{
		Context:   shared.FlowContext(config),
		Request:   reparentflow.Request{Branches: args, To: to},
		Prompter:  shared.FlowPrompter(shared.FlowPrompterParams{Interactive: interactive, Stderr: true}),
		Presenter: reparentPresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	return err
}
