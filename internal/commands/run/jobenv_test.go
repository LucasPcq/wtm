package run

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// startEnv is the environment the commands asked the daemon to start the named
// job with — what a job actually receives, read off the wire.
func (d *fakeDaemon) startEnv(t *testing.T, job string) map[string]string {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, req := range d.requests {
		if req.Action == process.ActionStart && req.Job != nil && req.Job.Name == job {
			return req.Env
		}
	}
	t.Fatalf("the daemon was never asked to start %q", job)
	return nil
}

// startedJob is the job config the commands put on the wire, ports included.
func (d *fakeDaemon) startedJob(t *testing.T, job string) domain.JobConfig {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, req := range d.requests {
		if req.Action == process.ActionStart && req.Job != nil && req.Job.Name == job {
			return *req.Job
		}
	}
	t.Fatalf("the daemon was never asked to start %q", job)
	return domain.JobConfig{}
}

func addWorktree(t *testing.T, projectDir, branch string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), strings.ReplaceAll(branch, "/", "-"))
	cmd := exec.Command("git", "worktree", "add", "-q", "-b", branch, path, "HEAD")
	cmd.Dir = projectDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %s: %v", out, err)
	}
	// Created the way `wtm create` creates one: with its isolation chosen. A
	// worktree without one is refused a start until it chooses.
	if err := worktree.SetIsolation(worktree.SetIsolationParams{
		Ref:       worktree.WorktreeRef{ProjectDir: projectDir, StateDir: filepath.Join(projectDir, ".git", "wtm"), Branch: branch},
		Isolation: domain.IsolationIsolated,
	}); err != nil {
		t.Fatalf("record isolation: %v", err)
	}
	return path
}

func enterWorktree(t *testing.T, path string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(path); err != nil {
		t.Fatalf("chdir %q: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

// composeProject is the name wtm derives for a worktree of the test project:
// the repository's own directory qualifies it, so two clones on the same branch
// do not share a stack.
func composeProject(t *testing.T, worktreeSlug string) string {
	t.Helper()
	return rules.ComposeProjectName(rules.ComposeProjectNameParams{
		Project:  filepath.Base(os.Getenv("WTM_PROJECT_DIR")),
		Worktree: worktreeSlug,
	})
}

func assertEnv(t *testing.T, env map[string]string, want map[string]string) {
	t.Helper()
	for key, value := range want {
		if env[key] != value {
			t.Errorf("%s = %q, want %q", key, env[key], value)
		}
	}
}

func TestRunUpInjectsMainWorktreeEnv(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	fakeTTY(t, false)
	enterWorktree(t, os.Getenv("WTM_PROJECT_DIR"))

	if _, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagDetach); err != nil {
		t.Fatalf("run up: %v", err)
	}

	// The main checkout keeps the project's default ports, and a compose project
	// named after the repository alone: it hosts the shared stack, which must not
	// move with its branch.
	assertEnv(t, daemon.startEnv(t, "docker"), map[string]string{
		domain.EnvBranch:             "main",
		domain.EnvWorktree:           "main",
		domain.EnvOrdinal:            "0",
		domain.EnvPortOffset:         "0",
		domain.EnvComposeProjectName: rules.WorktreeSlug(filepath.Base(os.Getenv("WTM_PROJECT_DIR"))),
	})
}

func TestRunUpInjectsLinkedWorktreeEnv(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	fakeTTY(t, false)
	enterWorktree(t, addWorktree(t, os.Getenv("WTM_PROJECT_DIR"), "feat/x"))

	if _, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagDetach); err != nil {
		t.Fatalf("run up: %v", err)
	}

	assertEnv(t, daemon.startEnv(t, "docker"), map[string]string{
		domain.EnvBranch:             "feat/x",
		domain.EnvWorktree:           "feat-x",
		domain.EnvOrdinal:            "1",
		domain.EnvPortOffset:         "10",
		domain.EnvComposeProjectName: composeProject(t, "feat-x"),
	})
}

// run start bypasses flow/runlogs entirely; it must carry the same env.
func TestRunStartInjectsWorktreeEnv(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	fakeTTY(t, false)
	enterWorktree(t, addWorktree(t, os.Getenv("WTM_PROJECT_DIR"), "feat/y"))

	if _, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api", "--"+domain.FlagDetach); err != nil {
		t.Fatalf("run start: %v", err)
	}

	assertEnv(t, daemon.startEnv(t, "api"), map[string]string{
		domain.EnvBranch:     "feat/y",
		domain.EnvOrdinal:    "1",
		domain.EnvPortOffset: "10",
	})
}

