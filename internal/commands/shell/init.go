package shell

import (
	"fmt"
	"github.com/LucasPcq/wtm/internal/domain"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/shell"
)

// NewCmd creates the wtm shell-init command.
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "shell-init",
		Annotations: map[string]string{domain.AnnotationMachineOutput: domain.AnnotationOn},
		Short:       "Generate shell integration function",
		Long:        "Output a shell function to eval in your rc file.\nUsage: eval \"$(wtm shell-init)\"",
		RunE:        runShellInit,
	}
}

func runShellInit(cmd *cobra.Command, _ []string) error {
	detected := shell.DetectShell()
	fmt.Fprint(cmd.OutOrStdout(), rules.GenerateShellInit(detected))
	return nil
}
