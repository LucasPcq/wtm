package run

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func setupTestProject(t *testing.T) string {
	t.Helper()
	// Without this the tests read the developer's own daemon and LaunchAgents,
	// so what `run url` prints depends on whether they happen to have `run up`
	// running — which is how a green suite hid a real bug once.
	shortHome(t)
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	// Standing in the project, not in whatever checkout runs the suite: a run
	// command resolves its worktree from the current directory.
	t.Chdir(dir)

	if err := config.WriteProject(config.WriteProjectParams{
		StateDir: stateDir,
		Answers: domain.InitProjectAnswers{
			BasePath:    "../.trees",
			BaseBranch:  "main",
			EnvStrategy: domain.EnvStrategyExample,
		},
	}); err != nil {
		t.Fatalf("setup project: %v", err)
	}

	return stateDir
}

func writeRunTOML(t *testing.T, stateDir string, cfg domain.RunConfig) {
	t.Helper()
	if err := config.WriteRun(config.WriteRunParams{
		StateDir: stateDir,
		Config:   cfg,
		Force:    true,
	}); err != nil {
		t.Fatalf("write run.toml: %v", err)
	}
}

func runCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := NewCmd()
	// The root command sets both, and a subcommand built on its own does not
	// inherit them: without this cobra writes its usage onto the same stream as
	// the command's output every time a test exercises a failure.
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// TestRunExport_NotInitialized verifies the opt-in guard blocks export on an
// uninitialized run module (nothing to export) with ErrRunNotInitialized.
func TestRunExport_NotInitialized(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t, domain.CmdExport)
	if err == nil {
		t.Fatal("expected error on uninitialized run module")
	}
	if !errors.Is(err, domain.ErrRunNotInitialized) {
		t.Errorf("expected ErrRunNotInitialized, got: %v", err)
	}
}

func TestRunExportFullConfig(t *testing.T) {
	dir := setupTestProject(t)
	writeRunTOML(t, dir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev", Cwd: "."},
			{Name: "build", Kind: domain.JobKindTask, Cmd: "pnpm build"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "full", Jobs: []string{"dev", "build"}, Default: true},
		},
	})

	stdout, _, err := runCmd(t, domain.CmdExport)
	if err != nil {
		t.Fatalf("run export: %v", err)
	}

	var got domain.RunConfig
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if len(got.Jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(got.Jobs))
	}
	if got.Jobs[0].Name != "dev" {
		t.Errorf("expected jobs[0].name=dev, got %q", got.Jobs[0].Name)
	}
	if len(got.Profiles) != 1 || got.Profiles[0].Name != "full" {
		t.Errorf("expected profile 'full', got %+v", got.Profiles)
	}
}

func TestRunExportProfile(t *testing.T) {
	dir := setupTestProject(t)
	writeRunTOML(t, dir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
			{Name: "build", Kind: domain.JobKindTask, Cmd: "pnpm build"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"dev"}},
			{Name: "ci", Jobs: []string{"build"}},
		},
	})

	stdout, _, err := runCmd(t, domain.CmdExport, "--profile", "dev")
	if err != nil {
		t.Fatalf("run export --profile dev: %v", err)
	}

	var got domain.RunConfig
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if len(got.Jobs) != 1 || got.Jobs[0].Name != "dev" {
		t.Errorf("expected only job 'dev', got %+v", got.Jobs)
	}
	if len(got.Profiles) != 1 || got.Profiles[0].Name != "dev" {
		t.Errorf("expected only profile 'dev', got %+v", got.Profiles)
	}
}

func TestRunExportProfileNotFound(t *testing.T) {
	dir := setupTestProject(t)
	writeRunTOML(t, dir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		},
	})

	_, _, err := runCmd(t, domain.CmdExport, "--profile", "ghost")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("expected error to mention 'ghost', got %v", err)
	}
}

