package wt

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// newTreeCmd creates the wtm tree subcommand.
func newTreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdTree,
		Short: "Show the worktree forest (parent → child)",
		Long: "Render the forest of managed worktrees, parents above their children, with the\n" +
			"orchestration signals that matter for a stacked-branch workflow: commits ahead\n" +
			"(↑N), uncommitted changes (⚠ dirty), and \"needs sync\" when a parent has moved and\n" +
			"the child must be rebased. Parents with no worktree appear as greyed virtual roots.\n" +
			"\n" +
			"--with-prs adds PR numbers and merged/closed markers (fetched eagerly). --output\n" +
			"json emits the structured tree for agents; --output mermaid emits a flowchart to\n" +
			"paste into a PR or Notion.",
		RunE: runTree,
	}

	shared.AddOutputFlag(cmd)
	cmd.Flags().Bool(domain.FlagWithPRs, false, "Include GitHub PR info (open/merged/closed; fetched eagerly)")

	return cmd
}

func runTree(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	withPRs, _ := cmd.Flags().GetBool(domain.FlagWithPRs)

	var forest domain.Forest
	err = components.RunLoading(components.LoadingParams{
		Message: "Building worktree tree…",
		Animate: shared.Animate(cmd, rules.IsHumanFormat(format)),
		Work: func() error {
			var prs []domain.PRInfo
			if withPRs {
				prs = shared.LoadPRsAllStatesGraceful(cfg.ProjectDir)
			}
			var e error
			forest, e = worktree.BuildTree(worktree.BuildTreeParams{
				ProjectDir: cfg.ProjectDir,
				StateDir:   cfg.StateDir,
				Config:     cfg.Config,
				PRs:        prs,
			})
			return e
		},
	})
	if err != nil {
		return fmt.Errorf("build worktree tree: %w", err)
	}

	switch format {
	case domain.OutputJSON:
		return output.WriteTreeJSON(cmd.OutOrStdout(), forest)
	case domain.OutputMermaid:
		return output.WriteTreeMermaid(cmd.OutOrStdout(), forest)
	default:
		output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			fmt.Fprintln(w, strings.TrimRight(output.FormatTree(forest), "\n"))
		})
		return nil
	}
}
