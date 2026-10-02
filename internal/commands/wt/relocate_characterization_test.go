package wt

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

// These tests pin `wtm relocate` as it behaved before its move to internal/flow
// (LUC-238). They are not edited afterwards: a diff here is a behaviour change.

type relocateRepo struct {
	dir      string
	stateDir string
}

func newRelocateRepo(t *testing.T) relocateRepo {
	t.Helper()
	dir, err := filepath.EvalSymlinks(gittest.InitRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}
	return relocateRepo{dir: dir, stateDir: stateDir}
}

func (r relocateRepo) create(t *testing.T, branch string) {
	t.Helper()
	if _, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create %s: %v", branch, err)
	}
}

func (r relocateRepo) external(t *testing.T, branch, path string) {
	t.Helper()
	gittest.CreateBranch(t, r.dir, branch)
	gitWorktreeAdd(t, r.dir, path, branch)
}

func (r relocateRepo) root() string { return filepath.Dir(r.dir) }

func (r relocateRepo) gitPath(t *testing.T, branch string) string {
	t.Helper()
	worktrees, err := worktree.ListAll(worktree.ListAllParams{ProjectDir: r.dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, wt := range worktrees {
		if wt.Branch == branch {
			return wt.Path
		}
	}
	t.Fatalf("no worktree on %s", branch)
	return ""
}

func (r relocateRepo) meta(t *testing.T, branch string) domain.WorktreeMetadata {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(rules.WorktreeMetaDir(r.stateDir, branch), domain.MetaFileName))
	if err != nil {
		t.Fatalf("read meta.json of %s: %v", branch, err)
	}
	var meta domain.WorktreeMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("parse meta.json of %s: %v", branch, err)
	}
	return meta
}

func (r relocateRepo) config(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(r.stateDir, domain.ConfigFileName))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return string(data)
}

func relocateJSON(t *testing.T, args ...string) (domain.RelocateResult, error) {
	t.Helper()
	stdout, _, err := runWtCmd(t, append([]string{domain.CmdRelocate, "--output", domain.OutputJSON}, args...)...)
	// A failing run is followed by cobra's usage text: only the first value is the result.
	var result domain.RelocateResult
	if jsonErr := json.NewDecoder(strings.NewReader(stdout)).Decode(&result); jsonErr != nil {
		t.Fatalf("parse json %q: %v (command error: %v)", stdout, jsonErr, err)
	}
	return result, err
}

func statuses(result domain.RelocateResult) map[string]domain.RelocateStatus {
	out := make(map[string]domain.RelocateStatus, len(result.Steps))
	for _, step := range result.Steps {
		out[step.Branch] = step.Status
	}
	return out
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s to exist: %v", path, err)
	}
}

func assertGone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected %s to be gone, stat: %v", path, err)
	}
}

func TestRelocateCharacterizeRefusesWithoutTerminalOrYes(t *testing.T) {
	newRelocateRepo(t)

	_, _, err := runWtCmd(t, domain.CmdRelocate)
	if err == nil || err.Error() != "relocate needs a terminal to confirm; re-run with --yes to proceed unattended" {
		t.Fatalf("err = %v", err)
	}
}

func TestRelocateCharacterizeJSONNeedsYesOrDryRun(t *testing.T) {
	newRelocateRepo(t)

	_, _, err := runWtCmd(t, domain.CmdRelocate, "--output", domain.OutputJSON)
	if err == nil || err.Error() != "--output json requires --yes or --dry-run (the confirmation cannot run in JSON mode)" {
		t.Fatalf("err = %v", err)
	}
}

func TestRelocateCharacterizeNothingToDo(t *testing.T) {
	repo := newRelocateRepo(t)
	repo.create(t, "feat/in-place")

	stdout, _, err := runWtCmd(t, domain.CmdRelocate, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("relocate: %v", err)
	}
	if want := "\n  All worktrees are already aligned with base_path.\n\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	stdout, _, err = runWtCmd(t, domain.CmdRelocate, "--"+domain.FlagYes, "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("relocate json: %v", err)
	}
	if want := "{\n  \"base_path\": \"../.trees\",\n  \"base_path_updated\": false,\n  \"steps\": []\n}\n"; stdout != want {
		t.Errorf("json = %q, want %q", stdout, want)
	}
}

