package missing // want `cmd.go reads the interactive gate but registers no --yes`

import (
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/spf13/cobra"
)

func newCmd() *cobra.Command { return &cobra.Command{} }

func run() bool { return shared.Interactive() }
