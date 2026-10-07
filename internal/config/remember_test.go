package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestLoadReadsTheRememberedAnswers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, domain.ConfigFileName, minimalToml+"\n[wizard.remembered]\nenv_strategy = \"parent\"\nisolation = \"verbatim\"\n")

	cfg, err := Load(LoadParams{StateDir: dir})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	remembered := cfg.Project.Wizard.Remembered
	if remembered[domain.RememberEnvStrategy] != "parent" || remembered[domain.RememberIsolation] != "verbatim" {
		t.Errorf("remembered = %v, want env_strategy=parent and isolation=verbatim", remembered)
	}
}

func TestLoadRefusesARememberedAnswerNoQuestionAsks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, domain.ConfigFileName, minimalToml+"\n[wizard.remembered]\ndelete = \"force\"\n")

	if _, err := Load(LoadParams{StateDir: dir}); err == nil || !strings.Contains(err.Error(), `"delete"`) {
		t.Errorf("err = %v, want the unknown id refused at load", err)
	}
}

// Every writer of config.toml re-renders it whole — relocate, `init --only`, the
// env targets of `run init` — so the memory must survive a rewrite it took no
// part in.
func TestARewriteKeepsTheRememberedAnswers(t *testing.T) {
	dir := t.TempDir()
	cfg := domain.ProjectConfig{
		Worktrees: domain.WorktreesConfig{BasePath: "../.trees", BaseBranch: "main"},
		Env:       domain.EnvConfig{Strategy: domain.EnvStrategyExample},
		Wizard:    domain.WizardConfig{Remembered: map[string]string{domain.RememberIsolation: "verbatim", domain.RememberEnvStrategy: "main"}},
	}
	if err := WriteProjectConfig(WriteProjectConfigParams{StateDir: dir, Config: cfg}); err != nil {
		t.Fatalf("WriteProjectConfig: %v", err)
	}

	got, err := LoadProjectRaw(dir)
	if err != nil {
		t.Fatalf("LoadProjectRaw: %v", err)
	}
	if len(got.Wizard.Remembered) != 2 || got.Wizard.Remembered[domain.RememberIsolation] != "verbatim" {
		t.Errorf("remembered = %v, want both answers kept", got.Wizard.Remembered)
	}
}

func TestNothingRememberedWritesNoSection(t *testing.T) {
	dir := t.TempDir()
	cfg := domain.ProjectConfig{Worktrees: domain.WorktreesConfig{BasePath: "../.trees", BaseBranch: "main"}}
	if err := WriteProjectConfig(WriteProjectConfigParams{StateDir: dir, Config: cfg}); err != nil {
		t.Fatalf("WriteProjectConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, domain.ConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "[wizard") {
		t.Errorf("config.toml holds a wizard section with nothing remembered:\n%s", data)
	}
}
