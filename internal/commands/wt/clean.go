package wt

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	cleanflow "github.com/LucasPcq/wtm/internal/flow/clean"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newCleanCmd creates the wtm clean subcommand.
func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdClean + " [branch]",
		Short: "Remove a worktree and its local branch",
		Long:  "Remove a git worktree and delete the local branch. The remote branch is never touched.\nWithout arguments, shows an interactive picker.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runClean,
	}

	cmd.Flags().Bool(domain.FlagForce, false, "Lift safety refusals (dirty/unpushed/open-PR); still asks to confirm unless --yes")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (keeps safety checks unless --force)")
	cmd.Flags().Bool(domain.FlagReparentChildren, false, "Reparent orphaned child worktrees onto the grandparent (no prompt)")
	cmd.Flags().Bool(domain.FlagKeepData, false, domain.FlagKeepDataDesc)
	shared.AddOutputFlag(cmd)

	return cmd
}

func runClean(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	reparentFlag, _ := cmd.Flags().GetBool(domain.FlagReparentChildren)
	keepData, _ := cmd.Flags().GetBool(domain.FlagKeepData)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON && !yes {
		return domain.ErrCleanJSONNeedsYes
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	config, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	branchName := ""
	if len(args) > 0 {
		branchName = args[0]
	}

	// --force is the safety axis, not a confirmation bypass: it still runs the wizard,
	// with the refusals lifted.
	interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes

	_, err = cleanflow.Run(cleanflow.Params{
		Context: shared.FlowContext(config),
		Request: cleanflow.Request{
			Branch:           branchName,
			Force:            force,
			ReparentChildren: reparentFlag,
			KeepData:         keepData,
			BaseBranch:       resolveBase("", config),
			// The CLI owns the terminal it prompts on, so it can hand it to sudo.
			AllowPrivileged: true,
		},
		// The picker may be reached through the shell wrapper, which consumes stdout.
		Prompter:  shared.FlowPrompter(shared.FlowPrompterParams{Interactive: interactive, Stderr: true}),
		Presenter: cleanPresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	return err
}
