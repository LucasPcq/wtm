package shared

import "github.com/spf13/cobra"

func Interactive() bool { return true }

func AddYesFlag(cmd *cobra.Command) {}

func AddNoPromptFlags(cmd *cobra.Command) {}

type PrompterParams struct{ Interactive bool }

func Prompter(params PrompterParams) bool { return params.Interactive }
