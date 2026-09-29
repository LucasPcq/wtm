package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func breakRunTOML(t *testing.T, stateDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(stateDir, domain.RunFileName), []byte("bogus_key = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func projectDirOf(stateDir string) string { return filepath.Dir(filepath.Dir(stateDir)) }

// gitToplevel spells a worktree the way the run commands key it on the daemon.
func gitToplevel(t *testing.T, dir string) string {
	t.Helper()
	top, err := infra.Toplevel(dir)
	if err != nil {
		t.Fatal(err)
	}
	return top
}

func (d *fakeDaemon) actions() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	actions := make([]string, 0, len(d.requests))
	for _, req := range d.requests {
		actions = append(actions, string(req.Action)+" "+req.Name)
	}
	return actions
}

// G1: stopping must always be possible, so a run.toml that cannot be read is
// warned about and the worktree emptied anyway.
func TestRunDownStopsOverAnInvalidRunToml(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{Jobs: []domain.JobInfo{{Name: "api", Status: domain.JobStatusRunning}}})
	stateDir := os.Getenv("WTM_STATE_DIR")
	breakRunTOML(t, stateDir)
	t.Chdir(projectDirOf(stateDir))
	fakeTTY(t, false)

	_, stderr, err := runCmd(t, domain.CmdDown, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("run down must not fail over run.toml: %v", err)
	}
	if !strings.Contains(stderr, "bogus_key") {
		t.Errorf("stderr = %q, want the unreadable run.toml named", stderr)
	}
	if !strings.Contains(strings.Join(daemon.actions(), ","), string(process.ActionStopAll)) {
		t.Errorf("daemon saw %v, want the worktree emptied", daemon.actions())
	}
}

func TestRunStopStopsByNameOverAnInvalidRunToml(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	stateDir := os.Getenv("WTM_STATE_DIR")
	breakRunTOML(t, stateDir)
	t.Chdir(projectDirOf(stateDir))
	fakeTTY(t, false)
	daemon.setJobs([]domain.JobInfo{{Name: "api", Status: domain.JobStatusRunning, WorkDir: gitToplevel(t, projectDirOf(stateDir))}})

	_, stderr, err := runCmd(t, domain.CmdStop, "--"+domain.FlagJob, "api", "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("run stop must not fail over run.toml: %v", err)
	}
	if !strings.Contains(stderr, "bogus_key") {
		t.Errorf("stderr = %q, want the unreadable run.toml named", stderr)
	}
	if !strings.Contains(strings.Join(daemon.actions(), ","), string(process.ActionStop)+" api") {
		t.Errorf("daemon saw %v, want api stopped", daemon.actions())
	}
}

func TestRunPsListsOverAnInvalidRunToml(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})
	stateDir := os.Getenv("WTM_STATE_DIR")
	breakRunTOML(t, stateDir)
	t.Chdir(projectDirOf(stateDir))

	if _, _, err := runCmd(t, domain.CmdPs, "--output", domain.OutputJSON); err != nil {
		t.Fatalf("run ps must not fail over run.toml: %v", err)
	}
}

// Starting remains refused: run.toml is what `up` and `start` are about.
func TestRunStartStillRefusesAnInvalidRunToml(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	stateDir := os.Getenv("WTM_STATE_DIR")
	breakRunTOML(t, stateDir)
	t.Chdir(projectDirOf(stateDir))
	fakeTTY(t, false)

	if _, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api", "-d", "--"+domain.FlagYes); err == nil {
		t.Fatal("run start must refuse a run.toml it cannot read")
	}
	if got := daemon.startedJobs(); len(got) != 0 {
		t.Errorf("started %v, want nothing", got)
	}
}

// D5: a worktree whose environment cannot be resolved is refused by name,
// never started on offset 0 — the main checkout's ports.
func TestRunRefusesAWorktreeWhoseEnvCannotBeResolved(t *testing.T) {
	for _, args := range [][]string{
		{domain.CmdStart, "--" + domain.FlagJob, "api", "-d", "--" + domain.FlagYes},
		{domain.CmdUp, "-d", "--" + domain.FlagYes},
	} {
		t.Run(args[0], func(t *testing.T) {
			daemon := setupStartProject(t, &fakeDaemon{})
			detached := filepath.Join(t.TempDir(), "detached")
			gittest.Git(t, projectDirOf(os.Getenv("WTM_STATE_DIR")), "worktree", "add", "--detach", detached)
			t.Chdir(detached)
			fakeTTY(t, false)

			_, _, err := runCmd(t, args...)
			if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
				t.Fatalf("err = %v, want the detached HEAD named", err)
			}
			if got := daemon.startedJobs(); len(got) != 0 {
				t.Errorf("started %v on another worktree's ports, want nothing", got)
			}
		})
	}
}

// The proxy port is the run module's: a value no socket can bind is refused by
// the commands that would start it, naming the key, instead of a daemon that
// silently serves no name.
func TestRunUpRefusesAProxyPortOutOfRange(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})
	stateDir := os.Getenv("WTM_STATE_DIR")
	t.Chdir(projectDirOf(stateDir))
	fakeTTY(t, false)

	path := config.GlobalPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("shell = \"zsh\"\n\n[proxy]\nport = 70000\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagYes)
	if err == nil || !strings.Contains(err.Error(), "70000") {
		t.Fatalf("run up = %v, want a refusal naming the port", err)
	}
}