func TestRelocateCharacterizeMovesABatch(t *testing.T) {
	repo := newRelocateRepo(t)
	repo.create(t, "feat/a")
	repo.create(t, "feat/b")

	stdout, _, err := runWtCmd(t, domain.CmdRelocate, "--to", "../.worktrees", "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("relocate: %v", err)
	}
	want := "\n" +
		"  ✓ Relocation complete  2 applied\n" +
		"\n" +
		"  ✓ feat/a → ../.worktrees/feat-a\n" +
		"  ✓ feat/b → ../.worktrees/feat-b\n" +
		"\n" +
		"  ✓ config base_path updated to \"../.worktrees\"\n" +
		"\n"
	if stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout, want)
	}

	for _, name := range []string{"feat-a", "feat-b"} {
		assertGone(t, filepath.Join(repo.root(), ".trees", name))
		assertExists(t, filepath.Join(repo.root(), ".worktrees", name))
	}
	if !strings.Contains(repo.config(t), `base_path = "../.worktrees"`) {
		t.Errorf("config not updated:\n%s", repo.config(t))
	}
}

func TestRelocateCharacterizeAdoptsInPlace(t *testing.T) {
	repo := newRelocateRepo(t)
	repo.external(t, "feat/here", filepath.Join(repo.root(), ".trees", "feat-here"))

	stdout, _, err := runWtCmd(t, domain.CmdRelocate, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("relocate: %v", err)
	}
	want := "\n" +
		"  ✓ Relocation complete  1 applied\n" +
		"\n" +
		"  ✓ feat/here adopted in place (parent: main)\n" +
		"\n"
	if stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout, want)
	}

	meta := repo.meta(t, "feat/here")
	if meta.SourceBranch != "main" || meta.CreatedAt == "" {
		t.Errorf("adoption not recorded: %+v", meta)
	}
}

func TestRelocateCharacterizeMovesAndAdoptsWithHumanOutput(t *testing.T) {
	repo := newRelocateRepo(t)
	repo.external(t, "feat/manual", filepath.Join(repo.root(), "manual-wt"))

	stdout, _, err := runWtCmd(t, domain.CmdRelocate, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("relocate: %v", err)
	}
	want := "\n" +
		"  ✓ Relocation complete  1 applied\n" +
		"\n" +
		"  ✓ feat/manual → ../.trees/feat-manual (adopted, parent: main)\n" +
		"\n"
	if stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout, want)
	}
}

func TestRelocateCharacterizeAdoptionKeepsMetaJSON(t *testing.T) {
	repo := newRelocateRepo(t)
	repo.external(t, "feat/x", filepath.Join(repo.root(), "x-wt"))
	held := `{"ordinal": 3, "isolation": "verbatim", "namespaces": ["postgres"]}`
	metaDir := rules.WorktreeMetaDir(repo.stateDir, "feat/x")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, domain.MetaFileName), []byte(held), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := relocateJSON(t, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("relocate: %v", err)
	}
	if got := statuses(result)["feat/x"]; got != domain.RelocateStatusMovedAdopted {
		t.Fatalf("status = %q", got)
	}
	meta := repo.meta(t, "feat/x")
	if meta.SourceBranch != "main" || meta.CreatedAt == "" {
		t.Errorf("adoption not recorded: %+v", meta)
	}
	if meta.Ordinal != 3 || meta.Isolation != domain.IsolationVerbatim || len(meta.Namespaces) != 1 {
		t.Errorf("meta.json lost what it held: %+v", meta)
	}
}

