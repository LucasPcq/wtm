package hooks

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestRunHooksSuccess(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "echo hello"}},
		WorkDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunHooksFailure(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "false"}},
		WorkDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error from failing command")
	}
}

func TestRunHooksStopsOnFirstError(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks: []domain.HookCommand{
			{Cmd: "false"},
			{Cmd: "echo should-not-run"},
		},
		WorkDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunHooksContinueOnError(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks: []domain.HookCommand{
			{Cmd: "false", ContinueOnError: true},
			{Cmd: "echo should-run"},
		},
		WorkDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected no error with continue_on_error: %v", err)
	}
}

func TestRunHooksEmptyList(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks:   nil,
		WorkDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("empty hooks should be no-op: %v", err)
	}
}

// TestRunHooksRelativeCwdJoinsWorkDir locks the documented contract: a relative
// cwd is resolved against the worktree root, not the process working directory.
// The marker file only exists under WorkDir, so the hook fails if cwd is used raw.
func TestRunHooksRelativeCwdJoinsWorkDir(t *testing.T) {
	workDir := t.TempDir()
	sub := filepath.Join(workDir, "apps", "api")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "marker.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The marker lives only in the sub-directory, so a hook without cwd must fail:
	// that is what makes the cwd assertion below meaningful.
	err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "cat marker.txt"}},
		WorkDir: workDir,
		Output:  io.Discard,
	})
	if err == nil {
		t.Fatal("sanity: hook without cwd should not find the sub-directory marker")
	}

	err = RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "cat marker.txt", Cwd: filepath.Join("apps", "api")}},
		WorkDir: workDir,
		Output:  io.Discard,
	})
	if err != nil {
		t.Fatalf("relative cwd should resolve under WorkDir: %v", err)
	}
}

// TestRunHooksAbsoluteCwdUsedAsIs guards the other branch — an absolute cwd
// (including one produced by interpolating {{worktree}}) is never joined.
func TestRunHooksAbsoluteCwdUsedAsIs(t *testing.T) {
	elsewhere := t.TempDir()
	if err := os.WriteFile(filepath.Join(elsewhere, "marker.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "cat marker.txt", Cwd: elsewhere}},
		WorkDir: t.TempDir(),
		Output:  io.Discard,
	})
	if err != nil {
		t.Fatalf("absolute cwd should be used as-is: %v", err)
	}
}

func TestInterpolate(t *testing.T) {
	vars := rules.TemplateVars{
		Worktree:   "/trees/feat-auth",
		Branch:     "feature/auth",
		Root:       "/repo",
		FromBranch: "main",
	}

	tests := []struct {
		input string
		want  string
	}{
		{"echo {{branch}}", "echo feature/auth"},
		{"cp {{root}}/.env {{worktree}}/.env", "cp /repo/.env /trees/feat-auth/.env"},
		{"echo from {{from_branch}}", "echo from main"},
		{"no vars here", "no vars here"},
		{"", ""},
	}

	for _, tt := range tests {
		got := rules.Interpolate(tt.input, vars)
		if got != tt.want {
			t.Errorf("Interpolate(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveTemplateVars(t *testing.T) {
	vars := rules.TemplateVars{
		Worktree: "/trees/feat",
		Branch:   "feat",
	}

	hook := domain.HookCommand{
		Cmd: "echo {{branch}}",
		Cwd: "{{worktree}}/apps",
	}

	resolved := rules.ResolveTemplateVars(hook, vars)

	if resolved.Cmd != "echo feat" {
		t.Errorf("cmd: expected 'echo feat', got %q", resolved.Cmd)
	}
	if resolved.Cwd != "/trees/feat/apps" {
		t.Errorf("cwd: expected '/trees/feat/apps', got %q", resolved.Cwd)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		ms   int
		want string
	}{
		{50, "50ms"},
		{500, "500ms"},
		{1500, "1.5s"},
		{10000, "10.0s"},
	}

	for _, tt := range tests {
		got := formatDuration(time.Duration(tt.ms) * time.Millisecond)
		if !strings.Contains(got, tt.want[:len(tt.want)-1]) {
			t.Errorf("formatDuration(%dms) = %q, want ~%q", tt.ms, got, tt.want)
		}
	}
}
