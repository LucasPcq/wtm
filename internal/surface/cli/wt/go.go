package wt

import (
	"github.com/spf13/cobra"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

// newGoCmd creates the wtm go subcommand (fallback when shell wrapper is not configured).
func newGoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   domain.CmdGo + " [branch]",
		Short: "Switch to a worktree",
		Long:  "Navigate to a worktree directory. Requires shell integration to work.",
		Example: `  # Pick a worktree
  wtm go

  wtm go feat/login

  # Back to the main checkout
  wtm go main`,
		RunE: runGo,
	}
}

func runGo(cmd *cobra.Command, _ []string) error {
	render.Frame(cmd.ErrOrStderr(), func(w io.Writer) {
		render.Warning(w, "wtm go requires shell integration to change your working directory.")
		render.Blank(w)
		render.Message(w, domain.MsgShellInitHint)
	})
	return nil
}
