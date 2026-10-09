package wt

import (
	"context"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	syncflow "github.com/LucasPcq/wtm/internal/flow/sync"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

// Without a TTY, in human output and with neither --yes nor --dry-run, sync
// refuses instead of attempting a confirmation that cannot be displayed. This is
// prune's model (PruneNeedsTerminal), and the only behavior the flow migration
// changes.
func TestSyncWithoutTerminalRefuses(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "feat-a")
	if err == nil {
		t.Fatal("sync without a terminal and without --yes must be refused")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagYes) {
		t.Fatalf("the refusal must name --yes, got: %v", err)
	}
}

// onATerminal stands in for a user at the wizard, answering from prompter.
func onATerminal(t *testing.T, prompter *flowtest.ScriptedPrompter) {
	t.Helper()
	previousTTY, previousPrompter := shared.StdinIsTerminal, shared.InteractivePrompter
	shared.StdinIsTerminal = func() bool { return true }
	shared.InteractivePrompter = func(context.Context, shared.FlowPrompterParams) flow.Prompter { return prompter }
	t.Cleanup(func() { shared.StdinIsTerminal, shared.InteractivePrompter = previousTTY, previousPrompter })
}

// LUC-281: on a terminal, --keep-conflict only led the options with "keep" and
// the wizard still put the conflict question.
func TestSyncKeepConflictIsNotAskedOnATerminal(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	prompter := &flowtest.ScriptedPrompter{
		Sets:    map[string][]string{syncflow.KeySelection: {"feat-a"}},
		Answers: map[string]string{syncflow.KeyConfirm: domain.WizardCancelValue},
	}
	onATerminal(t, prompter)

	if _, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagKeepConflict); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if got, want := prompter.AskedKeys(), syncflow.KeySelection+","+syncflow.KeyConfirm; got != want {
		t.Fatalf("asked %q, want %q: --keep-conflict answers the conflict step", got, want)
	}
}

func TestSyncAsksTheConflictStepWithoutTheFlag(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	prompter := &flowtest.ScriptedPrompter{
		Sets:    map[string][]string{syncflow.KeySelection: {"feat-a"}},
		Answers: map[string]string{syncflow.KeyConflict: "normal", syncflow.KeyConfirm: domain.WizardCancelValue},
	}
	onATerminal(t, prompter)

	if _, _, err := runWtCmd(t, domain.CmdSync); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if !strings.Contains(prompter.AskedKeys(), syncflow.KeyConflict) {
		t.Fatalf("without --keep-conflict the conflict step must be asked, got %q", prompter.AskedKeys())
	}
}
