package initcmd

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

// newReinitTestCmd returns a cobra command carrying the string flags
// buildReinitAnswers reads, with output captured to a discardable buffer.
func newReinitTestCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String(domain.FlagBaseBranch, "", "")
	cmd.Flags().String(domain.FlagEnvStrategy, "", "")
	cmd.Flags().String(domain.FlagInstallCommand, "", "")
	cmd.SetOut(&bytes.Buffer{})
	return cmd
}

// seedProjectConfig writes a full config.toml into dir for re-init tests.
func seedProjectConfig(t *testing.T, dir string, cfg domain.ProjectConfig) {
	t.Helper()
	if err := config.WriteProjectConfig(config.WriteProjectConfigParams{StateDir: dir, Config: cfg}); err != nil {
		t.Fatalf("seed project config: %v", err)
	}
}

func TestParseSections(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr bool
	}{
		{name: "single", input: []string{"env"}, want: []string{"env"}},
		{name: "worktrees", input: []string{"worktrees"}, want: []string{"worktrees"}},
		{name: "csv", input: []string{"env,hooks"}, want: []string{"env", "hooks"}},
		{name: "repeated", input: []string{"env", "hooks"}, want: []string{"env", "hooks"}},
		{name: "dedup", input: []string{"env", "env"}, want: []string{"env"}},
		{name: "trim", input: []string{" env , hooks "}, want: []string{"env", "hooks"}},
		{name: "services redirected", input: []string{"services"}, wantErr: true},
		{name: "unknown", input: []string{"nope"}, wantErr: true},
		{name: "mixed valid+invalid", input: []string{"env,bogus"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSections(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %v", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// fullSeededConfig is a config.toml with every section populated, used to assert
// that re-initializing one section leaves the others byte-for-byte intact.
func fullSeededConfig() domain.ProjectConfig {
	return domain.ProjectConfig{
		Worktrees: domain.WorktreesConfig{BasePath: "../.trees", BaseBranch: "develop"},
		Env:       domain.EnvConfig{Strategy: domain.EnvStrategyParent, Files: []domain.EnvFile{{Target: ".env"}}},
		Hooks:     domain.HooksConfig{OnCreate: []domain.HookCommand{{Cmd: "pnpm install"}}},
	}
}

func TestApplyConfigReinitOnlyRewritesRequestedSection(t *testing.T) {
	dir := t.TempDir()
	seedProjectConfig(t, dir, fullSeededConfig())

	answers := domain.InitProjectAnswers{
		// Values for sections NOT requested must be ignored.
		BaseBranch: "should-not-apply",
		OnCreate:   []domain.HookCommand{{Cmd: "should-not-apply"}},
		// The env section is the only one requested.
		EnvStrategy: domain.EnvStrategyExample,
		EnvFiles:    []domain.EnvFile{{Target: ".env.local", Local: true}},
	}

	if err := applyConfigReinit(applyReinitParams{
		Out:      io.Discard,
		StateDir: dir,
		Sections: []string{domain.SectionEnv},
		Answers:  answers,
	}); err != nil {
		t.Fatalf("applyConfigReinit: %v", err)
	}

	got, err := config.LoadProjectRaw(dir)
	if err != nil {
		t.Fatalf("LoadProjectRaw: %v", err)
	}

	// Requested section was rewritten.
	if got.Env.Strategy != domain.EnvStrategyExample {
		t.Errorf("env.strategy: got %q, want %q", got.Env.Strategy, domain.EnvStrategyExample)
	}
	if len(got.Env.Files) != 1 || got.Env.Files[0].Target != ".env.local" {
		t.Errorf("env.file: got %+v", got.Env.Files)
	}

	// Every untouched section preserved its on-disk value.
	if got.Worktrees.BaseBranch != "develop" {
		t.Errorf("worktrees.base_branch clobbered: %q", got.Worktrees.BaseBranch)
	}
	if len(got.Hooks.OnCreate) != 1 || got.Hooks.OnCreate[0].Cmd != "pnpm install" {
		t.Errorf("hooks not preserved: %+v", got.Hooks.OnCreate)
	}
}

func TestBuildReinitAnswersKeepsCurrentConfigUnlessFlagOverrides(t *testing.T) {
	dir := t.TempDir()
	seedProjectConfig(t, dir, fullSeededConfig())

	t.Run("no flags keep current values", func(t *testing.T) {
		got, err := buildReinitAnswers(newReinitTestCmd(), dir, domain.InitDetectionResult{})
		if err != nil {
			t.Fatalf("buildReinitAnswers: %v", err)
		}
		if got.BaseBranch != "develop" {
			t.Errorf("base branch: got %q, want develop", got.BaseBranch)
		}
		if got.EnvStrategy != domain.EnvStrategyParent {
			t.Errorf("env strategy: got %q, want parent", got.EnvStrategy)
		}
		if len(got.OnCreate) != 1 || got.OnCreate[0].Cmd != "pnpm install" {
			t.Errorf("install command not carried from hooks: %+v", got.OnCreate)
		}
	})

	t.Run("flags override current values", func(t *testing.T) {
		cmd := newReinitTestCmd()
		_ = cmd.Flags().Set(domain.FlagBaseBranch, "feature")
		_ = cmd.Flags().Set(domain.FlagEnvStrategy, string(domain.EnvStrategyMain))
		_ = cmd.Flags().Set(domain.FlagInstallCommand, "yarn install")

		got, err := buildReinitAnswers(cmd, dir, domain.InitDetectionResult{})
		if err != nil {
			t.Fatalf("buildReinitAnswers: %v", err)
		}
		if got.BaseBranch != "feature" {
			t.Errorf("base branch override: got %q", got.BaseBranch)
		}
		if got.EnvStrategy != domain.EnvStrategyMain {
			t.Errorf("env strategy override: got %q", got.EnvStrategy)
		}
		if len(got.OnCreate) != 1 || got.OnCreate[0].Cmd != "yarn install" {
			t.Errorf("install command override: %+v", got.OnCreate)
		}
	})
}
