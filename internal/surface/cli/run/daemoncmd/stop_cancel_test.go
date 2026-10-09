package daemoncmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/commands/run/daemoncmd"
	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Backing out of the stop confirmation — Esc, Ctrl-C or "No" — is a
// cancellation like every other one: `= Aborted.`, exit 19, daemon untouched.
func TestBackingOutOfTheStopConfirmationIsACancellation(t *testing.T) {
	answers := map[string]func(components.ConfirmModel) (bool, error){
		"escaped":  func(components.ConfirmModel) (bool, error) { return false, components.ErrAborted },
		"declined": func(components.ConfirmModel) (bool, error) { return false, nil },
	}
	for name, answer := range answers {
		t.Run(name, func(t *testing.T) {
			daemon := processtest.Serve(t, mixed)
			previous := runctx.IsTTY
			runctx.IsTTY = func() bool { return true }
			t.Cleanup(func() { runctx.IsTTY = previous })
			daemoncmd.StubConfirm(t, answer)

			root := daemoncmd.NewCmd()
			var stdout bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&bytes.Buffer{})
			root.SetArgs([]string{domain.CmdStop})
			root.SilenceUsage, root.SilenceErrors = true, true
			stop, _, _ := root.Find([]string{domain.CmdStop})

			if err := root.Execute(); err != nil {
				t.Fatalf("stop: %v, want the abort concluded, not failed", err)
			}
			if !shared.Cancelled(stop) {
				t.Error("the run is not marked cancelled: it would exit 1, not 19")
			}
			if !strings.Contains(stdout.String(), domain.AbortedMessage) {
				t.Errorf("stdout = %q, want %q", stdout.String(), domain.AbortedMessage)
			}
			if shutdowns(daemon) != 0 {
				t.Errorf("requests = %v, want the daemon left alone", daemon.Actions())
			}
		})
	}
}
