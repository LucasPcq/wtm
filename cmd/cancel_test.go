package cmd

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// A run the user backed out of ends the process on its own code, so a shell
// chaining `wtm create x && wtm go x` stops there; the `=` line it printed is
// the whole report.
func TestABackedOutRunExitsCancelled(t *testing.T) {
	cmd := &cobra.Command{}
	shared.MarkCancelled(cmd)

	err := rootCmd.PersistentPostRunE(cmd, nil)
	if got := rules.ExitCode(err); got != domain.ExitCodeCancelled {
		t.Fatalf("exit code = %d, want %d", got, domain.ExitCodeCancelled)
	}
	if !errors.Is(err, domain.ErrAborted) {
		t.Error("the error must carry ErrAborted: the report is already on screen")
	}
	if got := abortLine(err); got != domain.AbortedMessage {
		t.Errorf("under --quiet the run says %q, want %q", got, domain.AbortedMessage)
	}
}

func TestARunThatWentThroughExitsZero(t *testing.T) {
	if err := rootCmd.PersistentPostRunE(&cobra.Command{}, nil); err != nil {
		t.Errorf("err = %v, want none", err)
	}
}

func TestTheMarkDoesNotOutliveItsRun(t *testing.T) {
	cmd := &cobra.Command{}
	shared.MarkCancelled(cmd)
	shared.ClearCancelled(cmd)
	if shared.Cancelled(cmd) {
		t.Error("a cleared command still reads as cancelled")
	}
}
