package cli

import (
	"fmt"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestQuietSilencesHumanOutput(t *testing.T) {
	cmd := quietTestCmd(t, "--quiet")
	silenceHumanOutput(cmd)
	if !silenced(cmd) {
		t.Error("--quiet left the command writing")
	}
}

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

func TestAbortLineStandsInForADiscardedReport(t *testing.T) {
	if got := abortLine(domain.ErrAborted); got != domain.QuietAbortedMessage {
		t.Errorf("abortLine(bare) = %q, want the stand-in message", got)
	}
	withCause := fmt.Errorf("%w: %s", domain.ErrAborted, "job web has no cmd")
	if got := abortLine(withCause); got != withCause.Error() {
		t.Errorf("abortLine(wrapped) = %q, want the cause kept", got)
	}
}
