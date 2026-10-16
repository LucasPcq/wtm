// Package cli is the command line of wtm-next: the cobra root, its commands and their exit codes.
package cli

import (
	"context"

	"github.com/spf13/cobra"
)

const (
	exitOK    = 0
	exitError = 1
)

func NewRoot(version string) *cobra.Command {
	return &cobra.Command{
		Use:          "wtm-next",
		Short:        "Git worktree manager, rebuilt as slices",
		Version:      version,
		SilenceUsage: true,
	}
}

func Execute(ctx context.Context, root *cobra.Command) int {
	if err := root.ExecuteContext(ctx); err != nil {
		return exitError
	}
	return exitOK
}
