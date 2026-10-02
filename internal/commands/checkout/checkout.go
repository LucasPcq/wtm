// Package checkout implements the wtm checkout command: create a worktree from
// an existing GitHub pull request.
package checkout

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	checkoutflow "github.com/LucasPcq/wtm/internal/flow/checkout"
	"github.com/LucasPcq/wtm/internal/rules"
)

// NewCmd creates the wtm checkout command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdCheckout + " [number]",
		Short: "Create a worktree from an existing pull request",
		Long: "Create a worktree from a pull request.\n" +
			"A local branch of the PR's name is checked out as-is, keeping commits you never\n" +
			"pushed; interactive runs offer to fast-forward it when it is behind origin (--ff\n" +
			"does it without asking).\n" +
			"Without arguments, shows an interactive picker of open PRs.",
		Example: `  # Pick among the open pull requests
  wtm checkout

  # Only the ones waiting for your review
  wtm checkout --review

  wtm checkout 42

  # No prompts, with a JSON result
  wtm checkout 42 --yes --output json`,
		Args: cobra.MaximumNArgs(1),
		RunE: runCheckout,
	}

	cmd.Flags().Bool(domain.FlagReview, false, "Show only PRs where you are requested as reviewer")
	cmd.Flags().Bool(domain.FlagMine, false, "Show only your PRs")
	cmd.Flags().String(domain.FlagFrom, "", "Parent branch for sync (defaults to the PR base branch)")
	cmd.Flags().String(domain.FlagEnvFrom, "", "Override env strategy (example, main, parent)")
	cmd.Flags().Bool(domain.FlagFF, false, "Fast-forward the PR's branch to origin when it already exists locally and is behind (non-interactive; skipped when it has diverged)")
	shared.AddIsolationFlag(cmd)
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (PR number required)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runCheckout(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	fromOverride, _ := cmd.Flags().GetString(domain.FlagFrom)
	envOverride, _ := cmd.Flags().GetString(domain.FlagEnvFrom)
	ffFlag, _ := cmd.Flags().GetBool(domain.FlagFF)
	review, _ := cmd.Flags().GetBool(domain.FlagReview)
	mine, _ := cmd.Flags().GetBool(domain.FlagMine)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	isolation, err := shared.IsolationFlag(cmd)
	if err != nil {
		return err
	}

	if format == domain.OutputJSON && !yes {
		return fmt.Errorf("--output json requires --%s (prompts cannot run in JSON mode)", domain.FlagYes)
	}

	number := 0
	if len(args) == 1 {
		parsed, parseErr := strconv.Atoi(args[0])
		if parseErr != nil || parsed <= 0 {
			return fmt.Errorf("invalid PR number %q", args[0])
		}
		number = parsed
	}

	interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes

	_, err = checkoutflow.Run(checkoutflow.Params{
		Context: shared.FlowContext(result),
		Request: checkoutflow.Request{
			Number:      number,
			Filter:      rules.PRFilterFor(rules.PRFilterParams{Review: review, Mine: mine}),
			From:        fromOverride,
			EnvFrom:     envOverride,
			FastForward: ffFlag,
			Isolation:   isolation,
		},
		Prompter:  shared.FlowPrompter(shared.FlowPrompterParams{Interactive: interactive}),
		Presenter: checkoutPresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	return err
}
