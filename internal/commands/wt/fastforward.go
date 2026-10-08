package wt

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	ffflow "github.com/LucasPcq/wtm/internal/flow/fastforward"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newFastForwardCmd creates the wtm fast-forward subcommand.
func newFastForwardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     domain.CmdFastForward + " [branch...]",
		Aliases: []string{domain.CmdFastForwardAlias},
		Short:   "Advance worktree branches to their origin counterpart",
		Long: "Fast-forward one or more managed worktrees to origin/<branch>, and nothing else:\n" +
			"no rebase onto the parent, no merge. Pass branch names, --all for every worktree, or\n" +
			"no arguments to pick interactively. A branch that has diverged from origin is refused —\n" +
			"`wtm sync` is the command that replays local commits onto it, and --force does not lift\n" +
			"that refusal. A worktree with uncommitted changes is refused too; --force fast-forwards\n" +
			"it anyway, and git still refuses if a modified file would be overwritten.",
		Example: `  # Pick the worktrees to bring up to origin
  wtm fast-forward

  wtm ff feat/login

  # Every worktree, no prompts
  wtm fast-forward --all --yes --output json`,
		Args: cobra.ArbitraryArgs,
		RunE: runFastForward,
	}

	cmd.Flags().Bool(domain.FlagAll, false, "Fast-forward every managed worktree")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (requires branch args or --all)")
	cmd.Flags().Bool(domain.FlagForce, false, "Fast-forward a worktree that has uncommitted changes")
	shared.AddOutputFlag(cmd)
	shared.RequireYesInJSON(cmd)

	return cmd
}

func runFastForward(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool(domain.FlagAll)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if all && len(args) > 0 {
		return rules.Usage(fmt.Errorf("--%s cannot be combined with branch arguments", domain.FlagAll))
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	config, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	// The one place --yes is read: which Prompter gets installed. --force is a
	// business input and rides in the Request, so it lifts the refusal without
	// also skipping the question.
	interactive := rules.IsHumanFormat(format) && !yes && term.IsTerminal(int(os.Stdin.Fd()))

	_, err = ffflow.Run(cmd.Context(), ffflow.Params{
		Context: shared.FlowContext(config),
		Request: ffflow.Request{
			Branches: args,
			All:      all,
			Force:    force,
		},
		// The picker may be reached through the shell wrapper, which consumes stdout.
		Prompter:  shared.FlowPrompter(cmd.Context(), shared.FlowPrompterParams{Interactive: interactive, Stderr: true}),
		Presenter: ffPresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	return err
}
