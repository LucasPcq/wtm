package aliased // want `cmd.go reads the interactive gate but registers no --yes`

import (
	sh "github.com/LucasPcq/wtm/internal/commands/shared"
	cb "github.com/spf13/cobra"
)

func newCmd() *cb.Command { return &cb.Command{} }

func run() bool { return sh.Interactive() }
