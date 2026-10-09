package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
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

// An interrupt that left a worktree behind says so: "Aborted." alone hid a
// worktree the reader did not know had been created.
func TestAnInterruptThatLeftSomethingBehindIsNamed(t *testing.T) {
	err := fmt.Errorf(domain.CreateSetupInterruptedFmt, domain.ErrLeftBehind, "/trees/feat-x")
	interrupted := rules.Interrupted(rules.InterruptedParams{Err: err, Signalled: true})

	if got := abortLine(interrupted); !strings.Contains(got, "/trees/feat-x was created but not set up") {
		t.Errorf("the run says %q, want the worktree left behind named", got)
	}
	if rules.ExitCode(interrupted) != domain.ExitCodeCancelled {
		t.Errorf("exit code = %d, want %d", rules.ExitCode(interrupted), domain.ExitCodeCancelled)
	}
	if got := abortLine(fmt.Errorf("%w: git fetch: signal: interrupt", domain.ErrCancelled)); got != domain.AbortedMessage {
		t.Errorf("a bare interrupt says %q, want %q", got, domain.AbortedMessage)
	}
}

// A bare interrupt reads like a run the user backed out of — nothing failed —
// while one that left a worktree behind stays a failure the reader must act on.
func TestAnInterruptIsReportedInTheBackedOutRegister(t *testing.T) {
	var bare, left bytes.Buffer
	reportFailure(&bare, fmt.Errorf("%w: git fetch: signal: interrupt", domain.ErrCancelled))
	reportFailure(&left, fmt.Errorf(domain.CreateSetupInterruptedFmt, domain.ErrLeftBehind, "/trees/feat-x"))

	if !strings.Contains(bare.String(), domain.GlyphUnchanged+" "+domain.AbortedMessage) {
		t.Errorf("bare interrupt = %q, want the %q line", bare.String(), domain.GlyphUnchanged)
	}
	if !strings.Contains(left.String(), domain.GlyphFailure) {
		t.Errorf("left behind = %q, want it reported as a failure", left.String())
	}
}
