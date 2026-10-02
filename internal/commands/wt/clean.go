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
		Use:   domain.CmdClean + " [branch...]",
		Short: "Remove worktrees and their local branches",
		Long: "Remove git worktrees and delete their local branches. The remote branch is never touched.\n" +
			"Without arguments, shows an interactive picker where several can be checked.\n" +
			"\n" +
			"Several worktrees are removed one after the other: a failure does not stop the others, and\n" +
			"the run exits with the first failure's code. Under --yes, one unsafe worktree (dirty,\n" +
			"unpushed, open PR) refuses the whole run before anything is removed, unless --force.\n" +
			"\n" +
			"The removal runs in a fixed order: the worktree's jobs are stopped and checked gone (a job\n" +
			"that will not stop refuses the removal unless --force), the on_clean hooks run, git removes\n" +
			"the worktree — and only then is its data dropped. A failure before that last step leaves\n" +
			"the data where it was.\n" +
			"\n" +
			"By default, clean DROPS the namespaces the worktree carved out of shared services (a\n" +
			"database per worktree in a shared postgres, say): the confirmation names each one, and\n" +
			"--output json reports each as dropped, deferred or kept. --keep-data withholds the drop. A\n" +
			"service that is down cannot take its data back: the form asks whether to start it now or\n" +
			"keep the data until wtm next starts it; --yes keeps it, --drop-data starts it. A namespace\n" +
			"another worktree reaches under the same name is never dropped.",
		Example: `  # Pick the worktrees to remove
  wtm clean

  wtm clean feat/login

  # Several at once, no prompts; their children move onto the nearest survivor
  wtm clean feat/login feat/signup --yes --reparent-children

  # Keep the databases it holds in shared services
  wtm clean feat/login --yes --keep-data --output json`,
		Args: cobra.ArbitraryArgs,
		RunE: runClean,
	}

	cmd.Flags().Bool(domain.FlagForce, false, "Lift safety refusals (dirty/unpushed/open-PR); still asks to confirm unless --yes")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (keeps safety checks unless --force)")
	cmd.Flags().Bool(domain.FlagReparentChildren, false, domain.FlagReparentChildrenDesc)
	cmd.Flags().Bool(domain.FlagKeepData, false, domain.FlagKeepDataDesc)
	cmd.Flags().Bool(domain.FlagDropData, false, domain.FlagDropDataDesc)
	cmd.MarkFlagsMutuallyExclusive(domain.FlagKeepData, domain.FlagDropData)
	shared.AddOutputFlag(cmd)

	return cmd
}

func runClean(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	reparentFlag, _ := cmd.Flags().GetBool(domain.FlagReparentChildren)
	keepData, _ := cmd.Flags().GetBool(domain.FlagKeepData)
	dropData, _ := cmd.Flags().GetBool(domain.FlagDropData)
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

	// --force is the safety axis, not a confirmation bypass: it still runs the wizard,
	// with the refusals lifted.
	interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes

	_, err = cleanflow.Run(cleanflow.Params{
		Context: shared.FlowContext(config),
		Request: cleanflow.Request{
			Branches:         args,
			Force:            force,
			ReparentChildren: reparentFlag,
			KeepData:         keepData,
			DropData:         dropData,
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
