package initcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/events"
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

// The hints a second init prints share one column for their notes.
func TestInitOnAnInitialisedProjectAlignsItsNextSteps(t *testing.T) {
	freshProject(t)
	if out, err := runInitCmd(t, "--"+domain.FlagYes); err != nil {
		t.Fatalf("first init: %v\n%s", err, out)
	}

	out, err := runInitCmd(t, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("second init: %v\n%s", err, out)
	}
	columns := map[int]bool{}
	for _, note := range []string{domain.InitReconfigureNote, domain.InitEditNote, domain.InitRunInitNote} {
		for _, line := range strings.Split(out, "\n") {
			if i := strings.Index(line, note); i >= 0 {
				columns[i] = true
			}
		}
	}
	if len(columns) != 1 {
		t.Errorf("notes start on %d different columns:\n%s", len(columns), out)
	}
}

func TestInitFromASubdirectoryRegistersTheRepositoryRoot(t *testing.T) {
	dir := freshProject(t)
	sub := filepath.Join(dir, "packages", "api")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	if out, err := runInitCmd(t, "--"+domain.FlagYes); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}

	root, _ := filepath.EvalSymlinks(dir)
	if repos, _ := events.Registered(); len(repos) != 1 || repos[0].Root != root {
		t.Fatalf("registered %+v, want the root %s", repos, root)
	}
}
