package wt

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmds returns the worktree subcommands, each assigned to its root --help
// group, ready to be registered directly on the root command.
func NewCmds() []*cobra.Command {
	grouped := func(cmd *cobra.Command, group string) *cobra.Command {
		cmd.GroupID = group
		return cmd
	}

	return []*cobra.Command{
		grouped(newListCmd(), domain.CmdGroupWorktrees),
		grouped(newTreeCmd(), domain.CmdGroupWorktrees),
		grouped(newCreateCmd(), domain.CmdGroupWorktrees),
		grouped(newCleanCmd(), domain.CmdGroupWorktrees),
		grouped(newPruneCmd(), domain.CmdGroupWorktrees),
		grouped(newExtractCmd(), domain.CmdGroupWorktrees),
		grouped(newEnvCmd(), domain.CmdGroupWorktrees),
		grouped(newRelocateCmd(), domain.CmdGroupWorktrees),
		grouped(newGoCmd(), domain.CmdGroupNavigate),
		grouped(newSyncCmd(), domain.CmdGroupStack),
		grouped(newReparentCmd(), domain.CmdGroupStack),
		grouped(newFastForwardCmd(), domain.CmdGroupStack),
	}
}
