package cmd

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

func quietTestCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return nil }}
	cmd.Flags().Bool(domain.FlagQuiet, false, "")
	cmd.Flags().String(domain.FlagOutput, domain.OutputText, "")
	if err := cmd.Flags().Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

func silenced(cmd *cobra.Command) bool {
	return cmd.OutOrStdout() == io.Discard && cmd.ErrOrStderr() == io.Discard
}

func TestQuietSilencesHumanOutput(t *testing.T) {
	cmd := quietTestCmd(t, "--quiet")
	silenceHumanOutput(cmd)
	if !silenced(cmd) {
		t.Error("--quiet left the command writing")
	}
}

// A caller asking for less noise did not ask for less answer: the machine
// contracts are what --quiet must never touch.
func TestQuietLeavesMachineOutputAlone(t *testing.T) {
	json := quietTestCmd(t, "--quiet", "--output", domain.OutputJSON)
	silenceHumanOutput(json)
	if silenced(json) {
		t.Error("--quiet swallowed the JSON document")
	}

	machine := quietTestCmd(t, "--quiet")
	machine.Annotations = map[string]string{domain.AnnotationMachineOutput: domain.AnnotationOn}
	silenceHumanOutput(machine)
	if silenced(machine) {
		t.Error("--quiet swallowed a command whose stdout is its contract")
	}
}

func TestWithoutQuietNothingIsSilenced(t *testing.T) {
	cmd := quietTestCmd(t)
	silenceHumanOutput(cmd)
	if silenced(cmd) {
		t.Error("a command with no --quiet was silenced")
	}
}

// --quiet takes the report away, so ErrAborted — which means "the report is
// already on screen" — stops being a reason to say nothing.
func TestQuietRecordsThatTheReportWasDiscarded(t *testing.T) {
	humanOutputSilenced = false
	t.Cleanup(func() { humanOutputSilenced = false })

	silenceHumanOutput(quietTestCmd(t))
	if humanOutputSilenced {
		t.Error("a run with no --quiet was recorded as silenced")
	}

	silenceHumanOutput(quietTestCmd(t, "--quiet"))
	if !humanOutputSilenced {
		t.Error("--quiet discarded the report without recording it")
	}
}

// A run that exits non-zero with nothing on either stream cannot be told from
// one that hung.
func TestAbortLineStandsInForADiscardedReport(t *testing.T) {
	if got := abortLine(domain.ErrAborted); got != domain.QuietAbortedMessage {
		t.Errorf("abortLine(bare) = %q, want the stand-in message", got)
	}
	withCause := fmt.Errorf("%w: %s", domain.ErrAborted, "job web has no cmd")
	if got := abortLine(withCause); got != withCause.Error() {
		t.Errorf("abortLine(wrapped) = %q, want the cause kept", got)
	}
}
