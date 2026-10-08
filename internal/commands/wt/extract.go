package wt

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	extractflow "github.com/LucasPcq/wtm/internal/flow/extract"
	"github.com/LucasPcq/wtm/internal/rules"
)

func newExtractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdExtract + " [source]",
		Short: "Move uncommitted changes to another worktree",
		Long: "Move a subset of a worktree's uncommitted changes to another worktree\n" +
			"(new or existing) to split an oversized PR or isolate unrelated work.\n\n" +
			"The source worktree is the first thing chosen: pass its branch as [source],\n" +
			"or omit it to pick interactively from the worktrees that have changes. A source\n" +
			"is required when there is no terminal or with --output json.\n\n" +
			"A --to branch that already exists locally is checked out as-is, keeping its\n" +
			"commits. Its parent can't be inferred, so --from then names the branch recorded\n" +
			"for `wtm sync` — asked in the wizard, required without it.\n\n" +
			"Untracked files are listed one by one, including inside brand-new directories,\n" +
			"so you can take part of a new folder; gitignored files are never listed.\n\n" +
			"On conflict it aborts by default, leaving the source intact; --on-conflict resolve\n" +
			"applies conflict markers in the target so you can resolve them like a rebase.\n" +
			"A file that merely already exists in the target counts as a conflict too.",
		Example: `  # Pick the source, the files and the target
  wtm extract

  # Move a directory's changes to a new branch
  wtm extract feat/login --files apps/api --to feat/login-api --yes

  # Copy one file instead, onto a branch stacked on the source
  wtm extract feat/login --files apps/web/login.ts --to feat/login-web --from feat/login --keep --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: runExtract,
	}

	cmd.Flags().StringSlice(domain.FlagFiles, nil, "Files to extract, or a directory to take everything below it (skips interactive selection)")
	cmd.Flags().String(domain.FlagTo, "", "Target worktree branch; created if it does not exist")
	shared.AddIsolationFlag(cmd)
	cmd.Flags().String(domain.FlagFrom, "", "Parent branch when creating the target worktree")
	cmd.Flags().Bool(domain.FlagFF, false, "Fast-forward the parent branch to origin before creating the target (non-interactive; skipped when it has diverged)")
	cmd.Flags().Bool(domain.FlagKeep, false, "Copy instead of move (keep the changes in the source)")
	cmd.Flags().String(domain.FlagOnConflict, "", "On conflict: abort (default) or resolve (write conflict markers in the target)")
	shared.AddAskFlag(cmd)
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (requires a source arg, --files and --to; --from is also required when --to already exists locally; errors if a selection is missing)")
	shared.AddOutputFlag(cmd)
	shared.RequireYesInJSON(cmd)

	return cmd
}

func runExtract(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	config, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	onConflict, err := onConflictFlag(cmd)
	if err != nil {
		return err
	}
	isolation, err := shared.IsolationFlag(cmd)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)

	files, _ := cmd.Flags().GetStringSlice(domain.FlagFiles)
	to, _ := cmd.Flags().GetString(domain.FlagTo)
	from, _ := cmd.Flags().GetString(domain.FlagFrom)
	keep, _ := cmd.Flags().GetBool(domain.FlagKeep)
	ff, _ := cmd.Flags().GetBool(domain.FlagFF)
	ask, _ := cmd.Flags().GetBool(domain.FlagAsk)
	source := ""
	if len(args) == 1 {
		source = args[0]
	}

	interactive := rules.IsHumanFormat(format) && !yes && term.IsTerminal(int(os.Stdin.Fd()))

	_, err = extractflow.Run(cmd.Context(), extractflow.Params{
		Context: shared.FlowContext(config),
		Request: extractflow.Request{
			Source:      source,
			Files:       files,
			To:          to,
			From:        from,
			Keep:        keep,
			KeepSet:     cmd.Flags().Changed(domain.FlagKeep),
			FastForward: ff,
			OnConflict:  onConflict,
			Isolation:   isolation,
			Ask:         ask,
		},
		Prompter:  shared.FlowPrompter(cmd.Context(), shared.FlowPrompterParams{Interactive: interactive}),
		Presenter: extractPresenter{CLIPresenter: shared.NewPresenter(cmd, format), config: config},
	})
	return err
}

func onConflictFlag(cmd *cobra.Command) (string, error) {
	mode, _ := cmd.Flags().GetString(domain.FlagOnConflict)
	if !cmd.Flags().Changed(domain.FlagOnConflict) {
		return "", nil
	}
	if mode != domain.OnConflictAbort && mode != domain.OnConflictResolve {
		return "", rules.InvalidFlagValue(rules.InvalidFlagValueParams{
			Flag:    domain.FlagOnConflict,
			Value:   mode,
			Allowed: []string{domain.OnConflictAbort, domain.OnConflictResolve},
		})
	}
	return mode, nil
}
