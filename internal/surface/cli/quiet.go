package cli

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// silenceHumanOutput is the whole of --quiet: the command's writers become
// io.Discard, so every framed conclusion, notice and progress line goes nowhere
// while the exit code and the error still do — Execute prints those to
// os.Stderr, not through the command.
//
// It never touches a machine contract: --output json still emits its document,
// and a command whose stdout IS the answer (a path, a script, a URL) says so
// with an annotation. Asking for less noise is not asking for less answer.
func silenceHumanOutput(cmd *cobra.Command) {
	quiet, _ := cmd.Flags().GetBool(domain.FlagQuiet)
	if !quiet {
		return
	}
	// Text is the only human format. Every other one — json, mermaid, whatever is
	// added next — is a document someone asked for, and --quiet is about noise.
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

// humanOutputSilenced records that --quiet actually took a command's writers
// away. It is what makes ErrAborted safe: that sentinel means "the report is
// already on screen", which stops being true the moment the screen was
// io.Discard.
var humanOutputSilenced bool
