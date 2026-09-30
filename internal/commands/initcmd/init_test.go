package initcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

func freshProject(t *testing.T) string {
	t.Helper()
	globaldir.Isolate(t)
	dir := gittest.InitRepo(t)
	t.Chdir(dir)
	return dir
}

func runInitCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func projectConfigPath(t *testing.T, dir string) string {
	t.Helper()
	return filepath.Join(dir, ".git", domain.StateDirName, domain.ConfigFileName)
}

func assertBootstrapped(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(config.GlobalPath()); err != nil {
		t.Errorf("global config not written: %v", err)
	}
	if _, err := os.Stat(projectConfigPath(t, dir)); err != nil {
		t.Errorf("project config not written: %v", err)
	}
}

func TestInitYesBootstrapsAFreshHomeWithoutPrompting(t *testing.T) {
	for _, flag := range []string{"--" + domain.FlagYes, "-y"} {
		t.Run(flag, func(t *testing.T) {
			dir := freshProject(t)

			if out, err := runInitCmd(t, flag); err != nil {
				t.Fatalf("init %s: %v\n%s", flag, err, out)
			}
			assertBootstrapped(t, dir)
		})
	}
}

func TestInitWithoutATerminalBootstrapsFromDetection(t *testing.T) {
	dir := freshProject(t)

	if out, err := runInitCmd(t); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	assertBootstrapped(t, dir)
}

func TestInitNoLongerAcceptsNonInteractive(t *testing.T) {
	freshProject(t)

	_, err := runInitCmd(t, "--non-interactive")
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("err = %v, want an unknown flag", err)
	}
}

func TestInitOnlyWithYesRegeneratesTheSectionWithoutPrompting(t *testing.T) {
	dir := freshProject(t)
	if out, err := runInitCmd(t, "--"+domain.FlagYes); err != nil {
		t.Fatalf("bootstrap: %v\n%s", err, out)
	}

	out, err := runInitCmd(t, "--"+domain.FlagOnly, domain.SectionEnv, "--"+domain.FlagYes, "--"+domain.FlagEnvStrategy, string(domain.EnvStrategyMain))
	if err != nil {
		t.Fatalf("re-init: %v\n%s", err, out)
	}
	cfg, err := config.LoadProjectRaw(filepath.Dir(projectConfigPath(t, dir)))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env.Strategy != domain.EnvStrategyMain {
		t.Errorf("env strategy = %q, want %q", cfg.Env.Strategy, domain.EnvStrategyMain)
	}
}
