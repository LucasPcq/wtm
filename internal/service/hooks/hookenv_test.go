package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestRunHooksInjectsWorktreeEnv(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "seen")

	var out bytes.Buffer
	if err := RunHooks(t.Context(), RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "printenv " + domain.EnvComposeProjectName + " > " + marker}},
		WorkDir: dir,
		Env:     map[string]string{domain.EnvComposeProjectName: "feat-x"},
		Output:  &out,
	}); err != nil {
		t.Fatalf("RunHooks: %v", err)
	}

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("hook left no marker: %v", err)
	}
	if strings.TrimSpace(string(got)) != "feat-x" {
		t.Errorf("hook saw %s=%q, want %q", domain.EnvComposeProjectName, strings.TrimSpace(string(got)), "feat-x")
	}
}

// A caller with nothing to say leaves the hook's environment as it was.
func TestRunHooksWithoutEnvKeepsProcessEnvironment(t *testing.T) {
	t.Setenv("WTM_HOOK_PROBE", "inherited")

	dir := t.TempDir()
	marker := filepath.Join(dir, "seen")
	script := filepath.Join(dir, "hook.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintenv WTM_HOOK_PROBE > "+marker+"\n"), 0o755); err != nil {
		t.Fatalf("write hook script: %v", err)
	}

	var out bytes.Buffer
	if err := RunHooks(t.Context(), RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: script}},
		WorkDir: dir,
		Output:  &out,
	}); err != nil {
		t.Fatalf("RunHooks: %v", err)
	}

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("hook left no marker: %v", err)
	}
	if strings.TrimSpace(string(got)) != "inherited" {
		t.Errorf("hook saw %q, want %q", strings.TrimSpace(string(got)), "inherited")
	}
}

// A hook given a worktree's variables gets that worktree's alone: what the
// launching shell carries for another worktree does not fill a name left unset.
func TestRunHooksWithEnvDropsTheCallersWorktreeVariables(t *testing.T) {
	t.Setenv(domain.EnvComposeProjectName, "launched-from-another-worktree")

	dir := t.TempDir()
	marker := filepath.Join(dir, "seen")

	var out bytes.Buffer
	if err := RunHooks(t.Context(), RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "printenv " + domain.EnvComposeProjectName + " > " + marker + " || true"}},
		WorkDir: dir,
		Env:     map[string]string{domain.EnvIsolation: string(domain.IsolationVerbatim)},
		Output:  &out,
	}); err != nil {
		t.Fatalf("RunHooks: %v", err)
	}

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("hook left no marker: %v", err)
	}
	if strings.TrimSpace(string(got)) != "" {
		t.Errorf("hook saw %s=%q, want it unset", domain.EnvComposeProjectName, strings.TrimSpace(string(got)))
	}
}
