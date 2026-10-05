package clean

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

func plantUndeletable(t *testing.T, dir string) {
	t.Helper()
	locked := filepath.Join(dir, "root-owned")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "pgdata"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
}

// Declining the privileged deletion settles the removal as git left it: the
// failure is said, the question asked once, and the leftover named — sudo is
// never reached.
func TestADeclinedPrivilegedRemovalSettlesWhatGitDid(t *testing.T) {
	d := newDataFixture(t)
	plantUndeletable(t, d.path)
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteYes}}
	presenter := newRecorder()

	_, err := Run(t.Context(), Params{
		Context:   d.ctx,
		Request:   Request{Branches: []string{d.branch}, BaseBranch: "main", Force: true, AllowPrivileged: true},
		Prompter:  prompter,
		Presenter: presenter,
	})

	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if prompter.Confirms != 1 {
		t.Errorf("confirms = %d, want the privileged deletion offered once", prompter.Confirms)
	}
	if !hasStatus(presenter, "Removal failed") {
		t.Errorf("statuses = %+v, want the failure said before the question", presenter.Statuses)
	}
	if !hasStatus(presenter, "sudo rm -rf "+d.path) {
		t.Errorf("statuses = %+v, want the leftover named", presenter.Statuses)
	}
	if worktree.StillTracked(worktree.FindByBranchParams{ProjectDir: d.ctx.ProjectDir, Branch: d.branch}) {
		t.Error("git still tracks the worktree")
	}
}

// Without a privileged fallback on offer nobody is asked: a surface that did
// not allow it, or one that cannot answer, settles straight away.
func TestAnUnofferedPrivilegedRemovalAsksNothing(t *testing.T) {
	d := newDataFixture(t)
	plantUndeletable(t, d.path)
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteYes}}

	if _, err := Run(t.Context(), Params{
		Context:   d.ctx,
		Request:   Request{Branches: []string{d.branch}, BaseBranch: "main", Force: true},
		Prompter:  prompter,
		Presenter: newRecorder(),
	}); err != nil {
		t.Fatalf("clean: %v", err)
	}
	if prompter.Confirms != 0 {
		t.Errorf("confirms = %d, want none", prompter.Confirms)
	}
}
