package fastforward

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// recorder is the Presenter half, keeping the conclusion the run reached.
type recorder struct {
	*flowtest.Recorder
	outcome Outcome
}

func (r *recorder) FastForwarded(outcome Outcome) error {
	r.outcome = outcome
	return nil
}

// behindRepo returns a repo whose checked-out main is one commit behind
// origin/main, so the run has something real to move.
func behindRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.InitRepo(t)
	gittest.AddOrigin(t, dir)
	gittest.Git(t, dir, "push", "-u", "origin", "main")

	gittest.Git(t, dir, "checkout", "-b", "ahead")
	gittest.Git(t, dir, "commit", "--allow-empty", "-m", "pushed by someone else")
	gittest.Git(t, dir, "push", "origin", "ahead:main")
	gittest.Git(t, dir, "checkout", "main")
	gittest.Git(t, dir, "branch", "-D", "ahead")
	return dir
}

func tipOf(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-parse", ref)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

// runFlow drives a whole Run with the confirm step answered as a human would.
func runFlow(t *testing.T, dir, confirm string) (Outcome, error) {
	t.Helper()
	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{KeyConfirm: confirm},
	}
	presenter := &recorder{Recorder: &flowtest.Recorder{}}
	return Run(t.Context(), Params{
		Context:   flow.Context{ProjectDir: dir, StateDir: filepath.Join(t.TempDir(), "state")},
		Request:   Request{Branches: []string{"main"}},
		Prompter:  prompter,
		Presenter: presenter,
	})
}

// A plain confirmation is not a licence to touch a dirty worktree: the run
// refuses and leaves the branch exactly where it was.
func TestAPlainConfirmationLeavesADirtyWorktreeAlone(t *testing.T) {
	dir := behindRepo(t)
	writeUntracked(t, dir)
	before := tipOf(t, dir, "main")

	outcome, err := runFlow(t, dir, confirmYes)
	if err == nil {
		t.Fatal("a refused branch must end the run non-nil: it is a failure the caller reports")
	}
	if len(outcome.Results) != 1 || outcome.Results[0].Status != domain.FFFailed {
		t.Fatalf("results = %+v, want the dirty refusal", outcome.Results)
	}
	if !strings.Contains(outcome.Results[0].Detail, "uncommitted") {
		t.Errorf("detail = %q, want it to name the uncommitted changes", outcome.Results[0].Detail)
	}
	if got := tipOf(t, dir, "main"); got != before {
		t.Errorf("main moved to %s, want it left at %s", got, before)
	}
}

// The danger option is what a human clicks instead of passing --force, so the
// run has to read the answer back. Discarding it — falling back to a
// Request.Force no interactive surface ever sets — is the bug this locks: the
// dashboard's "Fast-forward anyway" reported "failed — uncommitted changes".
func TestTheDangerAnswerLiftsTheDirtyRefusalWithNoForceFlag(t *testing.T) {
	dir := behindRepo(t)
	writeUntracked(t, dir)
	want := tipOf(t, dir, "origin/main")

	outcome, err := runFlow(t, dir, confirmForce)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcome.Results) != 1 || outcome.Results[0].Status != domain.FFAdvanced {
		t.Fatalf("results = %+v, want the branch advanced", outcome.Results)
	}
	if got := tipOf(t, dir, "main"); got != want {
		t.Errorf("main = %s, want it advanced to origin/main %s", got, want)
	}
}

func writeUntracked(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "scratch.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatalf("write scratch file: %v", err)
	}
}
