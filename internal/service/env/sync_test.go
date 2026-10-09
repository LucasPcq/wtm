package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// readFile returns a target file's content, or "" when absent.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// entryStatus returns the status recorded for key in a file result's diff.
func entryStatus(res domain.EnvFileResult, key string) domain.EnvKeyStatus {
	for _, e := range res.Diff.Entries {
		if e.Key == key {
			return e.Status
		}
	}
	return ""
}

func baseFiles() []domain.EnvFile {
	return []domain.EnvFile{{Target: ".env", Template: ".env.example"}}
}

// TestExampleStrategyMissingUnresolved: example sources only the template, so an
// expected-but-absent key is missing_unresolved and is never added non-interactively.
func TestSyncExampleStrategyMissingUnresolved(t *testing.T) {
	main := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, ".env.example"), "API_KEY=changeme\nDB_HOST=localhost")
	writeTestFile(t, filepath.Join(wt, ".env"), "DB_HOST=localhost")

	res, err := SyncEnv(SyncEnvParams{
		Branch:       "dev",
		MainPath:     main,
		WorktreePath: wt,
		Files:        baseFiles(),
		Strategy:     domain.EnvStrategyExample,
		Mode:         domain.EnvModeAdd,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := entryStatus(res.Files[0], "API_KEY"); got != domain.EnvKeyMissing {
		t.Fatalf("API_KEY status = %q, want missing_unresolved", got)
	}
	if res.Files[0].Applied {
		t.Fatal("example strategy must not add a missing_unresolved key non-interactively")
	}
	if res.Files[0].Source != domain.EnvSourceLabelTemplate {
		t.Fatalf("source = %q, want template", res.Files[0].Source)
	}
	if got := readFile(t, filepath.Join(wt, ".env")); got != "DB_HOST=localhost" {
		t.Fatalf(".env changed unexpectedly: %q", got)
	}
}

// TestSyncMainStrategyAddsSafely: a key present in main but absent from the child
// resolves from main and is added (safe add) non-interactively.
func TestSyncMainStrategyAddsSafely(t *testing.T) {
	main := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, ".env.example"), "API_KEY=\nDB_HOST=")
	writeTestFile(t, filepath.Join(main, ".env"), "API_KEY=secret\nDB_HOST=localhost")
	writeTestFile(t, filepath.Join(wt, ".env"), "DB_HOST=localhost")

	res, err := SyncEnv(SyncEnvParams{
		Branch:       "dev",
		MainPath:     main,
		WorktreePath: wt,
		Files:        baseFiles(),
		Strategy:     domain.EnvStrategyMain,
		Mode:         domain.EnvModeAdd,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Files[0].Applied {
		t.Fatal("expected safe add to be applied")
	}
	if got := entryStatus(res.Files[0], "API_KEY"); got != domain.EnvKeyResolved {
		t.Fatalf("API_KEY status = %q, want resolved", got)
	}
	got := readFile(t, filepath.Join(wt, ".env"))
	want := "DB_HOST=localhost\nAPI_KEY=secret"
	if got != want {
		t.Fatalf(".env = %q, want %q", got, want)
	}
}

// TestSyncAddModeIgnoresConflicts: in add mode a diverging existing value is never
// a conflict and is left untouched.
func TestSyncAddModeIgnoresConflicts(t *testing.T) {
	main := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, ".env.example"), "DB_HOST=")
	writeTestFile(t, filepath.Join(main, ".env"), "DB_HOST=prod")
	writeTestFile(t, filepath.Join(wt, ".env"), "DB_HOST=localhost")

	res, err := SyncEnv(SyncEnvParams{
		Branch:       "dev",
		MainPath:     main,
		WorktreePath: wt,
		Files:        baseFiles(),
		Strategy:     domain.EnvStrategyMain,
		Mode:         domain.EnvModeAdd,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := entryStatus(res.Files[0], "DB_HOST"); got != domain.EnvKeyResolved {
		t.Fatalf("DB_HOST status = %q, want resolved (add ignores conflicts)", got)
	}
	if res.Files[0].Applied {
		t.Fatal("add mode must not touch an existing diverging value")
	}
}

// TestSyncRefreshConflictReportOnly: refresh surfaces a conflict; non-interactive
// without --on-conflict keeps the child value (report-only).
func TestSyncRefreshConflictReportOnly(t *testing.T) {
	main := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, ".env.example"), "DB_HOST=")
	writeTestFile(t, filepath.Join(main, ".env"), "DB_HOST=prod")
	writeTestFile(t, filepath.Join(wt, ".env"), "DB_HOST=localhost")

	res, err := SyncEnv(SyncEnvParams{
		Branch:       "dev",
		MainPath:     main,
		WorktreePath: wt,
		Files:        baseFiles(),
		Strategy:     domain.EnvStrategyMain,
		Mode:         domain.EnvModeRefresh,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := entryStatus(res.Files[0], "DB_HOST"); got != domain.EnvKeyConflict {
		t.Fatalf("DB_HOST status = %q, want conflict", got)
	}
	if res.Files[0].Applied {
		t.Fatal("conflict without --on-conflict must stay report-only")
	}
	if got := readFile(t, filepath.Join(wt, ".env")); got != "DB_HOST=localhost" {
		t.Fatalf(".env changed: %q", got)
	}
}

// TestSyncRefreshOverwrite: --on-conflict overwrite settles the conflict to the
// source value.
func TestSyncRefreshOverwrite(t *testing.T) {
	main := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, ".env.example"), "DB_HOST=")
	writeTestFile(t, filepath.Join(main, ".env"), "DB_HOST=prod")
	writeTestFile(t, filepath.Join(wt, ".env"), "DB_HOST=localhost")

	res, err := SyncEnv(SyncEnvParams{
		Branch:       "dev",
		MainPath:     main,
		WorktreePath: wt,
		Files:        baseFiles(),
		Strategy:     domain.EnvStrategyMain,
		Mode:         domain.EnvModeRefresh,
		OnConflict:   domain.EnvDecisionOverwrite,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Files[0].Applied {
		t.Fatal("expected overwrite to be applied")
	}
	if got := readFile(t, filepath.Join(wt, ".env")); got != "DB_HOST=prod" {
		t.Fatalf(".env = %q, want DB_HOST=prod", got)
	}
}

// TestSyncPrune: an orphan key is reported and left in place without --prune,
// removed with it.
func TestSyncPrune(t *testing.T) {
	main := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, ".env.example"), "DB_HOST=")
	writeTestFile(t, filepath.Join(main, ".env"), "DB_HOST=localhost")
	writeTestFile(t, filepath.Join(wt, ".env"), "DB_HOST=localhost\nOLD_KEY=stale")

	params := SyncEnvParams{
		Branch:       "dev",
		MainPath:     main,
		WorktreePath: wt,
		Files:        baseFiles(),
		Strategy:     domain.EnvStrategyMain,
		Mode:         domain.EnvModeAdd,
	}

	res, err := SyncEnv(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := entryStatus(res.Files[0], "OLD_KEY"); got != domain.EnvKeyOrphan {
		t.Fatalf("OLD_KEY status = %q, want orphan", got)
	}
	if res.Files[0].Applied {
		t.Fatal("orphan must not be removed without --prune")
	}

	params.Prune = true
	if _, err := SyncEnv(params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := readFile(t, filepath.Join(wt, ".env")); got != "DB_HOST=localhost" {
		t.Fatalf(".env after prune = %q, want DB_HOST=localhost", got)
	}
}

// TestSyncParentFallbackToMain: parent strategy with no parent worktree sources
// from main and flags the fallback.
func TestSyncParentFallbackToMain(t *testing.T) {
	main := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, ".env.example"), "API_KEY=")
	writeTestFile(t, filepath.Join(main, ".env"), "API_KEY=fromMain")
	// child .env absent — the whole file is drift.

	res, err := SyncEnv(SyncEnvParams{
		Branch:             "dev",
		MainPath:           main,
		WorktreePath:       wt,
		ParentWorktreePath: "", // parent absent
		Files:              baseFiles(),
		Strategy:           domain.EnvStrategyParent,
		Mode:               domain.EnvModeAdd,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Files[0].ParentFallback {
		t.Fatal("expected ParentFallback to be set when the parent has no worktree")
	}
	if res.Files[0].Source != domain.EnvSourceLabelMain {
		t.Fatalf("source = %q, want main", res.Files[0].Source)
	}
	if got := readFile(t, filepath.Join(wt, ".env")); got != "API_KEY=fromMain" {
		t.Fatalf(".env = %q, want API_KEY=fromMain", got)
	}
}

// TestSyncParentPrecedesMain: parent strategy prefers the parent worktree's value
// over main.
func TestSyncParentPrecedesMain(t *testing.T) {
	main := t.TempDir()
	parent := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, ".env.example"), "API_KEY=")
	writeTestFile(t, filepath.Join(main, ".env"), "API_KEY=fromMain")
	writeTestFile(t, filepath.Join(parent, ".env"), "API_KEY=fromParent")
	// child .env absent — the whole file is drift.

	res, err := SyncEnv(SyncEnvParams{
		Branch:             "dev",
		MainPath:           main,
		WorktreePath:       wt,
		ParentWorktreePath: parent,
		Files:              baseFiles(),
		Strategy:           domain.EnvStrategyParent,
		Mode:               domain.EnvModeAdd,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Files[0].Source != domain.EnvSourceLabelParent {
		t.Fatalf("source = %q, want %q", res.Files[0].Source, domain.EnvSourceLabelParent)
	}
	if got := readFile(t, filepath.Join(wt, ".env")); got != "API_KEY=fromParent" {
		t.Fatalf(".env = %q, want API_KEY=fromParent", got)
	}
}

// TestSyncParentFileAbsentFallsBackToMain: the parent worktree exists but does not
// contain this env file → fall back to main (aligned with create's per-file
// fileExists fallback), flagged.
func TestSyncParentFileAbsentFallsBackToMain(t *testing.T) {
	main := t.TempDir()
	parent := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, ".env.example"), "API_KEY=")
	writeTestFile(t, filepath.Join(main, ".env"), "API_KEY=fromMain")
	// parent worktree exists but has NO .env file for this target.

	res, err := SyncEnv(SyncEnvParams{
		Branch:             "dev",
		MainPath:           main,
		WorktreePath:       wt,
		ParentWorktreePath: parent,
		ParentBranch:       "feature",
		Files:              baseFiles(),
		Strategy:           domain.EnvStrategyParent,
		Mode:               domain.EnvModeAdd,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Files[0].ParentFallback {
		t.Fatal("expected ParentFallback when the parent lacks the file")
	}
	if entryStatus(res.Files[0], "API_KEY") != domain.EnvKeyResolved {
		t.Fatal("API_KEY should resolve from main via the file-absent fallback")
	}
	if got := readFile(t, filepath.Join(wt, ".env")); got != "API_KEY=fromMain" {
		t.Fatalf(".env = %q, want API_KEY=fromMain", got)
	}
}

// TestSyncParentStrictKeyMissing: the parent worktree HAS the file but lacks a key —
// that key stays missing (strict, not pulled from main).
func TestSyncParentStrictKeyMissing(t *testing.T) {
	main := t.TempDir()
	parent := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, ".env.example"), "API_KEY=\nMAIN_ONLY=")
	writeTestFile(t, filepath.Join(main, ".env"), "API_KEY=fromMain\nMAIN_ONLY=fromMain")
	writeTestFile(t, filepath.Join(parent, ".env"), "API_KEY=fromParent")

	res, err := SyncEnv(SyncEnvParams{
		Branch:             "dev",
		MainPath:           main,
		WorktreePath:       wt,
		ParentWorktreePath: parent,
		ParentBranch:       "feature",
		Files:              baseFiles(),
		Strategy:           domain.EnvStrategyParent,
		Mode:               domain.EnvModeAdd,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Files[0].ParentFallback {
		t.Fatal("no fallback expected: the parent file exists")
	}
	if entryStatus(res.Files[0], "MAIN_ONLY") != domain.EnvKeyMissing {
		t.Fatal("MAIN_ONLY should stay missing (strict — not pulled from main)")
	}
}

// TestSyncNoValueSource: a template exists but the main checkout has no copy of
// the file → every key is missing, and the source names the file the main
// checkout lacks, which is why they need a value (LUC-276).
func TestSyncNoValueSource(t *testing.T) {
	main := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, "apps/api/.env.example"), "API_KEY=changeme\nDB_HOST=localhost")
	files := []domain.EnvFile{{Target: "apps/api/.env", Template: "apps/api/.env.example"}}
	want := "template (no apps/api/.env in the main checkout)"

	for _, strat := range []domain.EnvStrategy{domain.EnvStrategyMain, domain.EnvStrategyParent} {
		res, err := SyncEnv(SyncEnvParams{
			Branch:       "dev",
			MainPath:     main,
			WorktreePath: wt,
			Files:        files,
			Strategy:     strat,
			Mode:         domain.EnvModeRefresh,
		})
		if err != nil {
			t.Fatalf("[%s] unexpected error: %v", strat, err)
		}
		if res.Files[0].Source != want {
			t.Fatalf("[%s] source = %q, want %q", strat, res.Files[0].Source, want)
		}
		if res.Files[0].ParentFallback {
			t.Fatalf("[%s] no fallback claim when there is nothing to sync from", strat)
		}
		if entryStatus(res.Files[0], "API_KEY") != domain.EnvKeyMissing {
			t.Fatalf("[%s] API_KEY should be missing", strat)
		}
	}
}

