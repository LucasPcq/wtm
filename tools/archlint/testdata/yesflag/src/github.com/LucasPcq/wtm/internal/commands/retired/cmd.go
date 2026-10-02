package retired // want `cmd.go reads the interactive gate but registers no --yes`

import (
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/spf13/cobra"
)

func newCmd() *cobra.Command {
	cmd := &cobra.Command{}
	shared.AddNoPromptFlags(cmd)
	return cmd
}

func run() bool { return shared.Interactive() }
