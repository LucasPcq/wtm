package wt

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// These tests characterize `wtm reparent` on the non-interactive path (--yes,
// --output json, or simply no terminal): the worktrees and the new parent are
// required selections with no safe default, so each one either comes from an
// argument or errors naming what would have answered it — never from a picker.
// They are written before the flow/ migration and must pass unchanged after it.

// TestReparentWithoutBranchesErrors: the worktrees are a required selection, so
// a run that cannot prompt refuses instead of opening the multi-select.
func TestReparentWithoutBranchesErrors(t *testing.T) {
	_, _ = setupStack(t)

	_, _, err := runWtCmd(t, domain.CmdReparent, "--to", "main", "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("expected a missing worktree selection to error under --yes")
	}
	if !strings.Contains(err.Error(), "worktree") {
		t.Errorf("error %q should say a worktree is required", err)
	}
}

// TestReparentWithoutParentErrors: --to has no safe default either — reparenting
// onto a guessed branch would rewrite the stack the user did not ask for.
func TestReparentWithoutParentErrors(t *testing.T) {
	_, _ = setupStack(t)

	_, _, err := runWtCmd(t, domain.CmdReparent, "dev-b", "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("expected a missing --to to error under --yes")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagTo) {
		t.Errorf("error %q should direct to --%s", err, domain.FlagTo)
	}
}

// TestReparentWithoutTerminalRefusesLikeYes: no terminal resolves the same way
// --yes does. The picker only ever runs in a fully interactive run.
func TestReparentWithoutTerminalRefusesLikeYes(t *testing.T) {
	_, _ = setupStack(t)

	_, _, err := runWtCmd(t, domain.CmdReparent, "dev-b")
	if err == nil {
		t.Fatal("expected a run with no terminal to refuse rather than prompt")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagTo) {
		t.Errorf("error %q should direct to --%s", err, domain.FlagTo)
	}
}

// TestReparentJSONRequiresYes: the confirmation cannot run in JSON mode, so the
// confirmation axis must be bypassed explicitly rather than implicitly.
func TestReparentJSONRequiresYes(t *testing.T) {
	_, _ = setupStack(t)

	_, _, err := runWtCmd(t, domain.CmdReparent, "dev-b", "--to", "main", "--output", domain.OutputJSON)
	if err == nil {
		t.Fatal("expected --output json without --yes to be rejected")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagYes) {
		t.Errorf("error %q should direct to --%s", err, domain.FlagYes)
	}
}

// TestReparentUnmanagedBranchErrors: only worktrees wtm records a parent for can
// be reparented; the main worktree is not one of them.
func TestReparentUnmanagedBranchErrors(t *testing.T) {
	_, _ = setupStack(t)

	_, _, err := runWtCmd(t, domain.CmdReparent, "main", "--to", "feat", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("expected reparenting the main worktree to error")
	}
}

// TestReparentHumanOutputNamesTheWorktreeToSync: reparent only rewrites metadata,
// so the conclusion has to say how the change is applied — naming the worktree
// when there is exactly one.
func TestReparentHumanOutputNamesTheWorktreeToSync(t *testing.T) {
	_, _ = setupStack(t)

	stdout, _, err := runWtCmd(t, domain.CmdReparent, "dev-b", "--to", "feat", "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("reparent: %v", err)
	}
	if !strings.Contains(stdout, "dev-b") || !strings.Contains(stdout, "feat") {
		t.Errorf("output should restate the reparented worktree and its new parent, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, domain.CmdSync+" dev-b") {
		t.Errorf("output should suggest `wtm %s dev-b`, got:\n%s", domain.CmdSync, stdout)
	}
}

// TestReparentBatchHumanOutputSuggestsBareSync: several worktrees have no single
// one to name, so the hint points at the bare command.
func TestReparentBatchHumanOutputSuggestsBareSync(t *testing.T) {
	_, _ = setupStack(t)

	stdout, _, err := runWtCmd(t, domain.CmdReparent, "dev-a", "dev-b", "--to", "main", "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("batch reparent: %v", err)
	}
	for _, branch := range []string{"dev-a", "dev-b"} {
		if !strings.Contains(stdout, branch) {
			t.Errorf("output should restate %s, got:\n%s", branch, stdout)
		}
	}
	if strings.Contains(stdout, domain.CmdSync+" dev-a") || strings.Contains(stdout, domain.CmdSync+" dev-b") {
		t.Errorf("a batch hint should not name one worktree, got:\n%s", stdout)
	}
}
