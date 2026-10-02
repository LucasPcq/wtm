package wt

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// --to with no worktree to move used to say "already aligned" and leave the
// config alone: base_path is the one thing such a run has to change.
func TestRelocateToRewritesBasePathWithNoWorktreeToMove(t *testing.T) {
	repo := newRelocateRepo(t)

	result, err := relocateJSON(t, "--to", "../elsewhere", "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("relocate --to: %v", err)
	}
	if !result.BasePathUpdated || result.BasePath != "../elsewhere" {
		t.Fatalf("result = %+v, want base_path rewritten", result)
	}
	if !strings.Contains(repo.config(t), `base_path = "../elsewhere"`) {
		t.Errorf("config not rewritten:\n%s", repo.config(t))
	}
}

func TestRelocateToDryRunPreviewsABasePathOnlyChange(t *testing.T) {
	repo := newRelocateRepo(t)

	stdout, _, err := runWtCmd(t, domain.CmdRelocate, "--to", "../elsewhere", "--"+domain.FlagDryRun)
	if err != nil {
		t.Fatalf("relocate --to --dry-run: %v", err)
	}
	want := "\n" +
		"  base_path: ../.trees → ../elsewhere (no worktree to move)\n" +
		"\n" +
		"  = Dry run — no changes made.\n" +
		"\n"
	if stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout, want)
	}
	if strings.Contains(repo.config(t), "../elsewhere") {
		t.Errorf("a dry run rewrote the config")
	}
}

func TestRelocateToWithNoWorktreeToMoveReportsTheRewriteAlone(t *testing.T) {
	newRelocateRepo(t)

	stdout, _, err := runWtCmd(t, domain.CmdRelocate, "--to", "../elsewhere", "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("relocate --to: %v", err)
	}
	if want := "\n  ✓ config base_path updated to \"../elsewhere\"\n\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}
