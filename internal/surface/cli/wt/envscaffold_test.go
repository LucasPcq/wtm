package wt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

const exampleProject = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n\n[env]\nstrategy = \"example\"\n\n[[env.file]]\ntarget = \".env\"\ntemplate = \".env.example\"\n\n[hooks]\non_create = []\n"

// exampleRepo is a project under the example strategy, its template committed,
// with feat/a created and its .env since deleted.
func exampleRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(exampleProject), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte("API_KEY=changeme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".env.example")
	gittest.Git(t, dir, "commit", "-m", "template")
	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/a", "--from", "main", "--"+domain.FlagYes); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := os.Remove(filepath.Join(resolveWorktreePath(t, dir, "feat/a"), ".env")); err != nil {
		t.Fatal(err)
	}
	return dir
}

// `wtm env --yes` used to leave a missing .env missing under the example
// strategy: its template's keys are placeholders, and an unattended run fills
// none. The fix status prints for it has to clear it.
func TestEnvRebuildsAMissingEnvAndStatusStopsReportingIt(t *testing.T) {
	dir := exampleRepo(t)

	before := statusDoc(t, "feat/a")
	if len(before.Problems) != 1 || before.Problems[0].Fix != "wtm env feat/a --yes" {
		t.Fatalf("problems = %+v, want env_missing fixed by wtm env", before.Problems)
	}

	if _, _, err := runWtCmd(t, domain.CmdEnv, "feat/a", "--"+domain.FlagYes); err != nil {
		t.Fatalf("env: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(resolveWorktreePath(t, dir, "feat/a"), ".env"))
	if err != nil || string(got) != "API_KEY=changeme\n" {
		t.Errorf(".env = %q (%v), want the template, as create writes it", got, err)
	}
	if after := statusDoc(t, "feat/a"); len(after.Problems) != 0 {
		t.Errorf("problems after the fix = %+v, want none", after.Problems)
	}
}