// TestSyncCheckNeverWrites: --check computes drift but writes nothing.
func TestSyncCheckNeverWrites(t *testing.T) {
	main := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, ".env.example"), "API_KEY=")
	writeTestFile(t, filepath.Join(main, ".env"), "API_KEY=secret")
	writeTestFile(t, filepath.Join(wt, ".env"), "")

	res, err := SyncEnv(SyncEnvParams{
		Branch:       "dev",
		MainPath:     main,
		WorktreePath: wt,
		Files:        baseFiles(),
		Strategy:     domain.EnvStrategyMain,
		Mode:         domain.EnvModeAdd,
		Check:        true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Files[0].Applied {
		t.Fatal("--check must not write")
	}
	if got := readFile(t, filepath.Join(wt, ".env")); got != "" {
		t.Fatalf(".env written under --check: %q", got)
	}
}

// TestApplyEnvSyncDrivesDecisions: ApplyEnvSync applies a resolution supplying a
// filled value for a missing key and an overwrite for a conflict.
func TestApplyEnvSyncDrivesDecisions(t *testing.T) {
	main := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, ".env.example"), "TOKEN=<your-token>\nDB_HOST=")
	writeTestFile(t, filepath.Join(main, ".env"), "DB_HOST=prod")
	writeTestFile(t, filepath.Join(wt, ".env"), "DB_HOST=localhost")

	res, err := ApplyEnvSync(ApplyEnvSyncParams{
		Branch:       "dev",
		MainPath:     main,
		WorktreePath: wt,
		Files:        baseFiles(),
		Strategy:     domain.EnvStrategyMain,
		Mode:         domain.EnvModeRefresh,
		Resolutions: map[string]EnvResolution{
			".env": {
				Decisions:    map[string]domain.EnvConflictDecision{"DB_HOST": domain.EnvDecisionOverwrite},
				FilledValues: map[string]string{"TOKEN": "abc123"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Files[0].Applied {
		t.Fatal("expected resolution decisions to be applied")
	}
	got := readFile(t, filepath.Join(wt, ".env"))
	want := "DB_HOST=prod\nTOKEN=abc123"
	if got != want {
		t.Fatalf(".env = %q, want %q", got, want)
	}
}

// TestComputeEnvDiffNoWrite: ComputeEnvDiff reports drift and writes nothing.
func TestComputeEnvDiffNoWrite(t *testing.T) {
	main := t.TempDir()
	wt := t.TempDir()
	writeTestFile(t, filepath.Join(wt, ".env.example"), "API_KEY=")
	writeTestFile(t, filepath.Join(main, ".env"), "API_KEY=secret")
	writeTestFile(t, filepath.Join(wt, ".env"), "OLD=1")

	files, err := ComputeEnvDiff(ComputeEnvParams{
		Branch:       "dev",
		MainPath:     main,
		WorktreePath: wt,
		Files:        baseFiles(),
		Strategy:     domain.EnvStrategyMain,
		Mode:         domain.EnvModeAdd,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if files[0].Applied {
		t.Fatal("ComputeEnvDiff must not apply")
	}
	if entryStatus(files[0], "API_KEY") != domain.EnvKeyResolved {
		t.Fatal("API_KEY should be a resolved add")
	}
	if entryStatus(files[0], "OLD") != domain.EnvKeyOrphan {
		t.Fatal("OLD should be an orphan")
	}
	if got := readFile(t, filepath.Join(wt, ".env")); got != "OLD=1" {
		t.Fatalf(".env changed: %q", got)
	}
}

// TestSyncNoFiles: a project with no configured env files errors clearly.
func TestSyncNoFiles(t *testing.T) {
	_, err := SyncEnv(SyncEnvParams{Branch: "dev", Files: nil})
	if err != domain.ErrEnvNoFiles {
		t.Fatalf("err = %v, want ErrEnvNoFiles", err)
	}
}
