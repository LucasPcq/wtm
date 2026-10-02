package shared

import "github.com/spf13/cobra"

func Interactive() bool { return true }

func AddYesFlag(cmd *cobra.Command) {}

func AddNoPromptFlags(cmd *cobra.Command) {}