func TestRelocateCharacterizeDirtySkippedUnlessForce(t *testing.T) {
	repo := newRelocateRepo(t)
	repo.create(t, "feat/dirty")
	from := filepath.Join(repo.root(), ".trees", "feat-dirty")
	if err := os.WriteFile(filepath.Join(from, "scratch.txt"), []byte("wip"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := runWtCmd(t, domain.CmdRelocate, "--to", "../.worktrees", "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("relocate: %v", err)
	}
	want := "\n" +
		"  ✓ Relocation complete  1 skipped\n" +
		"\n" +
		"  ! Skipped: feat/dirty (re-run with --force)\n" +
		"\n" +
		"  ✓ config base_path updated to \"../.worktrees\"\n" +
		"\n"
	if stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout, want)
	}
	assertExists(t, from)

	result, err := relocateJSON(t, "--"+domain.FlagForce, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("relocate --force: %v", err)
	}
	if got := statuses(result)["feat/dirty"]; got != domain.RelocateStatusMoved {
		t.Fatalf("status = %q", got)
	}
	assertGone(t, from)
	assertExists(t, filepath.Join(repo.root(), ".worktrees", "feat-dirty"))
}

func TestRelocateCharacterizeJobsBlockEvenWithForce(t *testing.T) {
	globaldir.Isolate(t)
	repo := newRelocateRepo(t)
	repo.create(t, "feat/busy")
	path := repo.gitPath(t, "feat/busy")
	if err := process.NewStateStore(process.StatePath()).Save([]domain.JobRecord{{Name: "api", WorkDir: path}}); err != nil {
		t.Fatal(err)
	}

	result, err := relocateJSON(t, "--to", "../.worktrees", "--"+domain.FlagForce, "--"+domain.FlagYes)
	if !errors.Is(err, domain.ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}
	if got := statuses(result)["feat/busy"]; got != domain.RelocateStatusBlockedJobs {
		t.Fatalf("status = %q", got)
	}
	if !result.BasePathUpdated {
		t.Errorf("a blocked worktree does not hold back the base_path rewrite: %+v", result)
	}
	assertExists(t, path)
}

func TestRelocateCharacterizeDryRunText(t *testing.T) {
	repo := newRelocateRepo(t)
	repo.create(t, "feat/a")
	external := filepath.Join(repo.root(), "manual-wt")
	repo.external(t, "feat/manual", external)

	stdout, _, err := runWtCmd(t, domain.CmdRelocate, "--to", "../.worktrees", "--"+domain.FlagDryRun)
	if err != nil {
		t.Fatalf("relocate --dry-run: %v", err)
	}
	want := "\n" +
		"  To apply (2)\n" +
		"  • feat/a → ../.worktrees/feat-a\n" +
		"  • feat/manual → ../.worktrees/feat-manual (+ adopt)\n" +
		"\n" +
		"  → 1 worktree(s) to adopt: the wizard asks each parent, --yes uses main.\n" +
		"\n" +
		"  = Dry run — no changes made.\n" +
		"\n"
	if stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout, want)
	}
	assertExists(t, external)
	assertExists(t, filepath.Join(repo.root(), ".trees", "feat-a"))
	if strings.Contains(repo.config(t), "../.worktrees") {
		t.Errorf("dry run rewrote the config")
	}
}

func TestRelocateCharacterizeDryRunJSON(t *testing.T) {
	repo := newRelocateRepo(t)
	repo.create(t, "feat/a")
	external := filepath.Join(repo.root(), "manual-wt")
	repo.external(t, "feat/manual", external)

	result, err := relocateJSON(t, "--"+domain.FlagDryRun)
	if err != nil {
		t.Fatalf("relocate --dry-run: %v", err)
	}
	got := statuses(result)
	if got["feat/a"] != domain.RelocateStatusNoop || got["feat/manual"] != domain.RelocateStatusMovedAdopted {
		t.Fatalf("statuses = %v", got)
	}
	if result.BasePath != "../.trees" || result.BasePathUpdated {
		t.Errorf("result = %+v", result)
	}
	assertExists(t, external)
	if _, err := os.Stat(filepath.Join(rules.WorktreeMetaDir(repo.stateDir, "feat/manual"), domain.MetaFileName)); !os.IsNotExist(err) {
		t.Errorf("dry run adopted feat/manual: %v", err)
	}
}
