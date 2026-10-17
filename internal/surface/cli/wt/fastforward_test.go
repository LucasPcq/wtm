package wt

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// These characterize `wtm fast-forward` on the non-interactive path: the
// worktrees are a required selection with no safe default, so a run that cannot
// prompt refuses naming --all instead of opening the multi-select.

func TestFastForwardWithoutSelectionErrors(t *testing.T) {
	_, _ = setupStack(t)

	_, _, err := runWtCmd(t, domain.CmdFastForward, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("expected a missing selection to error under --yes")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Errorf("error %q should direct to --%s", err, domain.FlagAll)
	}
}

// No terminal resolves the same way --yes does: the picker only ever runs in a
// fully interactive run.
func TestFastForwardWithoutTerminalRefusesLikeYes(t *testing.T) {
	_, _ = setupStack(t)

	_, _, err := runWtCmd(t, domain.CmdFastForward)
	if err == nil {
		t.Fatal("expected a run with no terminal to refuse rather than prompt")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Errorf("error %q should direct to --%s", err, domain.FlagAll)
	}
}

func TestFastForwardRejectsAllWithBranchArgs(t *testing.T) {
	_, _ = setupStack(t)

	_, _, err := runWtCmd(t, domain.CmdFastForward, "--"+domain.FlagAll, "feat")
	if err == nil {
		t.Fatal("expected --all with branch arguments to error")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Errorf("error %q should name --%s", err, domain.FlagAll)
	}
}

// The confirmation cannot run in JSON mode, so the confirmation axis must be
// bypassed explicitly rather than implicitly.
func TestFastForwardJSONRequiresYes(t *testing.T) {
	_, _ = setupStack(t)

	_, _, err := runWtCmd(t, domain.CmdFastForward, "feat", "--"+domain.FlagOutput, domain.OutputJSON)
	if err == nil {
		t.Fatal("expected --output json without --yes to error")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagYes) {
		t.Errorf("error %q should direct to --%s", err, domain.FlagYes)
	}
}

// A repo with no remote has no origin counterpart for anything, so every branch
// is refused by name rather than touched — and the run still reports each one.
func TestFastForwardReportsBranchesWithNoOriginCounterpart(t *testing.T) {
	_, _ = setupStack(t)

	stdout, _, err := runWtCmd(t, domain.CmdFastForward, "feat",
		"--"+domain.FlagOutput, domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var results []domain.FastForwardResult
	if decodeErr := json.Unmarshal([]byte(stdout), &results); decodeErr != nil {
		t.Fatalf("decode %q: %v", stdout, decodeErr)
	}
	if len(results) != 1 || results[0].Branch != "feat" {
		t.Fatalf("results = %+v, want the one branch that was named", results)
	}
	if results[0].Label != domain.FastForwardLabelNoRemote {
		t.Errorf("status = %q, want %q", results[0].Label, domain.FastForwardLabelNoRemote)
	}
}

// The alias is the frequent gesture's short form; it must reach the same command.
func TestFastForwardAliasResolves(t *testing.T) {
	_, _ = setupStack(t)

	_, _, err := runWtCmd(t, domain.CmdFastForwardAlias, "--"+domain.FlagAll, "feat")
	if err == nil {
		t.Fatal("expected the alias to reach the same flag validation")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Errorf("error %q should name --%s", err, domain.FlagAll)
	}
}
