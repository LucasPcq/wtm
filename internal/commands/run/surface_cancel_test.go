package run

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

// interruptedStart is a start an interrupt cut short with api up and migrate
// in flight — which the daemon took, and runs.
func interruptedStart(t *testing.T) (*cobra.Command, *bytes.Buffer, runlogs.StartFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	start := func(context.Context, runlogs.Sink) (runlogs.Outcomes, error) {
		return runlogs.Outcomes{{
			Worktree:   "main",
			Started:    []string{"api", "migrate"},
			NotStarted: []string{"web"},
			Steps:      3,
			Results:    []domain.JobActionResult{{Name: "api", Status: domain.JobActionStarted}, {Name: "migrate", Status: domain.JobActionStarted}},
		}}, context.Canceled
	}
	return cmd, &stdout, start
}

// Without the run view, an interrupted `run up` still says what it left
// running and how to stop it, instead of ending on the interrupt alone.
func TestAnInterruptedStreamRunSaysWhatItLeftRunning(t *testing.T) {
	cmd, stdout, start := interruptedStart(t)

	_, err := runOnStream(streamParams{Cmd: cmd, Start: start})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the interrupt passed on", err)
	}
	for _, want := range []string{fmt.Sprintf(domain.RunViewRecapRunningFmt, "api, migrate"), fmt.Sprintf(domain.RunViewRecapNotStartedFmt, "web"), domain.RunStreamStopHint} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), domain.RunViewRecapNoneRunning) {
		t.Errorf("stdout claims nothing is left running:\n%s", stdout.String())
	}
}

// --output json writes its document even when interrupted: a script reading it
// learns what is still running.
func TestAnInterruptedMachineRunStillWritesItsDocument(t *testing.T) {
	cmd, stdout, start := interruptedStart(t)

	_, err := runForMachine(streamParams{Cmd: cmd, Start: start})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the interrupt passed on", err)
	}
	var documents []domain.WorktreeRunResult
	if decodeErr := json.Unmarshal(stdout.Bytes(), &documents); decodeErr != nil {
		t.Fatalf("stdout is not the document: %v\n%s", decodeErr, stdout.String())
	}
	if len(documents) != 1 || len(documents[0].Jobs) != 2 {
		t.Errorf("documents = %+v, want api and migrate listed", documents)
	}
}
