package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/process"
)

func (d *fakeDaemon) startWorkDir(t *testing.T, job string) string {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, req := range d.requests {
		if req.Action == process.ActionStart && req.Job != nil && req.Job.Name == job {
			return req.WorkDir
		}
	}
	t.Fatalf("no start request for %q", job)
	return ""
}

// The positional must carry the whole worktree with it, not just the name: the
// env a job is handed comes from the target's ordinal, so a run that resolved
// the branch but kept the current directory would start the job with the wrong
// ports and nothing would say so.
func TestRunStartOnANamedWorktreeUsesItsEnv(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	fakeTTY(t, false)
	other := addWorktree(t, os.Getenv(domain.EnvProjectDir), "feat/elsewhere")

	if _, _, err := runCmd(t, domain.CmdStart, "feat/elsewhere", "--"+domain.FlagJob, "api", "--"+domain.FlagDetach); err != nil {
		t.Fatalf("run start feat/elsewhere: %v", err)
	}

	assertEnv(t, daemon.startEnv(t, "api"), map[string]string{
		domain.EnvBranch: "feat/elsewhere",
	})
	if got, want := daemon.startWorkDir(t, "api"), infra.ResolvePath(other); infra.ResolvePath(got) != want {
		t.Errorf("the job was started in %q, want the named worktree %q", got, want)
	}
}

// WorkDir is both the daemon's key for a job (name + WorkDir, compared as a
// string) and the directory the job runs in, which run.toml's `cwd` resolves
// against. A subdirectory would therefore split one worktree into two keys —
// `run down` could not find what `run up` started — and mis-resolve every
// relative `cwd`. The target is always the worktree, whichever of its
// directories the command was launched from.
func TestRunStartFromASubdirectoryKeysOnTheWorktree(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	fakeTTY(t, false)

	worktreePath := addWorktree(t, os.Getenv(domain.EnvProjectDir), "feat/deep")
	nested := filepath.Join(worktreePath, "packages", "api")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	enterWorktree(t, nested)

	if _, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api", "--"+domain.FlagDetach); err != nil {
		t.Fatalf("run start: %v", err)
	}

	got := infra.ResolvePath(daemon.startWorkDir(t, "api"))
	if want := infra.ResolvePath(worktreePath); got != want {
		t.Errorf("the job was keyed on %q, want the worktree root %q", got, want)
	}
}

// The two ways of naming the same worktree — the positional and the current
// directory — must produce the same key, or a job started one way cannot be
// stopped the other.
func TestNamedAndCurrentWorktreeAgreeOnTheKey(t *testing.T) {
	setupUpProject(t, &fakeDaemon{})
	fakeTTY(t, false)

	worktreePath := addWorktree(t, os.Getenv(domain.EnvProjectDir), "feat/same")
	projectDir := os.Getenv(domain.EnvProjectDir)

	resolved, err := target.Named(t.Context(), target.ResolveParams{ProjectDir: projectDir, Query: "feat/same"})
	if err != nil {
		t.Fatalf("resolve named: %v", err)
	}
	named := target.WorkDir(t.Context(), target.WorkDirParams{Named: resolved, Cwd: projectDir})

	enterWorktree(t, worktreePath)
	current := target.WorkDir(t.Context(), target.WorkDirParams{Cwd: worktreePath})

	if named != current {
		t.Errorf("naming the worktree gives %q, standing in it gives %q — the daemon would see two jobs", named, current)
	}
}
