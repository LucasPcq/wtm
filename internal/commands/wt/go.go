package wt

import (
	"github.com/spf13/cobra"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
)

// newGoCmd creates the wtm go subcommand (fallback when shell wrapper is not configured).
func newGoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   domain.CmdGo + " [branch]",
		Short: "Switch to a worktree",
		Long:  "Navigate to a worktree directory. Requires shell integration to work.",
		RunE:  runGo,
	}
}

func runGo(cmd *cobra.Command, _ []string) error {
	output.Frame(cmd.ErrOrStderr(), func(w io.Writer) {
		output.Warning(w, "wtm go requires shell integration to change your working directory.")
		output.Blank(w)
		output.Message(w, domain.MsgShellInitHint)
	})
	return nil
}
