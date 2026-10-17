package cli

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

func silenceHumanOutput(cmd *cobra.Command) {
	quiet, _ := cmd.Flags().GetBool(domain.FlagQuiet)
	if !quiet {
		return
	}
	if format, _ := cmd.Flags().GetString(domain.FlagOutput); format != "" && format != domain.OutputText {
		return
	}
	if cmd.Annotations[domain.AnnotationMachineOutput] != "" {
		return
	}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	humanOutputSilenced = true
}

var humanOutputSilenced bool