// The shell a command is typed in may belong to another worktree, or be a
// job's: its COMPOSE_PROJECT_NAME would recreate this worktree's stack under
// that one's name.
func TestRunUpIgnoresTheCallersComposeProjectName(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	fakeTTY(t, false)
	enterWorktree(t, addWorktree(t, os.Getenv("WTM_PROJECT_DIR"), "feat/z"))
	t.Setenv(domain.EnvComposeProjectName, "launched-from-another-worktree")

	if _, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagDetach); err != nil {
		t.Fatalf("run up: %v", err)
	}

	assertEnv(t, daemon.startEnv(t, "docker"), map[string]string{
		domain.EnvComposeProjectName: composeProject(t, "feat-z"),
		domain.EnvOrdinal:            "1",
	})
}

// The block is what turns an ordinal into an offset, so a project that widens
// it must see WTM_PORT_OFFSET widen with it — the value is resolved by the
// client, which is also the side that can read run.toml.
func TestRunUpHonoursDeclaredPortOffsetBlock(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		PortOffsetBlock: 100,
		Jobs:            []domain.JobConfig{apiJob},
		Profiles:        []domain.ProfileConfig{{Name: "dev", Jobs: []string{"api"}, Default: true}},
	})
	daemon := startFakeDaemon(t, &fakeDaemon{})
	fakeTTY(t, false)
	enterWorktree(t, addWorktree(t, os.Getenv("WTM_PROJECT_DIR"), "feat/block"))

	if _, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagDetach); err != nil {
		t.Fatalf("run up: %v", err)
	}

	assertEnv(t, daemon.startEnv(t, "api"), map[string]string{
		domain.EnvOrdinal:    "1",
		domain.EnvPortOffset: "100",
	})
}

// The ports themselves are resolved daemon-side, so what the client must get
// right is sending the declaration and the offset that pair with each other.
func TestRunUpSendsDeclaredPortsWithTheOffset(t *testing.T) {
	stateDir := setupTestProject(t)
	webJob := domain.JobConfig{
		Name:  "web",
		Kind:  domain.JobKindService,
		Cmd:   "pnpm dev",
		Ports: map[string]int{"PORT": 3000},
	}
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs:     []domain.JobConfig{webJob},
		Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"web"}, Default: true}},
	})
	daemon := startFakeDaemon(t, &fakeDaemon{})
	fakeTTY(t, false)
	enterWorktree(t, addWorktree(t, os.Getenv("WTM_PROJECT_DIR"), "feat/ports"))

	if _, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagDetach); err != nil {
		t.Fatalf("run up: %v", err)
	}

	assertEnv(t, daemon.startEnv(t, "web"), map[string]string{domain.EnvPortOffset: "10"})
	if got := daemon.startedJob(t, "web").Ports["PORT"]; got != 3000 {
		t.Errorf("the daemon was sent base %d, want the declared 3000", got)
	}
}

// A worktree created before the isolation choice still runs on its source's
// ports. Starting it isolated would move its jobs off them behind the user's
// back, so the start is refused until the choice is made — and resolving
// nothing allocates it no ordinal.
func TestRunStartRefusesAWorktreeThatNeverChoseItsIsolation(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	fakeTTY(t, false)
	projectDir := os.Getenv("WTM_PROJECT_DIR")
	path := filepath.Join(t.TempDir(), "feat-old")
	cmd := exec.Command("git", "worktree", "add", "-q", "-b", "feat/old", path, "HEAD")
	cmd.Dir = projectDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %s: %v", out, err)
	}

	_, _, err := runCmd(t, domain.CmdStart, "feat/old", "--"+domain.FlagJob, "api", "--"+domain.FlagDetach)
	if !errors.Is(err, domain.ErrIsolationAdoptionPending) {
		t.Fatalf("error = %v, want the isolation choice asked for", err)
	}
	for _, want := range []string{"wtm env feat/old --isolation isolated", "verbatim"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not name %q", err, want)
		}
	}
	if started := daemon.startedJobs(); len(started) != 0 {
		t.Errorf("started %v", started)
	}
	if _, err := os.Stat(rules.WorktreeMetaDir(os.Getenv("WTM_STATE_DIR"), "feat/old")); !os.IsNotExist(err) {
		t.Errorf("metadata written for the refused worktree (%v): an ordinal was allocated", err)
	}
}