func TestRunImportKeepsNothingFromTheExistingConfig(t *testing.T) {
	dir := setupTestProject(t)
	writeRunTOML(t, dir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		},
	})

	payload := `{"job":[{"name":"build","kind":"task","cmd":"pnpm build"}],"profile":[]}`
	payloadPath := filepath.Join(t.TempDir(), "layout.json")
	os.WriteFile(payloadPath, []byte(payload), 0o644)

	stdout, _, err := runCmd(t, domain.CmdImport, payloadPath, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("run import: %v", err)
	}
	if !strings.Contains(stdout, `"build"`) {
		t.Errorf("expected 'build' in the reported jobs, got: %s", stdout)
	}

	cfg, err := config.LoadRun(dir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 || cfg.Jobs[0].Name != "build" {
		t.Errorf("import must replace, not merge: %+v", cfg.Jobs)
	}
}

func TestRunExportImportRoundtrip(t *testing.T) {
	dir := setupTestProject(t)
	original := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api-dev", Kind: domain.JobKindService, Cmd: "pnpm run dev", Cwd: "apps/api"},
			{Name: "web-build", Kind: domain.JobKindTask, Cmd: "pnpm run build", Cwd: "apps/web"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api-dev"}, Default: true},
		},
	}
	writeRunTOML(t, dir, original)

	stdout, _, err := runCmd(t, domain.CmdExport)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	layoutPath := filepath.Join(t.TempDir(), "layout.json")
	if err := os.WriteFile(layoutPath, []byte(stdout), 0o644); err != nil {
		t.Fatalf("write layout: %v", err)
	}

	os.Remove(filepath.Join(dir, domain.RunFileName))

	_, _, err = runCmd(t, domain.CmdImport, layoutPath, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	restored, err := config.LoadRun(dir)
	if err != nil {
		t.Fatalf("load restored: %v", err)
	}
	if len(restored.Jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(restored.Jobs))
	}
	if len(restored.Profiles) != 1 {
		t.Errorf("expected 1 profile, got %d", len(restored.Profiles))
	}
	if restored.Jobs[0].Cwd != "apps/api" {
		t.Errorf("expected cwd=apps/api, got %q", restored.Jobs[0].Cwd)
	}
}

func TestRunImportInvalidJSON(t *testing.T) {
	setupTestProject(t)

	badPath := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(badPath, []byte("{bad json"), 0o644)

	_, _, err := runCmd(t, domain.CmdImport, badPath)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !errors.Is(err, err) || !strings.Contains(err.Error(), "parse JSON") {
		t.Errorf("expected parse error, got %v", err)
	}
}

func TestRunImportInvalidConfig(t *testing.T) {
	setupTestProject(t)

	invalidPath := filepath.Join(t.TempDir(), "invalid.json")
	os.WriteFile(invalidPath, []byte(`{"job":[{"name":"bad","kind":"task","cmd":"echo","stop":"echo nope"}],"profile":[]}`), 0o644)

	_, _, err := runCmd(t, domain.CmdImport, invalidPath)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "tasks cannot declare a stop command") {
		t.Errorf("expected validation message, got %v", err)
	}
}

// run export is machine output, like run url: it takes --output as its sibling
// does, and answers with the same document whatever the format, never framed.
func TestRunExportAcceptsOutputLikeRunURL(t *testing.T) {
	dir := setupTestProject(t)
	writeRunTOML(t, dir, domain.RunConfig{Jobs: []domain.JobConfig{{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"}}})

	plain, _, err := runCmd(t, domain.CmdExport)
	if err != nil {
		t.Fatalf("run export: %v", err)
	}
	for _, format := range []string{domain.OutputJSON, domain.OutputText} {
		stdout, _, err := runCmd(t, domain.CmdExport, "--"+domain.FlagOutput, format)
		if err != nil {
			t.Fatalf("run export --output %s: %v", format, err)
		}
		if stdout != plain {
			t.Errorf("--output %s = %q, want the same document as without it", format, stdout)
		}
	}
}
